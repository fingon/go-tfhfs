/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2018 Markus Stenberg
 *
 * Created:       Wed Jan  3 14:54:09 2018 mstenber
 * Last modified: Fri Jan  5 11:59:07 2018 mstenber
 * Edit time:     117 min
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
			mlog.Printf2("storage/block", "b.GetData - calling be.GetBlockData")
			self.Data = self.Backend.GetBlockData(self)
		} else {
			mlog.Printf2("storage/block", "b.GetData - calling s.be.GetBlockData")
			data := self.storage.Backend.GetBlockData(self)
			self.SetCodecData(data)
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

func (self *Block) deleteWithDeps() {
}

func (self *Block) flush() int {
	if !self.storage.dirtyBlocks.Remove(self) {
		return 0
	}
	defer self.storage.dirtyLock.Locked()()
	mlog.Printf2("storage/block", "b.flush %p %v %v", self, self.RefCount, self.storageRefCount)
	// self.Stored MUST be set, otherwise we wouldn't be dirty!
	ops := 0
	if self.RefCount == 0 {
		if self.Backend != nil {
			self.Backend.DeleteBlock(self)
		}
		ops++
	} else if self.Backend == nil {
		// We want to be added to backend
		self.storage.writes++
		self.storage.writebytes += len(self.GetData())
		self.storage.dirtyLock.Unlock()
		self.storage.Backend.StoreBlock(self)
		self.Backend = self.storage.Backend
		self.storage.dirtyLock.Lock()
		ops++
	} else {
		if self.Stored.Status != self.Status {
			self.flushStatus()
			ops = ops + 1
		}
		ops += self.storage.Backend.UpdateBlock(self)
	}
	self.Stored = nil
	self.addStorageRefCount(-1)
	return ops
}

func (self *Block) flushStatus() {
	// self.Stored.status != self.status
	if self.Status == BlockStatus_MISSING {
		// old type = NORMAL
		return
	}
	if self.Status == BlockStatus_WANT_WEAK {
		// old type = WEAK
		return
	}
	self.storage.updateBlockDataDependencies(self, true, self.Status)
	self.storage.updateBlockDataDependencies(self, false, self.Stored.Status)

}

func (self *Block) getCacheSize() int {
	s := int(unsafe.Sizeof(*self))
	return s + len(self.Id) + len(self.Data)
}

func (self *Block) addRefCount(count int32) {
	self.storage.dirtyLock.AssertLocked()
	mlog.Printf2("storage/block", "b.addRefCount %p %v -> %v", self, count, self.RefCount+count)
	self.markDirty()
	hadRefs := self.Stored.RefCount != 0
	haveRefs := self.RefCount != 0
	self.RefCount += count
	if hadRefs != haveRefs {
		mlog.Printf2("storage/block", " dependencies changed")
		self.storage.updateBlockDataDependencies(self, haveRefs, self.Status)
	}
}

func (self *Block) setStatus(st BlockStatus) {
	self.storage.dirtyLock.AssertLocked()
	mlog.Printf2("storage/block", "setStatus %p %v", self, st)
	self.markDirty()
	self.Status = st

}

func (self *BlockMapAtomicPointer) Add(b *Block) bool {
	for {
		m := self.Get()
		_, ok := m.Load(b.Id)
		if ok {
			return false
		}
		nm := m.Store(b.Id, b)
		if self.SetIfEqualTo(nm, m) {
			return true
		}
		mlog.Printf2("storage/block", "bmap.Add failed")
	}
}

func (self *BlockMapAtomicPointer) Remove(b *Block) bool {
	for {
		m := self.Get()
		v, ok := m.Load(b.Id)
		if !ok {
			return false
		}
		if v != b {
			return false
		}
		nm := m.Delete(b.Id)
		if self.SetIfEqualTo(nm, m) {
			return true
		}
		mlog.Printf2("storage/block", "bmap.Remove failed")
	}
}

func (self *Block) addStorageRefCount(v int32) {
	nv := atomic.AddInt32(&self.storageRefCount, v)
	mlog.Printf2("storage/block", "b.addStorageRefCount %p: %v -> %v", self, v, nv)
	if nv <= 0 {
		if nv < 0 {
			log.Panic("Negative reference count", nv)
		}
		if self.Stored != nil {
			log.Panic("Storage reference count before flush?")
		}
		mlog.Printf2("storage/block", " removed block %x", self.Id)
		self.storage.blocks.Remove(self)
	}
}

func (self *Block) Close() {
	mlog.Printf2("storage/block", "b.Close %p", self)
	self.addStorageRefCount(-1)
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
	self.storage.dirtyBlocks.Add(self)
}

// GetBlockById returns Block (if any) that matches id. The block
// reference MUST be eventually Close()d.
func (self *Storage) GetBlockById(id string) *Block {
	mlog.Printf2("storage/block", "st.GetBlockById %x", id)
	for {
		b, ok := self.blocks.Get().Load(id)
		if !ok {
			b = self.Backend.GetBlockById(id)
			if b == nil {
				mlog.Printf2("storage/block", " does not exist according to backend")
				return nil
			}
			b.storage = self
			b.addStorageRefCount(1)
			self.blocks.Add(b)
		} else {
			b.addStorageRefCount(1)
		}

		// Ensure no crafty remove+add occurred in parallel
		b2, ok2 := self.blocks.Get().Load(id)
		if ok2 && b2 == b {
			return b
		}

		mlog.Printf2("storage/block", " parallel access, retry GetBlockById")

		// We added reference just moment ago, just in case
		b.addStorageRefCount(-1)
	}
}

func (self *Storage) gocBlockById(id string) *Block {
	mlog.Printf2("storage/block", "st.gocBlockById %x", id)
	for {
		b := self.GetBlockById(id)
		if b != nil {
			return b
		}
		b = &Block{Id: id, storage: self}
		b.addStorageRefCount(1)
		if self.blocks.Add(b) {
			return b
		}
		b.addStorageRefCount(-1)
	}
}
