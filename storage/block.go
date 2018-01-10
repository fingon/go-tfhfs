/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2018 Markus Stenberg
 *
 * Created:       Wed Jan  3 14:54:09 2018 mstenber
 * Last modified: Wed Jan 10 11:02:58 2018 mstenber
 * Edit time:     211 min
 *
 */

package storage

import (
	"fmt"
	"log"

	"github.com/fingon/go-tfhfs/mlog"
	"github.com/fingon/go-tfhfs/util"
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
	Data util.ByteSliceAtomicPointer

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

	// TBD: would flags be better?
	haveDiskRefs, haveStorageRefs bool
}

func (self *Block) copy() *Block {
	nb := *self
	// Beyond this, the rest is ~immutable
	if self.Stored != nil {
		nst := *self.Stored
		nb.Stored = &nst
	}
	return &nb
}

func (self *Block) GetData() []byte {
	if self.Data.Get() == nil {
		if self.storage == nil {
			mlog.Printf2("storage/block", "%v.GetData  - calling be.GetBlockData", self)
			b := self.Backend.GetBlockData(self)
			self.Data.Set(&b)
		} else {
			mlog.Printf2("storage/block", "%v.GetData - calling s.be.GetBlockData", self)
			data := self.storage.Backend.GetBlockData(self)
			self.Data.Set(&data)
			self.storage.reads++
			self.storage.readbytes += len(data)
		}
	}
	return *self.Data.Get()
}

func (self *Block) String() string {
	return fmt.Sprintf("Bl@%p{Id:%x, rc:%v/src:%v}", self, self.Id[:4], self.RefCount, self.storageRefCount)
}

func (self *Block) flush() int {
	mlog.Printf2("storage/block", "%v.flush", self)
	// self.Stored MUST be set, otherwise we wouldn't be dirty!
	if self.Stored == nil {
		log.Panicf("self.Stored not set?!?")
	}
	ops := 0
	hadRefs := self.Backend != nil && self.Stored.RefCount != 0
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
		self.storage.Backend.StoreBlock(self)
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
				self.shouldHaveDiskDependencies(true)
				// Remove old disk dependencies
				self.updateDependencies(false, false, &self.Stored.Status)
			}
			ops++
		}
		ops += self.storage.Backend.UpdateBlock(self)
	}
	haveRefs := self.RefCount != 0
	if hadRefs != haveRefs {
		mlog.Printf2("storage/block", " dependencies changed")
		self.shouldHaveDiskDependencies(haveRefs)
		if haveRefs {
			// By default if we have dependencies on disk, there
			// is no need to have them also in storage (= RAM
			// bloat)
			self.shouldHaveStorageDependencies(false)
		}
	}
	self.Stored = nil

	self.addStorageRefCount(-1)
	delete(self.storage.dirtyBlocks, self.Id)
	return ops
}

func (self *Block) addRefCount(count int32) {
	if count == 0 {
		return
	}
	mlog.Printf2("storage/block", "%v.addRefCount %v", self, count)
	self.markDirty()
	self.RefCount += count
	if self.RefCount < 0 {
		log.Panicf("RefCount below 0 for %x", self.Id)
	}
	if self.RefCount == 0 {
		// Ensure we have at least in-memory references to dependencies
		self.shouldHaveStorageDependencies(true)
	}
}

func (self *Block) addStorageRefCount(v int32) {
	mlog.Printf2("storage/block", "%v.addStorageRefCount %v", self, v)
	self.storageRefCount += v
	nv := self.storageRefCount
	if nv < 0 {
		log.Panic("Negative reference count", nv)
	} else if nv == 0 {
		// Get rid of storage dependencies if any as well
		self.shouldHaveStorageDependencies(false)
		delete(self.storage.blocks, self.Id)
	} else if self.RefCount == 0 {
		// Ensure we have at least in-memory references to dependencies
		self.shouldHaveStorageDependencies(true)

	}
}

func (self *Block) markDirty() {
	if self.Stored != nil {
		mlog.Printf2("storage/block", "%v.markDirty (already)", self)
		return
	}
	mlog.Printf2("storage/block", "%v.markDirty (fresh)", self)
	self.addStorageRefCount(1)
	self.Stored = &BlockMetadata{Status: self.Status,
		RefCount: self.RefCount}
	self.storage.dirtyBlocks[self.Id] = self
}

func (self *Block) setStatus(st BlockStatus) {
	mlog.Printf2("storage/block", "%v.setStatus = %v", self, st)
	self.markDirty()
	self.Status = st

}

func (self *Block) updateDependencies(add, storage bool, stp *BlockStatus) {
	st := self.Status
	if stp != nil {
		st = *stp
	}
	// No sub-references
	if st >= BlockStatus_WANT_NORMAL {
		return
	}
	if self.storage.IterateReferencesCallback == nil {
		return
	}
	if storage {
		if self.haveStorageRefs == add {
			return
		}
		self.haveStorageRefs = add
	} else if stp == nil {
		if self.haveDiskRefs == add {
			return
		}
		self.haveDiskRefs = add
	}
	mlog.Printf2("storage/block", "%v.updateDependencies %v %v %v", self, add, storage, st)

	self.storage.IterateReferencesCallback(self.Id, self.GetData(), func(id string) {
		b := self.storage.getBlockById(id)
		if b == nil {
			log.Panicf("Block %x awol in updateBlockDataDependencies", id)
		}
		if storage {
			if add {
				b.addStorageRefCount(1)
			} else {
				b.addStorageRefCount(-1)
			}

		} else {
			if add {
				b.addRefCount(1)
			} else {
				b.addRefCount(-1)
			}

		}
	})
}

func (self *Block) shouldHaveStorageDependencies(value bool) {
	self.updateDependencies(value, true, nil)
}

func (self *Block) shouldHaveDiskDependencies(value bool) {
	self.updateDependencies(value, false, nil)
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
		b.haveDiskRefs = true
		self.blocks[id] = b
	}
	return b
}
