/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2018 Markus Stenberg
 *
 * Created:       Wed Jan  3 14:54:09 2018 mstenber
 * Last modified: Thu Jan 18 17:59:50 2018 mstenber
 * Edit time:     312 min
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

	// dependencies within data; it is set on first access if not
	// already available.
	deps *util.StringList

	// Storage this is stored on, if any
	storage *Storage

	// In-memory reference count
	storageRefCount int32

	// In-memory reference count (subset of storageCount that have
	// come due to serving API (new StorageBlocks) or via
	// subsequent Open/Close calls of StorageBlocks)
	externalStorageRefCount int32

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

const idLenInString = 4

func (self *Block) String() string {
	id := self.Id
	if len(id) > idLenInString {
		id = id[:idLenInString]
	}

	return fmt.Sprintf("Bl@%p{Id:%x, rc:%v/src:%v/erc:%v}", self, id, self.RefCount, self.storageRefCount, self.externalStorageRefCount)
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
	delete(self.storage.dirtyBlocks, self.Id)

	self.addStorageRefCount(-1)
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
	switch rc := self.storageRefCount; {
	case rc < 0:
		log.Panic("Negative reference count", rc)
	case rc == 0:
		self.storage.dirtyStorageRefBlocks[self.Id] = self
	default:
		if (self.RefCount == 0) != self.haveStorageRefs {
			self.storage.dirtyStorageRefBlocks[self.Id] = self
		}
	}
}

func (self *Block) flushStorageRef() int {
	delete(self.storage.dirtyStorageRefBlocks, self.Id)
	if self.storageRefCount == 0 {
		self.shouldHaveStorageDependencies(false)
		delete(self.storage.blocks, self.Id)
		return 1
	}
	if self.shouldHaveStorageDependencies(self.RefCount == 0) {
		return 1
	}
	return 0
}

func (self *Block) addExternalStorageRefCount(v int32) {
	mlog.Printf2("storage/block", "%v.addExternalStorageRefCount %v", self, v)
	self.externalStorageRefCount += v
	if self.externalStorageRefCount < 0 {
		log.Panic("Negative reference count", self.externalStorageRefCount)
	}
}

func (self *Block) markDirty() {
	if self.Stored != nil {
		mlog.Printf2("storage/block", "%v.markDirty (already)", self)
		if self.storage.dirtyBlocks[self.Id] != self {
			mlog.Panicf("markDirty - Stored set but not in dirtyBlocks list")
		}
		return
	}
	mlog.Printf2("storage/block", "%v.markDirty (fresh)", self)
	self.addStorageRefCount(1)
	self.Stored = &BlockMetadata{Status: self.Status,
		RefCount: self.RefCount}
	self.storage.dirtyBlocks[self.Id] = self
}

func (self *Block) setStatus(st BlockStatus) bool {
	if self.Status == st {
		return true
	}
	mlog.Printf2("storage/block", "%v.setStatus = %v", self, st)
	// Check if the status transition is actually POSSIBLE first
	shouldHaveDeps := st < BS_WANT_NORMAL
	hadDeps := self.Status < BS_WANT_NORMAL
	changingDeps := shouldHaveDeps != hadDeps
	if changingDeps && shouldHaveDeps {
		refs := make([]*Block, 0)
		cleanrefs := func() {
			for _, b := range refs {
				b.addStorageRefCount(0)
			}
		}
		defer cleanrefs()
		possible := true
		self.iterateReferences(func(id string) {
			if !possible {
				return
			}
			b := self.storage.getBlockById(id)
			if b == nil {
				possible = false
				return
			}
			refs = append(refs, b)
		})

		if !possible {
			return false
		}
	}

	self.markDirty()
	if changingDeps {
		disk := false
		storage := false
		// Set the dependencies according to what we know is
		// there
		if self.Stored.RefCount > 0 {
			self.setDependencies(true, false, st)
			disk = true
		} else if self.storageRefCount > 0 {
			self.setDependencies(true, true, st)
			storage = true
		}
		// Clear old dependencies
		self.shouldHaveStorageDependencies(false)
		self.shouldHaveDiskDependencies(false)
		// And note that dependency state actually matches
		// what we manually set
		self.haveDiskRefs = disk
		self.haveStorageRefs = storage
	}
	self.Status = st
	return true

}

func (self *Block) setDependencies(add, storage bool, st BlockStatus) {
	// These do not need actual ones
	if st >= BS_WANT_NORMAL {
		return
	}
	self.iterateReferences(func(id string) {
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

func (self *Block) updateDependencies(add, storage bool, stp *BlockStatus) bool {
	st := self.Status
	if stp != nil {
		st = *stp
	}
	if storage {
		if self.haveStorageRefs == add {
			return false
		}
		self.haveStorageRefs = add
	} else if stp == nil {
		if self.haveDiskRefs == add {
			return false
		}
		self.haveDiskRefs = add
	}
	mlog.Printf2("storage/block", "%v.updateDependencies %v %v %v", self, add, storage, st)
	self.setDependencies(add, storage, st)
	return true
}

func (self *Block) iterateReferences(cb func(id string)) {
	if self.storage.IterateReferencesCallback == nil {
		return
	}
	if self.deps == nil {
		self.deps = &util.StringList{}
		self.storage.IterateReferencesCallback(self.Id, self.GetData(), func(id string) {
			self.deps.PushFront(id)
		})
	}
	self.deps.Iterate(cb)
}

func (self *Block) shouldHaveStorageDependencies(value bool) bool {
	return self.updateDependencies(value, true, nil)
}

func (self *Block) shouldHaveDiskDependencies(value bool) bool {
	return self.updateDependencies(value, false, nil)
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
		b.storageRefCount = 0
		b.externalStorageRefCount = 0
		b.haveDiskRefs = true
		b.haveStorageRefs = false
		b.Stored = nil
		self.blocks[id] = b
	}
	return b
}
