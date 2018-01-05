/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2018 Markus Stenberg
 *
 * Created:       Wed Jan  3 14:54:09 2018 mstenber
 * Last modified: Fri Jan  5 17:25:06 2018 mstenber
 * Edit time:     159 min
 *
 */

package storage

import (
	"log"

	"github.com/fingon/go-tfhfs/mlog"
)

// Block is abstraction used between Storage and its Backends.
type Block struct {
	BlockMetadata // contains RefCount, Status

	// Id contains identity of the block, derived from Data if not
	// set.
	Id string

	// Actually plaintext data (if available; GetData() should be
	// used to get it always when accessing from outside backends
	// as it may not be set at that point otherwise).
	Data []byte

	// Node is the actual btree node encoded within this
	// block. Used to derive Data as needed.
	//Node *TreeNode

	// Backend this is fetched from, if any
	Backend Backend

	// Stored version of the block metadata, if any. Set only if
	// something has changed locally. For fresh blocks, is nil.
	Stored *BlockMetadata

	// Storage this is stored on, if any
	storage *Storage

	// In-memory reference count (from within Storage)
	storageRefCount int32
}

func (self *Block) GetData() []byte {
	if self.Data == nil {
		if self.storage == nil {
			mlog.Printf2("storage/block", "b.GetData %p - calling be.GetBlockData", self)
			self.Data = self.Backend.GetBlockData(self)
		} else {
			mlog.Printf2("storage/block", "b.GetData %p - calling s.be.GetBlockData", self)
			data := self.storage.Backend.GetBlockData(self)
			b, err := self.storage.Codec.DecodeBytes(data, []byte(self.Id))
			if err != nil {
				log.Panic("Decoding failed", err)
			}
			self.Data = b
			self.storage.reads++
			self.storage.readbytes += len(data)
		}
	}
	return self.Data
}

func (self *Block) flush() int {
	mlog.Printf2("storage/block", "b.flush %p %v %v", self, self.RefCount, self.storageRefCount)
	// self.Stored MUST be set, otherwise we wouldn't be dirty!
	if self.Stored == nil {
		log.Panicf("self.Stored not set?!?")
	}
	ops := 0
	if self.RefCount == 0 {
		if self.Backend != nil {
			// just in case grab data if we already do not
			// have it and we have to re-add this back
			self.GetData()
			self.Backend.DeleteBlock(self)
			self.Backend = nil
		}
		ops++
	} else if self.Backend == nil {
		// We want to be added to backend
		self.storage.writes++
		data := self.GetData()
		self.storage.writebytes += len(data)

		b, err := self.storage.Codec.EncodeBytes(data, []byte(self.Id))
		if err != nil {
			log.Panic("Encoding failed", err)
		}
		bl := *self
		bl.Data = b

		self.storage.Backend.StoreBlock(&bl)
		self.Backend = self.storage.Backend
		ops++
	} else {
		if self.Stored.Status != self.Status {
			// self.Stored.status != self.status
			if self.Status == BlockStatus_MISSING {
				// old type = NORMAL
			} else if self.Status == BlockStatus_WANT_WEAK {
				// old type = WEAK
			} else {
				mlog.Printf2("storage/block", " status changed")
				self.storage.updateBlockDataDependencies(self, true, self.Status)
				self.storage.updateBlockDataDependencies(self, false, self.Stored.Status)
			}
			ops++
		}
		ops += self.storage.Backend.UpdateBlock(self)
	}
	self.Stored = nil
	self.addStorageRefCount(-1)
	delete(self.storage.dirtyBlocks, self.Id)
	return ops
}

func (self *Block) addRefCount(count int32) {
	mlog.Printf2("storage/block", "b.addRefCount %p %v -> %v", self, count, self.RefCount+count)
	self.markDirty()
	self.RefCount += count
	if self.RefCount < 0 {
		log.Panicf("RefCount below 0 for %x", self.Id)
	}
	hadRefs := self.Stored.RefCount != 0
	haveRefs := self.RefCount != 0
	if hadRefs != haveRefs {
		mlog.Printf2("storage/block", " dependencies changed")
		self.storage.updateBlockDataDependencies(self, haveRefs, self.Status)
	}
}

func (self *Block) setStatus(st BlockStatus) {
	mlog.Printf2("storage/block", "setStatus %p %v", self, st)
	self.markDirty()
	self.Status = st

}

func (self *Block) addStorageRefCount(v int32) {
	self.storageRefCount += v
	nv := self.storageRefCount
	mlog.Printf2("storage/block", "b.addStorageRefCount %p: %v -> %v", self, v, nv)
	if nv <= 0 {
		if nv < 0 {
			log.Panic("Negative reference count", nv)
		}
		if self.Stored != nil {
			log.Panic("Storage reference count before flush - reference mismatch?")
		}
		mlog.Printf2("storage/block", " removed block %x", self.Id)
		delete(self.storage.blocks, self.Id)
	}
}

func (self *Block) markDirty() {
	if self.Stored != nil {
		mlog.Printf2("storage/block", "b.markDirty %p (already)", self)
		return
	}
	mlog.Printf2("storage/block", "b.markDirty %p (fresh)", self)
	self.addStorageRefCount(1)
	self.Stored = &BlockMetadata{Status: self.Status,
		RefCount: self.RefCount}
	self.storage.dirtyBlocks[self.Id] = self
}

// getBlockById returns Block (if any) that matches id.
func (self *Storage) getBlockById(id string) *Block {
	mlog.Printf2("storage/block", "st.getBlockById %x", id)
	b, ok := self.blocks[id]
	if !ok {
		b = self.Backend.GetBlockById(id)
		if b == nil {
			mlog.Printf2("storage/block", " does not exist according to backend")
			return nil
		}
		b.storage = self
		self.blocks[id] = b
	}
	return b
}
