/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2017 Markus Stenberg
 *
 * Created:       Thu Dec 14 19:10:02 2017 mstenber
 * Last modified: Tue Jan  9 10:48:31 2018 mstenber
 * Edit time:     553 min
 *
 */

package storage

import (
	"fmt"
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

type jobType int

const (
	jobFlush jobType = iota
	jobGetBlockById
	jobGetBlockIdByName
	jobSetNameToBlockId
	jobReferOrStoreBlock            // ReferOrStoreBlock, ReferOrStoreBlock0
	jobUpdateBlockIdRefCount        // ReferBlockId, ReleaseBlockId
	jobUpdateBlockIdStorageRefCount // ReleaseStorageBlockId
	jobStoreBlock                   // StoreBlock, StoreBlock0
	jobQuit
)

func (self jobType) String() string {
	switch self {
	case jobFlush:
		return "jobFlush"
	case jobGetBlockById:
		return "jobGetBlockById"
	case jobGetBlockIdByName:
		return "jobGetBlockIdByName"
	case jobSetNameToBlockId:
		return "jobSetNameToBlockId"
	case jobReferOrStoreBlock:
		return "jobReferOrStoreBlock"
	case jobUpdateBlockIdRefCount:
		return "jobUpdateBlockIdRefCount"
	case jobUpdateBlockIdStorageRefCount:
		return "jobUpdateBlockIdStorageRefCount"
	case jobStoreBlock:
		return "jobStoreBlock"
	case jobQuit:
		return "jobQuit"
	default:
		return fmt.Sprintf("%d", int(self))
	}
}

type jobIn struct {
	// see job* above
	jobType jobType

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
	blocks, dirtyBlocks, ref0Blocks blockMap

	// Stuff below here is ~DelayedStorage
	names map[string]*oldNewStruct

	reads, writes, readbytes, writebytes int

	jobChannel chan *jobIn
}

// Init sets up the default values to be usable
func (self Storage) Init() *Storage {
	self.names = make(map[string]*oldNewStruct)
	self.jobChannel = make(chan *jobIn, 100)
	self.blocks = make(blockMap)
	self.dirtyBlocks = make(blockMap)
	self.ref0Blocks = make(blockMap)

	// No need to special case Codec = nil elsewhere with this
	if self.Codec != nil {
		self.Backend = codecBackend{}.Init(self.Backend, self.Codec)
	}
	// self.Backend = fanoutBackend{}.Init(self.Backend)
	go func() {
		self.run()
	}()
	return &self
}

func (self *Storage) Close() {
	// Implicitly also flush; storage that persists randomly seems bad
	self.Flush()

	out := make(chan *jobOut)
	self.jobChannel <- &jobIn{jobType: jobQuit, out: out}
	<-out
}

func (self *Storage) run() {
	for job := range self.jobChannel {
		mlog.Printf2("storage/storage", "st.run job %v", job.jobType)
		switch job.jobType {
		case jobQuit:
			job.out <- nil
			return
		case jobFlush:
			self.flush()
			job.out <- nil
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
			mlog.Printf2("storage/storage", "fallthrough to storing block")
			fallthrough
		case jobStoreBlock:
			b := &Block{Id: job.id,
				storage: self,
			}
			b.Data.Set(&job.data)
			self.blocks[job.id] = b
			b.setStatus(job.status)
			b.addRefCount(job.count)
			job.out <- &jobOut{sb: NewStorageBlock(b)}
		case jobUpdateBlockIdRefCount:
			b := self.getBlockById(job.id)
			if b == nil {
				log.Panicf("block id %x disappeared", job.id)
			}
			b.addRefCount(job.count)
		case jobUpdateBlockIdStorageRefCount:
			b := self.getBlockById(job.id)
			if b == nil {
				log.Panicf("block id %x disappeared", job.id)
			}
			b.addStorageRefCount(job.count)
		case jobSetNameToBlockId:
			self.setNameToBlockId(job.name, job.id)
		default:
			log.Panicf("Unknown job type: %d", job.jobType)
		}
		mlog.Printf2("storage/storage", " st.run job done")
	}
}

func (self *Storage) Flush() {
	out := make(chan *jobOut)
	self.jobChannel <- &jobIn{jobType: jobFlush, out: out}
	<-out
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

func (self *Storage) setNameToBlockId(name, bid string) {
	if bid != "" {
		self.getBlockById(bid).addStorageRefCount(1)
	}
	n := self.getName(name)
	if n.new_value != "" {
		self.getBlockById(n.new_value).addStorageRefCount(-1)
	}
	n.new_value = bid
}

func (self *Storage) storeBlockInternal(jobType jobType, id string, data []byte, count int32) *StorageBlock {
	if data == nil {
		mlog.Printf2("storage/storage", "no data given")
	}
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

func (self *Storage) ReferStorageBlockId(id string) {
	mlog.Printf2("storage/storage", "ReferStorageBlockId %x", id)
	self.jobChannel <- &jobIn{jobType: jobUpdateBlockIdStorageRefCount,
		id: id, count: 1,
	}
}

func (self *Storage) ReleaseBlockId(id string) {
	self.jobChannel <- &jobIn{jobType: jobUpdateBlockIdRefCount,
		id: id, count: -1,
	}
}

func (self *Storage) ReleaseStorageBlockId(id string) {
	mlog.Printf2("storage/storage", "ReleaseStorageBlockId %x", id)
	self.jobChannel <- &jobIn{jobType: jobUpdateBlockIdStorageRefCount,
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
		b := self.getBlockById(id)
		if b == nil {
			log.Panicf("Block %x awol in updateBlockDataDependencies", id)
		}
		if add {
			b.addRefCount(1)
		} else {
			b.addRefCount(-1)
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
		b := self.getBlockById(v.new_value)
		b.addRefCount(1)
		// the new_value retains its +1 storage refcount as
		// otherwise bookkeeping gets ugly in
		// e.g. setNameToBlockId.
	}
	if v.old_value != "" {
		b := self.getBlockById(v.old_value)
		b.addRefCount(-1)
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
		mlog.Printf2("storage/storage", " flushing %d dirty", len(self.dirtyBlocks))
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
		mlog.Printf2("storage/storage", " flushing refcnt=0")
		for _, b := range self.dirtyBlocks {
			if b.RefCount != 0 {
				break
			}
			ops += b.flush()
		}
	}

	if len(self.ref0Blocks) > 0 {
		for _, b := range self.ref0Blocks {
			if b.storageRefCount == 0 {
				mlog.Printf2("storage/storage", " removing %v", b)
				delete(self.blocks, b.Id)
			}
		}
		self.ref0Blocks = make(blockMap)
	}

	mlog.Printf2("storage/storage", " ops:%v", ops)
	return ops
}
