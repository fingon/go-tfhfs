/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2017 Markus Stenberg
 *
 * Created:       Thu Dec 28 11:20:29 2017 mstenber
 * Last modified: Thu Feb  7 10:08:02 2019 mstenber
 * Edit time:     397 min
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

	"github.com/fingon/go-tfhfs/ibtree"
	"github.com/fingon/go-tfhfs/ibtree/hugger"
	"github.com/fingon/go-tfhfs/mlog"
	"github.com/fingon/go-tfhfs/storage"
	"github.com/fingon/go-tfhfs/util"
	"github.com/hanwen/go-fuse/fuse"
)

const iterations = 1234
const inodeDataLength = 8
const blockSubTypeOffset = inodeDataLength

type deleteNotify struct {
	Parent uint64
	Child  uint64
	Name   string
}

type Fs struct {
	Ops fsOps

	// These have their own locking or are used in single-threaded way
	inodeTracker
	hugger.Hugger
	closing       chan chan struct{}
	deleted       chan deleteNotify
	flushInterval time.Duration
	server        *fuse.Server
	storage       *storage.Storage
	writeLimiter  util.ParallelLimiter
	writeBuffers  util.ByteSliceAtomicList
}

func (self *Fs) Close() {

	mlog.Printf2("fs/fs", "fs.Close")

	// this will kill the underlying goroutine and ensure it has flushed
	ch := make(chan struct{})
	self.closing <- ch
	<-ch

	// then we can close storage (which will close backend)
	self.storage.Close()

	mlog.Printf2("fs/fs", " great success at closing Fs")
}

func (self *Fs) Flush() {
	mlog.Printf2("fs/fs", "fs.Flush started")
	self.Hugger.Flush()
	self.storage.Flush()
	mlog.Printf2("fs/fs", " done with fs.Flush")
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
		inode, key := file.ReadNextinode()
		if inode == nil {
			return
		}
		file.pos++
		file.lastKey = &key
		defer inode.Release()
		name := key.Filename()
		mlog.Printf2("fs/fs", " %s", name)
		ret = append(ret, name)
	}
	return
}

func iterateNodeReferences(nd *ibtree.NodeData, cb storage.BlockReferenceCallback) {
	if !nd.Leafy {
		for _, c := range nd.Children {
			cb(c.Value)
		}
		return
	}
	for _, c := range nd.Children {
		k := BlockKey(c.Key)
		switch k.SubType() {
		case BST_FILE_OFFSET2EXTENT:
			cb(c.Value)
		case BST_NAMEHASH_NAME_BLOCK:
			cb(c.Value)
		}
	}
}

func NewFs(st *storage.Storage, RootName string, cacheSize int) *Fs {
	fs := &Fs{storage: st}
	fs.RootName = RootName
	fs.Hugger.Storage = st
	fs.Hugger.IterateReferencesCallback = iterateNodeReferences
	fs.MergeCallback = MergeTo3
	(&fs.Hugger).Init(cacheSize)
	fs.Ops.fs = fs
	fs.closing = make(chan chan struct{})
	fs.deleted = make(chan deleteNotify, 10)
	fs.flushInterval = 1 * time.Second
	fs.inodeTracker.Init(fs)
	fs.writeLimiter.LimitPerCPU = 3 // somewhat IO bound
	fs.writeBuffers.New = func() []byte {
		return make([]byte, dataExtentSize+dataHeaderMaximumSize)
	}
	st.IterateReferencesCallback = func(id string, data []byte, cb storage.BlockReferenceCallback) {
		fs.iterateReferencesCallback(id, data, cb)
	}
	if fs.RootIsNew() {
		// getInode succeeds always; Get does not
		defer fs.inodeLock.Locked()()
		root := fs.getInode(fuse.FUSE_ROOT_ID)
		defer root.metaWriteLock.Locked()()
		var meta InodeMeta
		meta.StMode = 0777 | fuse.S_IFDIR
		meta.StNlink++ // root has always built-in link
		fs.Update(func(tr *hugger.Transaction) {
			root.SetMetaInTransaction(&meta, tr)
		})
	}
	go func() { // ok, singleton per fs
		for {
			select {
			case deleted := <-fs.deleted:
				if fs.server != nil {
					fs.server.DeleteNotify(deleted.Parent,
						deleted.Child,
						deleted.Name)
				}
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

func (self *Fs) hasExternalReferences(id string) bool {
	return false
}

func (self *Fs) iterateReferencesCallback(id string, data []byte, cb storage.BlockReferenceCallback) {
	nd, ok := self.GetCachedNodeData(ibtree.BlockId(id))
	if !ok {
		nd = BytesToNodeData(data)
	}
	if nd == nil {
		return
	}
	self.Hugger.IterateReferencesCallback(nd, cb)
}

func (self *Fs) deleteNotify(parent, child uint64, name string) {
	self.deleted <- deleteNotify{Parent: parent, Child: child, Name: name}
}

func BytesToNodeData(bd []byte) *ibtree.NodeData {
	mlog.Printf2("fs/fs", "BytesToNodeData - %d bytes", len(bd))
	dt := ibtree.BlockDataType(bd[0])
	switch dt {
	case BDT_EXTENT:
		break
	case ibtree.BDT_NODE:
		nd := ibtree.NewNodeDataFromBytes(bd)
		return nd
	default:
		log.Panicf("BytesToNodeData - wrong dt:%v", dt)
	}
	return nil
}

// WithoutParallelWrites ensures the data being read is entirely
// consistent. This means that there is no locks on filehandles, and
// that all pending data has been written to Storage (which will
// persist it eventually).
func (self *Fs) WithoutParallelWrites(cb func()) {
	self.writeLimiter.Exclusive(cb)
}

func (self *Fs) closeWithoutTransactions() {
	self.AssertNoTransactions()
	self.Close()
}
