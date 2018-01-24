/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2018 Markus Stenberg
 *
 * Created:       Fri Jan  5 12:54:18 2018 mstenber
 * Last modified: Wed Jan 24 13:49:19 2018 mstenber
 * Edit time:     27 min
 *
 */

package storage

import (
	"fmt"
	"log"

	"github.com/fingon/go-tfhfs/mlog"
)

// StorageBlock is the public read-only view of a block to the outside
// world. All provided methods are synchronous, and actually cause
// changes to be propagated to Storage (eventually) if need be.
type StorageBlock struct {
	block  *Block
	closed bool
}

func newStorageBlock(b *Block) *StorageBlock {
	if b == nil {
		return nil
	}
	// These are created only in main goroutine so this is fine;
	// however, as the objects are passed to clients, see below..
	if b.addExternalStorageRefCount(1) == 1 {
		b.addStorageRefCount(1)
	}
	self := &StorageBlock{block: b}
	mlog.Printf2("storage/storageblock", "newStorageBlock:%v", self)
	return self
}

func (self *StorageBlock) Open() *StorageBlock {
	if self.closed {
		log.Panic("use after close of ", self)
	}
	self.block.addExternalStorageRefCount(1)
	sb := &StorageBlock{block: self.block}
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
	if self.closed {
		log.Panic("double close of StorageBlock")
	}
	self.closed = true
	if self.block.addExternalStorageRefCount(-1) == 0 {
		// We may be in whatever thread -> do final release
		// through the job mechanism
		self.block.storage.ReleaseStorageBlockId(self.block.Id)
	}
}

func (self *StorageBlock) Id() string {
	if self.closed {
		log.Panic("use after close of ", self)
	}
	return self.block.Id
}

func (self *StorageBlock) IterateReferences(cb func(id string)) {
	self.block.iterateReferences(cb)
}

func (self *StorageBlock) Data() []byte {
	if self.closed {
		log.Panic("use after close  of ", self)
	}
	return self.block.GetData()
}

func (self *StorageBlock) Status() BlockStatus {
	if self.closed {
		log.Panic("use after close  of ", self)
	}
	return self.block.Status
}

func (self *StorageBlock) SetStatus(status BlockStatus) bool {
	if self.closed {
		log.Panic("use after close  of ", self)
	}
	return self.block.storage.setStorageBlockStatus(self, status)
}

func (self *StorageBlock) String() string {
	return fmt.Sprintf("SB@%p{%v}", self, self.block)
	// return fmt.Sprintf("SB{%v}", self.block)
}
