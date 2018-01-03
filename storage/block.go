/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2018 Markus Stenberg
 *
 * Created:       Wed Jan  3 14:54:09 2018 mstenber
 * Last modified: Thu Jan  4 01:08:03 2018 mstenber
 * Edit time:     4 min
 *
 */

package storage

import (
	"log"
	"sync/atomic"
	"unsafe"

	"github.com/fingon/go-tfhfs/mlog"
)

// Block is externally usable read-only structure that is handled
// using BlockStorer interface. Notably changes to 'Id' and 'Data' are
// not allowed, and Status should be mutated only via
// UpdateBlockStatus call of BlockStorer.
type Block struct {
	BlockMetadata // contains RefCount, Status

	// Id contains identity of the block, derived from Data if not
	// set.
	Id string

	// Actually plaintext data (if available; GetData() should be
	// used to get it always). Backends should use
	// GetCodecData() when writing things to disk and
	// SetCodecData() when loading from disk.
	Data []byte

	// Node is the actual btree node encoded within this
	// block. Used to derive Data as needed.
	//Node *TreeNode

	// Backend this is fetched from, if any
	backend BlockBackend

	// Storage this is stored on, if any
	storage *Storage

	// Stored version of the block metadata, if any. Set only if
	// something has changed locally.
	stored *BlockMetadata

	// Last usage time (in Storage.t units)
	t uint64
}

func (self *Block) GetData() []byte {
	if self.Data == nil {
		if self.storage == nil {
			mlog.Printf2("storage/block", "b.GetData - calling be.GetBlockData")
			self.Data = self.backend.GetBlockData(self)
		} else {
			oldSize := self.getCacheSize()
			mlog.Printf2("storage/block", "b.GetData - calling s.be.GetBlockData")
			data := self.storage.Backend.GetBlockData(self)
			self.SetCodecData(data)
			newSize := self.getCacheSize()
			self.storage.cacheSize += newSize - oldSize
			self.storage.reads++
			self.storage.readbytes += len(data)
		}
	}
	return self.Data
}

func (self *Block) GetCodecData() []byte {
	b := self.GetData()
	if self.storage == nil {
		return b
	}
	b, err := self.storage.Codec.EncodeBytes(b, []byte(self.Id))
	if err != nil {
		log.Panic("Encoding failed", err)
	}
	return b
}

func (self *Block) SetCodecData(b []byte) {
	if self.storage == nil {
		self.Data = b
		return
	}
	b, err := self.storage.Codec.DecodeBytes(b, []byte(self.Id))
	if err != nil {
		log.Panic("Decoding failed", err)
	}
	self.Data = b
}

func (self *Block) flush() int {
	// self.stored MUST be set, otherwise we wouldn't be dirty!
	ops := 0
	if self.stored.RefCount == 0 {
		if self.RefCount > 0 {
			self.storage.writes++
			self.storage.writebytes += len(self.GetData())
			self.storage.Backend.StoreBlock(self)
			ops = ops + 1
		} else {
			ops = ops + self.storage.updateBlock(self)
		}
	} else {
		if self.stored.Status != self.Status {
			self.flushStatus()
			ops = ops + 1
		}
		ops = ops + self.storage.updateBlock(self)
	}
	self.stored = nil
	return ops
}

func (self *Block) flushStatus() {
	// self.stored.status != self.status
	if self.Status == BlockStatus_MISSING {
		// old type = NORMAL
		return
	}
	if self.Status == BlockStatus_WANT_WEAK {
		// old type = WEAK
		return
	}
	self.storage.updateBlockDataDependencies(self, true, self.Status)
	self.storage.updateBlockDataDependencies(self, false, self.stored.Status)

}

func (self *Block) getCacheSize() int {
	s := int(unsafe.Sizeof(*self))
	return s + len(self.Id) + len(self.Data)
}

func (self *Block) setRefCount(count int) {
	self.markDirty()
	self.RefCount = count
}

func (self *Block) setStatus(st BlockStatus) {
	self.markDirty()
	self.Status = st

}

func (self *Block) markDirty() {
	if self.stored != nil {
		return
	}
	self.stored = &BlockMetadata{Status: self.Status,
		RefCount: self.RefCount}
	// Add to dirty block list
	self.storage.dirtyBid2Block[self.Id] = self
}

func (self *Storage) GetBlockById(id string) *Block {
	mlog.Printf2("storage/block", "st.GetBlockById %x", id)
	b := self.gocBlockById(id)
	if self.blockValid(b) {
		return b
	}
	return nil
}

func (self *Storage) gocBlockById(id string) *Block {
	mlog.Printf2("storage/block", "st.gocBlockById %x", id)
	b, ok := self.cacheBid2Block[id]
	if !ok {
		b = self.getBlockById(id)
		if b == nil {
			b = &Block{Id: id, storage: self}
		}
		self.cacheSize += b.getCacheSize()
		self.cacheBid2Block[id] = b
	}
	b.t = atomic.AddUint64(&self.t, uint64(1))
	return b
}

func (self *Storage) blockValid(b *Block) bool {
	if b == nil {
		mlog.Printf2("storage/block", "blockValid - not, nil")
		return false
	}
	if b.RefCount == 0 {
		if self.HasExternalReferencesCallback != nil && self.HasExternalReferencesCallback(b.Id) {
			mlog.Printf2("storage/block", "blockValid - yes, transient refs")
			return true
		}
		mlog.Printf2("storage/block", "blockValid - no")
		return false
	}
	mlog.Printf2("storage/block", "blockValid - yes, stored refs")
	return true
}
