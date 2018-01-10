/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2017 Markus Stenberg
 *
 * Created:       Thu Dec 28 11:20:29 2017 mstenber
 * Last modified: Wed Jan 10 12:02:44 2018 mstenber
 * Edit time:     316 min
 *
 */

// fs package implements fuse.RawFileSystem on top of the other
// modules of go-tfhfs project.
//
// The low-level API is more or less mandatory as e.g. list of files
// from huge directory is not feasible to provide in one chunk, and we
// want to have fairly dynamic relationship with the tree.
package fs

import (
	"log"
	"os"
	"time"

	"github.com/bluele/gcache"
	"github.com/fingon/go-tfhfs/codec"
	"github.com/fingon/go-tfhfs/ibtree"
	"github.com/fingon/go-tfhfs/mlog"
	"github.com/fingon/go-tfhfs/storage"
	"github.com/fingon/go-tfhfs/util"
	"github.com/hanwen/go-fuse/fuse"
)

const iterations = 1234
const inodeDataLength = 8
const blockSubTypeOffset = inodeDataLength

type Fs struct {
	// These have their own locking or are used in single-threaded way
	inodeTracker
	Ops                  fsOps
	closing              chan chan struct{}
	flushInterval        time.Duration
	server               *fuse.Server
	tree                 *ibtree.IBTree
	storage              *storage.Storage
	rootName             string
	root                 fsTreeRootAtomicPointer
	nodeDataCache        gcache.Cache
	transactionRetryLock util.MutexLocked
}

func (self *Fs) Close() {

	mlog.Printf2("fs/fs", "fs.Close")

	// this will kill the underlying goroutine and ensure it has flushed
	self.closing <- make(chan struct{})

	// then we can close storage (which will close backend)
	self.storage.Close()

	mlog.Printf2("fs/fs", " great success at closing Fs")
}

// ibtree.IBTreeBackend API
func (self *Fs) LoadNode(id ibtree.BlockId) *ibtree.IBNodeData {
	var v interface{}
	if self.nodeDataCache != nil {
		v, _ = self.nodeDataCache.Get(id)
	}
	if v == nil {
		nd := self.loadNode(id)
		if nd != nil {
			if self.nodeDataCache != nil {
				self.nodeDataCache.Set(id, nd)
			}
		} else {
			log.Panicf("Unable to find node %x", id)
		}
		return nd
	}
	mlog.Printf2("fs/fs", "fs.LoadNode found %x in cache: %p", id, v)
	return v.(*ibtree.IBNodeData)
}

func (self *Fs) Flush() {
	mlog.Printf2("fs/fs", "fs.Flush started")
	self.storage.Flush()
	mlog.Printf2("fs/fs", " .. done with fs.Flush")
}

// ibtree.IBTreeBackend API
func (self *Fs) SaveNode(nd *ibtree.IBNodeData) ibtree.BlockId {
	log.Panicf("should be always used via fsTransaction.SaveNode")
	return ibtree.BlockId("")
}

func (self *Fs) GetTransaction() *fsTransaction {
	// mlog.Printf2("fs/fs", "GetTransaction of %p", self.treeRoot)
	return newFsTransaction(self)
}

// ListDir provides testing utility as output of ReadDir/ReadDirPlus
// is binary garbage and I am too lazy to write a decoder for it.
func (self *Fs) ListDir(ino uint64) (ret []string) {
	mlog.Printf2("fs/fs", "Fs.ListDir #%d", ino)
	inode := self.GetInode(ino)
	defer inode.Release()

	file := inode.GetFile(uint32(os.O_RDONLY))
	defer file.Release()
	for {
		inode, name := file.ReadNextinode()
		if inode == nil {
			return
		}
		file.pos++
		defer inode.Release()
		mlog.Printf2("fs/fs", " %s", name)
		ret = append(ret, name)
	}
	return
}

