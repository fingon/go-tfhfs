/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2017 Markus Stenberg
 *
 * Created:       Thu Dec 14 19:10:02 2017 mstenber
 * Last modified: Fri Jan  5 02:54:35 2018 mstenber
 * Edit time:     438 min
 *
 */

package storage

import (
	"log"

	"github.com/fingon/go-tfhfs/codec"
	"github.com/fingon/go-tfhfs/mlog"
	"github.com/fingon/go-tfhfs/util"
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
type BlockIterateReferencesCallback func(string, []byte, BlockReferenceCallback)

// Storage is essentially DelayedStorage of Python prototype; it has
// dirty tracking of blocks, delayed flush to BlockBackend, and
// caching of data.
type oldNewStruct struct{ old_value, new_value string }
type Storage struct {
	Backend                   BlockBackend
	IterateReferencesCallback BlockIterateReferencesCallback
	Codec                     codec.Codec

	// blocks is Block object herd; they are reference counted, so
	// as long as someone keeps a reference to one, it stays
	// here. Being in dirtyBlocks means it also has extra
	// reference.
	blocks BlockMapAtomicPointer

	// Blocks that are dirty
	dirtyBlocks BlockMapAtomicPointer
	dirtyLock   util.MutexLocked

	// Stuff below here is ~DelayedStorage
	names    map[string]*oldNewStruct
	nameLock util.MutexLocked

	reads, writes, readbytes, writebytes int
}

// Init sets up the default values to be usable
func (self Storage) Init() *Storage {
	self.names = make(map[string]*oldNewStruct)
	// No need to special case Codec = nil elsewhere with this
	if self.Codec == nil {
		self.Codec = &codec.CodecChain{}
	}
	return &self
}

func (self *Storage) flushBlockName(k string, v *oldNewStruct) {
	mlog.Printf2("storage/storage", "flushBlockName %s=%x", k, v.new_value)
	self.Backend.SetNameToBlockId(k, v.new_value)
	if v.new_value != "" {
		bl := self.GetBlockById(v.new_value)
		bl.addRefCount(1)
		bl.Close()
	}
	if v.old_value != "" {
		self.releaseBlockId(v.old_value)
	}
	v.old_value = v.new_value
}

func (self *Storage) flushBlockNames() int {
	defer self.nameLock.Locked()()
	defer self.dirtyLock.Locked()()
	ops := 0
	for k, v := range self.names {
		if v.old_value != v.new_value {
			self.flushBlockName(k, v)
			ops++
		}
	}
	return ops
}

func (self *Storage) Flush() int {
	mlog.Printf2("storage/storage", "st.Flush")
	mlog.Printf2("storage/storage", " reads since last flush: %d - %d k", self.reads, self.reads/1024)
	mlog.Printf2("storage/storage", " writes since last flush: %d - %d k", self.writes, self.writebytes/1024)
	mlog.Printf2("storage/storage", " blocks:%d (%d dirty)",
		self.blocks.Get().Len(),
		self.dirtyBlocks.Get().Len())
	self.reads = 0
	self.readbytes = 0
	self.writes = 0
	self.writebytes = 0
	self.Backend.SetInFlush(true)
	defer self.Backend.SetInFlush(false)

	// _flush_names in Python prototype
	ops := self.flushBlockNames()

	// flush_dirty_stored_blocks in Python
	for {
		s := self.dirtyBlocks.Get()
		oops := ops
		mlog.Printf2("storage/storage", " flush_dirty_stored_blocks; %d to go", s.Len())
		// first nonzero refcounts as they may add references;
		// then zero refcounts as they reduce references
		for i := 0; i < 2; i++ {
			s.Range(func(_ string, b *Block) bool {
				if (b.RefCount == 0) == (i == 1) {
					ops += b.flush()
				}
				return true
			})
		}
		if ops == oops {
			break
		}
	}

	mlog.Printf2("storage/storage", " ops:%v", ops)
	return ops
}

func (self *Storage) GetBlockIdByName(name string) string {
	defer self.nameLock.Locked()()
	return self.getName(name).new_value
}

func (self *Storage) ReferOrStoreBlock(id string, data []byte) *Block {
	b := self.ReferOrStoreBlock0(id, data)
	if b != nil {
		defer self.dirtyLock.Locked()()
		b.addRefCount(1)
	}
	return b
}

func (self *Storage) ReferOrStoreBlock0(id string, data []byte) *Block {
	b := self.GetBlockById(id)
	if b != nil {
		defer self.dirtyLock.Locked()()
		return b
	}
	return self.StoreBlock0(id, data, BlockStatus_NORMAL)
}

// ReleaseBlockId will eventually release block (in Flush), if its
// refcnt is zero.
func (self *Storage) ReleaseBlockId(id string) {
	defer self.dirtyLock.Locked()()
	self.releaseBlockId(id)
}

func (self *Storage) releaseBlockId(id string) {
	b := self.GetBlockById(id)
	if b == nil {
		log.Panicf("block id %x disappeared", id)
	}
	b.addRefCount(-1)
	b.Close()
}

func (self *Storage) SetNameToBlockId(name, block_id string) {
	defer self.nameLock.Locked()()
	self.getName(name).new_value = block_id
}

func (self *Storage) StoreBlock0(id string, data []byte, status BlockStatus) *Block {
	mlog.Printf2("storage/storage", "st.StoreBlock %x", id)
	b := self.gocBlockById(id)
	defer self.dirtyLock.Locked()()
	b.setStatus(status)
	b.Data = data
	return b

}

/// Private
func (self *Storage) updateBlockDataDependencies(b *Block, add bool, st BlockStatus) {
	mlog.Printf2("storage/storage", "st.updateBlockDataDependencies %p %v %v", b, add, st)
	defer mlog.Printf2("storage/storage", " st.updateBlockDataDependencies done")
	// No sub-references
	if st >= BlockStatus_WANT_NORMAL {
		return
	}
	if self.IterateReferencesCallback == nil {
		return
	}
	self.IterateReferencesCallback(b.Id, b.GetData(), func(id string) {
		if add {
			b := self.GetBlockById(id)
			if b == nil {
				log.Panicf("nonexistent block requested: %x", id)
			}
			b.addRefCount(1)
			b.Close()
		} else {
			self.releaseBlockId(id)
		}
	})
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
