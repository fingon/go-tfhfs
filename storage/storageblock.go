/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2018 Markus Stenberg
 *
 * Created:       Fri Jan  5 12:54:18 2018 mstenber
 * Last modified: Thu Feb  1 19:34:41 2018 mstenber
 * Edit time:     44 min
 *
 */

package storage

import (
	"fmt"
	"log"
	"runtime"

	"github.com/fingon/go-tfhfs/mlog"
)

// StorageBlock is the public read-only view of a block to the outside
// world. All provided methods are synchronous, and actually cause
// changes to be propagated to Storage (eventually) if need be.
type StorageBlock struct {
	block  *BlockPointerFuture
	closed bool
	closer []byte
	id     string
}

func newStorageBlock(id string) *StorageBlock {
	self := &StorageBlock{id: id, block: &BlockPointerFuture{}}
	mlog.Printf2("storage/storageblock", "newStorageBlock:%v", self)
	return self
}

func (self *StorageBlock) assertNotClosed(what string) {
	if !self.closed {
		return
	}
	if self.closer != nil {
		log.Panic(what, " - closed at: ", string(self.closer))
	} else {
		log.Panic(what, " - close already")
	}
}

func (self *StorageBlock) Open() *StorageBlock {
	self.assertNotClosed("Open")
	self.block.Get().addExternalStorageRefCount(1)
	sb := &StorageBlock{id: self.id, block: self.block}
	mlog.Printf2("storage/storageblock", "%v.Open => %v", self, sb)
	return sb
}

func (self *StorageBlock) Close() {
	// direct path is tempting, but bad; do it via the channel so
	// we don't kill things too soon or without proper locking of
	// maps etc.
	//
	// self.block.addStorageRefCount(-1)

	mlog.Printf2("storage/storageblock", "%v.Close", self)
	self.assertNotClosed("Close")
	self.closed = true
	if mlog.IsEnabled() {
		self.closer = make([]byte, 1024)
		self.closer = self.closer[:runtime.Stack(self.closer, false)]
	}
	if self.block.Get().addExternalStorageRefCount(-1) == 0 {
		// We may be in whatever thread -> do final release
		// through the job mechanism
		self.block.Get().storage.ReleaseStorageBlockId(self.id)
	}
}

func (self *StorageBlock) Id() string {
	if self.closed {
		log.Panic("use after close of ", self)
	}
	return self.id
}

func (self *StorageBlock) IterateReferences(cb func(id string)) {
	self.block.Get().iterateReferences(cb)
}

func (self *StorageBlock) Data() []byte {
	if self.closed {
		log.Panic("use after close  of ", self)
	}
	return self.block.Get().GetData()
}

func (self *StorageBlock) Status() BlockStatus {
	if self.closed {
		log.Panic("use after close  of ", self)
	}
	return self.block.Get().Status
}

func (self *StorageBlock) SetStatus(status BlockStatus) bool {
	if self.closed {
		log.Panic("use after close  of ", self)
	}
	return self.block.Get().storage.setStorageBlockStatus(self, status)
}

func (self *StorageBlock) String() string {
	return fmt.Sprintf("SB@%p", self)
	//return fmt.Sprintf("SB@%p{%v}", self, self.block.Get())
	// return fmt.Sprintf("SB{%v}", self.block)
}

func (self *StorageBlock) setBlock(b *Block) {
	if b != nil {
		// This is called only within storage goroutine
		if b.addExternalStorageRefCount(1) == 1 {
			b.addStorageRefCount(1)
		}
	}
	self.block.Set(b)
}
