/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2017 Markus Stenberg
 *
 * Created:       Thu Dec 14 19:10:02 2017 mstenber
 * Last modified: Fri Jan  5 14:27:40 2018 mstenber
 * Edit time:     499 min
 *
 */

package storage

import (
	"log"

	"github.com/fingon/go-tfhfs/codec"
	"github.com/fingon/go-tfhfs/mlog"
)

type BlockReferenceCallback func(string)
type BlockIterateReferencesCallback func(string, []byte, BlockReferenceCallback)

// Storage is essentially DelayedStorage of Python prototype; it has
// dirty tracking of blocks, delayed flush to Backend, and
// caching of data.
type oldNewStruct struct{ old_value, new_value string }

type jobOut struct {
	sb *StorageBlock
	id string
}

const (
	jobFlush int = iota
	jobGetBlockById
	jobGetBlockIdByName
	jobReferOrStoreBlock     // ReferOrStoreBlock, ReferOrStoreBlock0
	jobUpdateBlockIdRefCount // ReferBlockId, ReleaseBlockId
	jobSetNameToBlockId
	jobStoreBlock // StoreBlock, StoreBlock0
	jobQuit
)

type jobIn struct {
	// see job* above
	jobType int

	sb *StorageBlock

	// in jobReferOrStoreBlock, jobUpdateBlockIdRefCount, jobStoreBlock
	count int32

	// block id
	id string

	// block name
	name string

	// block data
	data []byte

	status BlockStatus

	out chan *jobOut
}

type blockMap map[string]*Block

type Storage struct {
	Backend                   Backend
	IterateReferencesCallback BlockIterateReferencesCallback
	Codec                     codec.Codec

	// blocks is Block object herd; they are reference counted, so
	// as long as someone keeps a reference to one, it stays
	// here. Being in dirtyBlocks means it also has extra
	// reference.
	blocks, dirtyBlocks blockMap

	// Stuff below here is ~DelayedStorage
	names map[string]*oldNewStruct

	reads, writes, readbytes, writebytes int

	jobChannel chan *jobIn
}

// Init sets up the default values to be usable
func (self Storage) Init() *Storage {
	self.names = make(map[string]*oldNewStruct)
	self.jobChannel = make(chan *jobIn)
	self.blocks = make(blockMap)
	self.dirtyBlocks = make(blockMap)
	// No need to special case Codec = nil elsewhere with this
	if self.Codec == nil {
		self.Codec = &codec.CodecChain{}
	}
	go func() {
		self.run()
	}()
	return &self
}

func (self *Storage) Close() {
	out := make(chan *jobOut)
	self.jobChannel <- &jobIn{jobType: jobQuit, out: out}
	<-out
}

func (self *Storage) run() {
	for job := range self.jobChannel {
		switch job.jobType {
		case jobQuit:
			job.out <- nil
			return
		case jobFlush:
			self.flush()
		case jobGetBlockById:
			b := self.getBlockById(job.id)
			job.out <- &jobOut{sb: NewStorageBlock(b)}
		case jobGetBlockIdByName:
			job.out <- &jobOut{id: self.getName(job.name).new_value}
		case jobReferOrStoreBlock:
			b := self.getBlockById(job.id)
			if b != nil {
				b.addRefCount(job.count)
				job.out <- &jobOut{sb: NewStorageBlock(b)}
				continue
			}
			job.status = BlockStatus_NORMAL
			fallthrough
		case jobStoreBlock:
			b := self.gocBlockById(job.id)
			b.setStatus(job.status)
			b.addRefCount(job.count)
			b.Data = job.data
			job.out <- &jobOut{sb: NewStorageBlock(b)}
		case jobUpdateBlockIdRefCount:
			b := self.getBlockById(job.id)
			if b == nil {
				log.Panicf("block id %x disappeared", job.id)
			}
			b.addRefCount(job.count)
		case jobSetNameToBlockId:
			self.getName(job.name).new_value = job.id
		default:
			log.Panicf("Unknown job type: %d", job.jobType)
		}
	}
}

func (self *Storage) Flush() {
	self.jobChannel <- &jobIn{jobType: jobFlush}
}

