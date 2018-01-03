/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2017 Markus Stenberg
 *
 * Created:       Thu Dec 14 19:10:02 2017 mstenber
 * Last modified: Wed Jan  3 23:24:15 2018 mstenber
 * Edit time:     322 min
 *
 */

package storage

import (
	"sort"

	"github.com/fingon/go-tfhfs/codec"
	"github.com/fingon/go-tfhfs/mlog"
)

// BlockBackend is the shadow behind the throne; it actually
// handles the low-level operations of blocks.
type BlockBackend interface {
	// Close the backend
	Close()

	// DeleteBlock removes block from storage, and it MUST exist.
	DeleteBlock(b *Block)

	// GetBlockData retrieves lazily (if need be) block data
	GetBlockData(b *Block) []byte

	// GetBlockById returns block by id or nil.
	GetBlockById(id string) *Block

	// GetBlockIdByName returns block id mapped to particular name.
	GetBlockIdByName(name string) string

	// GetBytesAvailable returns number of bytes available.
	GetBytesAvailable() uint64

	// GetBytesUsed returns number of bytes used.
	GetBytesUsed() uint64

	// Update inflush status
	SetInFlush(bool)

	// SetBlockIdName sets the logical name to map to particular block id.
	SetNameToBlockId(name, block_id string)

	// StoreBlock adds new block to storage. It MUST NOT exist.
	StoreBlock(b *Block)

	// UpdateBlock updates block metadata in storage. It MUST exist.
	UpdateBlock(b *Block) int
}

type BlockReferenceCallback func(string)
type BlockIterateReferencesCallback func([]byte, BlockReferenceCallback)
type BlockHasExternalReferencesCallback func(string) bool

// Storage is essentially DelayedStorage of Python prototype; it has
// dirty tracking of blocks, delayed flush to BlockBackend, and
// caching of data.
type oldNewStruct struct{ old_value, new_value string }
type Storage struct {
	Backend                       BlockBackend
	IterateReferencesCallback     BlockIterateReferencesCallback
	HasExternalReferencesCallback BlockHasExternalReferencesCallback
	Codec                         codec.Codec
	MaximumCacheSize              int

	// Map of block id => block for dirty blocks.
	dirtyBid2Block map[string]*Block

	// Blocks that have refcnt0 but BlockHasExternalReferencesCallback has
	// claimed they should still be around
	referencedRefcnt0Blocks map[string]*Block

	// Stuff below here is ~DelayedStorage
	names          map[string]*oldNewStruct
	cacheBid2Block map[string]*Block
	cacheSize      int
	t              uint64

	reads, writes, readbytes, writebytes int
}

// Init sets up the default values to be usable
func (self Storage) Init() *Storage {
	self.dirtyBid2Block = make(map[string]*Block)
	self.cacheBid2Block = make(map[string]*Block)
	self.names = make(map[string]*oldNewStruct)
	// No need to special case Codec = nil elsewhere with this
	if self.Codec == nil {
		self.Codec = &codec.CodecChain{}
	}
	return &self
}

func (self *Storage) flushBlockName(k string, v *oldNewStruct) {
	mlog.Printf2("storage/storage", "flushBlockName %s=%x", k, v.new_value)
	if v.old_value != "" {
		self.ReleaseBlockId(v.old_value)
	}
	self.Backend.SetNameToBlockId(k, v.new_value)
	if v.new_value != "" {
		self.ReferBlockId(v.new_value)
	}
	v.old_value = v.new_value

}

func (self *Storage) Flush() int {
	mlog.Printf2("storage/storage", "st.Flush")
	mlog.Printf2("storage/storage", " reads since last flush: %d - %d k", self.reads, self.reads/1024)
	mlog.Printf2("storage/storage", " writes since last flush: %d - %d k", self.writes, self.writebytes/1024)
	self.reads = 0
	self.readbytes = 0
	self.writes = 0
	self.writebytes = 0
	mlog.Printf2("storage/storage", " cache size:%v", self.cacheSize)
	self.Backend.SetInFlush(true)
	defer self.Backend.SetInFlush(false)
	ops := 0
	// _flush_names in Python prototype
	for k, v := range self.names {
		if v.old_value != v.new_value {
			self.flushBlockName(k, v)
			ops = ops + 1
		}
	}
	// Main flush in Python prototype; handles deletion
	for self.referencedRefcnt0Blocks != nil {
		s := self.referencedRefcnt0Blocks
		mlog.Printf2("storage/storage", " flush (delete); %d candidates", len(s))
		self.referencedRefcnt0Blocks = nil
		oops := ops
		for _, v := range s {
			if v.RefCount == 0 && self.deleteBlockIfNoExtRef(v) {
				ops = ops + 1
			}
		}
		// If we didn't manage to kill any blocks, rest have
		// external references that are current.
		if oops == ops {
			break
		}
	}

	// flush_dirty_stored_blocks in Python
	for len(self.dirtyBid2Block) > 0 {
		mlog.Printf2("storage/storage", " flush_dirty_stored_blocks; %d to go", len(self.dirtyBid2Block))
		dirty := self.dirtyBid2Block
		self.dirtyBid2Block = make(map[string]*Block, len(dirty))
		nonzero_blocks := make([]*Block, 0, len(dirty))
		for _, b := range dirty {
			if b.RefCount == 0 {
				ops = ops + b.flush()
			} else {
				nonzero_blocks = append(nonzero_blocks, b)
			}
		}
		for _, b := range nonzero_blocks {
			if b.RefCount > 0 {
				ops = ops + b.flush()
			} else {
				// populate for subsequent round
				self.dirtyBid2Block[b.Id] = b
			}
		}
	}

	// end of flush in DelayedStorage in Python prototype
	if self.MaximumCacheSize > 0 && self.cacheSize > self.MaximumCacheSize {
		self.shrinkCache()
	}
	mlog.Printf2("storage/storage", " ops:%v, cache size:%v", ops, self.cacheSize)
	return ops
}