func NewFs(st *storage.Storage, rootName string, cacheSize int) *Fs {
	fs := &Fs{storage: st, rootName: rootName}
	if cacheSize > 0 {
		fs.nodeDataCache = gcache.New(cacheSize).
			ARC().
			//	LoaderFunc(func(k interface{}) (interface{}, error) {
			//		return fs.loadNode(k.(ibtree.BlockId)), nil
			//}).
			Build()
	}
	fs.Ops.fs = fs
	fs.closing = make(chan chan struct{})
	fs.flushInterval = 1 * time.Second
	fs.inodeTracker.Init(fs)
	fs.tree = ibtree.IBTree{NodeMaximumSize: 4096}.Init(fs)
	st.IterateReferencesCallback = func(id string, data []byte, cb storage.BlockReferenceCallback) {
		fs.iterateReferencesCallback(id, data, cb)
	}
	rootbid := st.GetBlockIdByName(rootName)
	if rootbid != "" {
		bid := ibtree.BlockId(rootbid)
		node := fs.tree.LoadRoot(bid)
		if node == nil {
			log.Panicf("Loading of root block %x failed", bid)
		}
		block := fs.storage.GetBlockById(string(bid))
		root := &fsTreeRoot{node, block}
		fs.root.Set(root)
	}
	if fs.root.Get() == nil {
		fs.root.Set(&fsTreeRoot{node: fs.tree.NewRoot()})
		// getInode succeeds always; Get does not
		defer fs.inodeLock.Locked()()
		root := fs.getInode(fuse.FUSE_ROOT_ID)
		defer root.metaWriteLock.Locked()()
		var meta InodeMeta
		meta.StMode = 0777 | fuse.S_IFDIR
		meta.StNlink++ // root has always built-in link
		fs.Update(func(tr *fsTransaction) {
			root.SetMetaInTransaction(&meta, tr)
		})
	}
	go func() {
		for {
			select {
			case done := <-fs.closing:
				fs.Flush()
				done <- struct{}{}
				return
			case <-time.After(fs.flushInterval):
				fs.Flush()
			}
		}
	}()

	return fs
}

func NewCryptoStorage(password, salt string, backend storage.Backend) *storage.Storage {
	c1 := codec.EncryptingCodec{}.Init([]byte(password), []byte(salt), iterations)
	c2 := &codec.CompressingCodec{}
	c := codec.CodecChain{}.Init(c1, c2)
	st := storage.Storage{Codec: c, Backend: backend}.Init()
	return st
}

func (self *Fs) hasExternalReferences(id string) bool {
	return false
}

func (self *Fs) iterateReferencesCallback(id string, data []byte, cb storage.BlockReferenceCallback) {
	var v interface{}
	if self.nodeDataCache != nil {
		v, _ = self.nodeDataCache.GetIFPresent(ibtree.BlockId(id))
	}
	var nd *ibtree.IBNodeData
	if v != nil {
		nd = v.(*ibtree.IBNodeData)
	} else {
		nd = self.loadNodeFromBytes(data)
		if nd == nil {
			return
		}
	}

	if !nd.Leafy {
		for _, c := range nd.Children {
			cb(c.Value)
		}
		return
	}
	for _, c := range nd.Children {
		k := blockKey(c.Key)
		switch k.SubType() {
		case BST_FILE_OFFSET2EXTENT:
			cb(c.Value)
		}
	}
}

func (self *Fs) loadNodeFromBytes(bd []byte) *ibtree.IBNodeData {
	mlog.Printf2("fs/fs", "loadNodeFromBytes - %d bytes", len(bd))
	dt := BlockDataType(bd[0])
	switch dt {
	case BDT_EXTENT:
		break
	case BDT_NODE:
		nd := &ibtree.IBNodeData{}
		_, err := nd.UnmarshalMsg(bd[1:])
		if err != nil {
			log.Panic(err)
		}
		return nd
	default:
		log.Panicf("loadNodeFromString - wrong dt:%v", dt)
	}
	return nil
}

func (self *Fs) loadNode(id ibtree.BlockId) *ibtree.IBNodeData {
	mlog.Printf2("fs/fs", "fs.loadNode %x", id)
	b := self.storage.GetBlockById(string(id))
	if b == nil {
		return nil
	}
	defer b.Close()
	return self.loadNodeFromBytes(b.Data())
}