func (self *Storage) GetBlockById(id string) *StorageBlock {
	out := make(chan *jobOut)
	self.jobChannel <- &jobIn{jobType: jobGetBlockById, out: out,
		id: id,
	}
	jr := <-out
	return jr.sb
}

func (self *Storage) GetBlockIdByName(name string) string {
	out := make(chan *jobOut)
	self.jobChannel <- &jobIn{jobType: jobGetBlockIdByName, out: out,
		name: name,
	}
	jr := <-out
	return jr.id
}

func (self *Storage) storeBlockInternal(jobType int, id string, data []byte, count int32) *StorageBlock {
	out := make(chan *jobOut)
	self.jobChannel <- &jobIn{jobType: jobType, out: out,
		id: id, data: data, count: count, status: BlockStatus_NORMAL,
	}
	jr := <-out
	return jr.sb
}

func (self *Storage) ReferOrStoreBlock(id string, data []byte) *StorageBlock {
	return self.storeBlockInternal(jobReferOrStoreBlock, id, data, 1)
}

func (self *Storage) ReferOrStoreBlock0(id string, data []byte) *StorageBlock {
	return self.storeBlockInternal(jobReferOrStoreBlock, id, data, 0)
}

func (self *Storage) ReferBlockId(id string) {
	self.jobChannel <- &jobIn{jobType: jobUpdateBlockIdRefCount,
		id: id, count: 1,
	}
}

func (self *Storage) ReleaseBlockId(id string) {
	self.jobChannel <- &jobIn{jobType: jobUpdateBlockIdRefCount,
		id: id, count: -1,
	}
}

func (self *Storage) SetNameToBlockId(name, block_id string) {
	self.jobChannel <- &jobIn{jobType: jobSetNameToBlockId,
		id: block_id, name: name,
	}
}

func (self *Storage) StoreBlock(id string, data []byte) *StorageBlock {
	return self.storeBlockInternal(jobStoreBlock, id, data, 1)
}

func (self *Storage) StoreBlock0(id string, data []byte) *StorageBlock {
	return self.storeBlockInternal(jobStoreBlock, id, data, 0)
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
			self.ReferBlockId(id)
		} else {
			self.ReleaseBlockId(id)
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

func (self *Storage) flushBlockName(k string, v *oldNewStruct) {
	mlog.Printf2("storage/storage", "flushBlockName %s=%x", k, v.new_value)
	self.Backend.SetNameToBlockId(k, v.new_value)
	if v.new_value != "" {
		self.ReferBlockId(v.new_value)
	}
	if v.old_value != "" {
		self.ReleaseBlockId(v.old_value)
	}
	v.old_value = v.new_value
}

func (self *Storage) flushBlockNames() int {
	ops := 0
	for k, v := range self.names {
		if v.old_value != v.new_value {
			self.flushBlockName(k, v)
			ops++
		}
	}
	return ops
}

func (self *Storage) flush() int {
	mlog.Printf2("storage/storage", "st.Flush")
	mlog.Printf2("storage/storage", " reads since last flush: %d - %d k", self.reads, self.reads/1024)
	mlog.Printf2("storage/storage", " writes since last flush: %d - %d k", self.writes, self.writebytes/1024)
	mlog.Printf2("storage/storage", " blocks:%d (%d dirty)",
		len(self.blocks),
		len(self.dirtyBlocks))
	self.reads = 0
	self.readbytes = 0
	self.writes = 0
	self.writebytes = 0

	// _flush_names in Python prototype
	ops := self.flushBlockNames()

	// flush_dirty_stored_blocks in Python
	for len(self.dirtyBlocks) > 0 {
		oops := ops
		mlog.Printf2("storage/storage", " flush_dirty_stored_blocks; %d to go", len(self.dirtyBlocks))
		// first nonzero refcounts as they may add references;
		// then zero refcounts as they reduce references
		for _, b := range self.dirtyBlocks {
			if b.RefCount != 0 {
				ops += b.flush()
			}
		}
		if ops != oops {
			continue
		}

		// only removals left
		for _, b := range self.dirtyBlocks {
			ops += b.flush()
		}
	}

	mlog.Printf2("storage/storage", " ops:%v", ops)
	return ops
}
