/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2018 Markus Stenberg
 *
 * Created:       Fri Jan  5 12:54:18 2018 mstenber
 * Last modified: Fri Jan  5 13:17:08 2018 mstenber
 * Edit time:     4 min
 *
 */

package storage

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
	b.addStorageRefCount(1)
	return &StorageBlock{block: b}
}

func (self *StorageBlock) Close() {
	self.block.addStorageRefCount(-1)
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