func (self *Storage) GetBlockIdByName(name string) string {
	return self.getName(name).new_value
}

func (self *Storage) ReferBlockId(id string) {
	b := self.GetBlockById(id)
	if b == nil {
		panic("block id disappeared")
	}
	b.setRefCount(b.RefCount + 1)
}

func (self *Storage) ReferOrStoreBlock(id string, data []byte) *Block {
	b := self.GetBlockById(id)
	if b != nil {
		self.ReferBlockId(id)
		return b
	}
	return self.StoreBlock(id, data, BlockStatus_NORMAL)
}

// ReleaseBlockId will eventually release block (in Flush), if its
// refcnt is zero.
func (self *Storage) ReleaseBlockId(id string) {
	b := self.GetBlockById(id)
	if b == nil {
		panic("block id disappeared")
	}
	b.setRefCount(b.RefCount - 1)
}

func (self *Storage) SetNameToBlockId(name, block_id string) {
	self.getName(name).new_value = block_id
}

func (self *Storage) StoreBlock(id string, data []byte, status BlockStatus) *Block {
	b := self.gocBlockById(id)
	b.setRefCount(1)
	b.setStatus(status)
	b.Data = data
	self.cacheSize += b.getCacheSize()
	self.updateBlockDataDependencies(data, true, status)
	return b

}

/// Private
func (self *Storage) updateBlockDataDependencies(data []byte, add bool, st BlockStatus) {
	// No sub-references
	if st >= BlockStatus_WANT_NORMAL {
		return
	}
	if self.IterateReferencesCallback == nil {
		return
	}
	self.IterateReferencesCallback(data, func(id string) {
		if add {
			self.ReferBlockId(id)
		} else {
			self.ReleaseBlockId(id)
		}
	})
}

// getBlockById is the old Storage version; GetBlockIdBy is the external one
func (self *Storage) getBlockById(id string) *Block {
	b := self.dirtyBid2Block[id]
	if self.blockValid(b) {
		return b
	}
	if self.referencedRefcnt0Blocks != nil {
		b := self.referencedRefcnt0Blocks[id]
		if self.blockValid(b) {
			return b
		}
	}
	b = self.Backend.GetBlockById(id)
	if b != nil {
		b.storage = self
	}
	return b
}

func (self *Storage) deleteBlockWithDeps(b *Block) bool {
	self.updateBlockDataDependencies(b.GetData(), false, b.Status)
	self.Backend.DeleteBlock(b)
	self.deleteCachedBlock(b)
	return true
}

func (self *Storage) deleteBlockIfNoExtRef(b *Block) bool {
	if self.HasExternalReferencesCallback != nil && self.HasExternalReferencesCallback(b.Id) {
		if self.referencedRefcnt0Blocks == nil {
			self.referencedRefcnt0Blocks = make(map[string]*Block)
		}
		self.referencedRefcnt0Blocks[b.Id] = b
		return false
	}
	if self.referencedRefcnt0Blocks != nil {
		delete(self.referencedRefcnt0Blocks, b.Id)
	}
	return self.deleteBlockWithDeps(b)
}

func (self *Storage) shrinkCache() {
	mlog.Printf2("storage/storage", "shrinkCache")
	n := len(self.cacheBid2Block)
	arr := make([]*Block, n)
	i := 0
	for _, v := range self.cacheBid2Block {
		arr[i] = v
		i = i + 1
	}
	sort.Slice(arr, func(i, j int) bool {
		return arr[i].t < arr[j].t
	})
	cnt := i
	i = 0
	goal := self.MaximumCacheSize * 3 / 4

	// recalculate cache size so we're actually doing correct
	// cleanup (TBD is this too expensive? probably not compared
	// to e.g. building and sorting the array above anyway)
	cs := 0
	for i = 0; i < n; i++ {
		cs += arr[i].getCacheSize()
	}
	self.cacheSize = cs

	for i = 0; i < n && self.cacheSize > goal; i++ {
		self.deleteCachedBlock(arr[i])
	}
	mlog.Printf2("storage/storage", " removed %d out of %d entries", i, cnt)

}

func (self *Storage) recalculateCacheSize() {

}

func (self *Storage) deleteCachedBlock(b *Block) {
	delete(self.cacheBid2Block, b.Id)
	self.cacheSize -= b.getCacheSize()
	if b.stored != nil && b.stored.RefCount == 0 {
		// Locally stored, never hit disk, but references did
		self.updateBlockDataDependencies(b.Data, false, b.Status)
	}
}

func (self *Storage) updateBlock(b *Block) int {
	if b.RefCount == 0 {
		if b.stored.RefCount == 0 {
			self.deleteCachedBlock(b)
			return 0
		}
		if self.deleteBlockIfNoExtRef(b) {
			return 1
		}
	}
	return self.Backend.UpdateBlock(b)
}

func (self *Storage) getName(name string) *oldNewStruct {
	n, ok := self.names[name]
	if ok {
		return n
	}
	id := self.Backend.GetBlockIdByName(name)
	n = &oldNewStruct{old_value: id, new_value: id}
	self.names[name] = n
	return n
}
