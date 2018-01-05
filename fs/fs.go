/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2017 Markus Stenberg
 *
 * Created:       Thu Dec 28 11:20:29 2017 mstenber
 * Last modified: Fri Jan  5 17:02:13 2018 mstenber
 * Edit time:     287 min
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
	"github.com/minio/sha256-simd"
)

const iterations = 1234
const inodeDataLength = 8
const blockSubTypeOffset = inodeDataLength

type Fs struct {
	// These have their own locking or are used in single-threaded way
	inodeTracker
	Ops           fsOps
	closing       chan chan struct{}
	flushInterval time.Duration
	server        *fuse.Server
	tree          *ibtree.IBTree
	storage       *storage.Storage
	rootName      string
	root          fsTreeRootAtomicPointer

	// data covers the things below here that involve writing
	// e.g. any write operation should grab the lock early on to
	// make sure writes are consistent.
	lock          util.MutexLocked
	nodeDataCache gcache.Cache
}

func (self *Fs) Close() {
	mlog.Printf2("fs/fs", "fs.Close")

	// this will kill the underlying goroutine and ensure it has flushed
	self.closing <- make(chan struct{})

	// then we can close backend
	self.storage.Close()

	// and finally backend
	self.storage.Backend.Close()
	mlog.Printf2("fs/fs", " great success at closing Fs")
}

// ibtree.IBTreeBackend API
func (self *Fs) LoadNode(id ibtree.BlockId) *ibtree.IBNodeData {
	v, _ := self.nodeDataCache.Get(id)
	if v == nil {
		nd := self.loadNode(id)
		if nd != nil {
			self.nodeDataCache.Set(id, nd)
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
	bb := make([]byte, nd.Msgsize()+1)
	bb[0] = byte(BDT_NODE)
	b, err := nd.MarshalMsg(bb[1:1])
	if err != nil {
		log.Panic(err)
	}
	b = bb[0 : 1+len(b)]
	mlog.Printf2("fs/fs", "SaveNode %d bytes", len(b))
	bl := self.getStorageBlock(b, nd)
	bid := ibtree.BlockId(bl.Id())
	bl.Close()
	return bid
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

func NewFs(st *storage.Storage, rootName string) *Fs {
	fs := &Fs{storage: st, rootName: rootName}
	defer fs.lock.Locked()()
	fs.nodeDataCache = gcache.New(10000).
		ARC().
		//	LoaderFunc(func(k interface{}) (interface{}, error) {
		//		return fs.loadNode(k.(ibtree.BlockId)), nil
		//}).
		Build()
	fs.Ops.fs = fs
	fs.closing = make(chan chan struct{})
	fs.flushInterval = 1 * time.Second
	fs.inodeTracker.Init(fs)
	fs.tree = ibtree.IBTree{}.Init(fs)
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
		var meta InodeMeta
		meta.StMode = 0777 | fuse.S_IFDIR
		meta.StNlink++ // root has always built-in link
		root.SetMeta(&meta)
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
	v, _ := self.nodeDataCache.GetIFPresent(ibtree.BlockId(id))
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

func (self *Fs) getStorageBlock(b []byte, nd *ibtree.IBNodeData) *storage.StorageBlock {
	mlog.Printf2("fs/fs", "fs.getStorageBlock %d", len(b))
	// self.lock.AssertLocked() // should not be necessary
	h := sha256.Sum256(b)
	bid := h[:]
	id := string(bid)
	if nd != nil {
		self.nodeDataCache.Set(ibtree.BlockId(id), nd)
	}
	return self.storage.ReferOrStoreBlock0(id, b)
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
