/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2018 Markus Stenberg
 *
 * Created:       Fri Jan  5 12:54:18 2018 mstenber
 * Last modified: Fri Jan  5 22:39:39 2018 mstenber
 * Edit time:     9 min
 *
 */

package storage

import "fmt"

// StorageBlock is the public read-only view of a block to the outside
// world. All provided methods are synchronous, and actually cause
// changes to be propagated to Storage (eventually) if need be.
type StorageBlock struct {
	block *Block
}

func NewStorageBlock(b *Block) *StorageBlock {
	if b == nil {
		return nil
	}
	// These are created only in main goroutine so this is fine;
	// however, as the objects are passed to clients, see below..
	b.addStorageRefCount(1)
	return &StorageBlock{block: b}
}

func (self *StorageBlock) Close() {
	// direct path is tempting, but bad; do it via the channel so
	// we don't kill things too soon or without proper locking of
	// maps etc.
	//
	// self.block.addStorageRefCount(-1)

	self.block.storage.ReleaseStorageBlockId(self.block.Id)
}

func (self *StorageBlock) Id() string {
	return self.block.Id
}

func (self *StorageBlock) Data() []byte {
	return self.block.GetData()
}

func (self *StorageBlock) Status() BlockStatus {
	return self.block.Status
}

func (self *StorageBlock) SetStatus(status BlockStatus) {
	self.block.setStatus(status)
}

func (self *StorageBlock) String() string {
	return fmt.Sprintf("SB{%v}", self.block)
}
