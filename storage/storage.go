/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2017 Markus Stenberg
 *
 * Created:       Thu Dec 14 19:10:02 2017 mstenber
 * Last modified: Wed Mar 21 13:07:36 2018 mstenber
 * Edit time:     664 min
 *
 */

package storage

import (
	"github.com/fingon/go-tfhfs/codec"
	"github.com/fingon/go-tfhfs/mlog"
	"github.com/fingon/go-tfhfs/util"
	"github.com/minio/sha256-simd"
)

type BlockReferenceCallback func(string)
type BlockIterateReferencesCallback func(string, []byte, BlockReferenceCallback)

// Storage is essentially DelayedStorage of Python prototype; it has
// dirty tracking of blocks, delayed flush to Backend, and
// caching of data.
type oldNewStruct struct {
	oldValue, newValue string
	gotStorageRef      bool
}

type blockMap map[string]*Block

type blockObjectMap map[*Block]bool

const (
	C_READ = iota
	C_READBYTES
	C_WRITE
	C_WRITEBYTES
	C_DELETE
	NUM_C
)

type Storage struct {
	// Backend specifies the backend to use.
	Backend Backend

	// IterateReferencesCallback is used to find block references
	// inside block data.
	IterateReferencesCallback BlockIterateReferencesCallback

	// QueueLength specifies how long channel queue is there for
	// storage operations. Zero means unbuffered (which is fancy
	// for debugging but crap performance-wise).
	QueueLength int

	// Codec (if set) specifies the codec used to encode the data
	// before it is stored in backend, or to decode it when
	// fetching it from backend
	Codec codec.Codec

	// blocks is Block object herd; they are reference counted, so
	// as long as someone keeps a reference to one, it stays
	// here. Being in dirtyBlocks means it also has extra
	// storage-reference. dirtyRefBlocks on the other hand do NOT
	// have references, but consist of blocks with either recently
	// zeroed or non-zeroed storageRefCount
	blocks                             blockMap
	dirtyBlocks, dirtyStorageRefBlocks blockObjectMap

	// Stuff below here is ~DelayedStorage
	names map[string]*oldNewStruct

	counters [NUM_C]util.AtomicInt

	jobChannel chan *jobIn

	jobCounts map[jobType]int
}

// Init sets up the default values to be usable
func (self Storage) Init() *Storage {
	self.names = make(map[string]*oldNewStruct)
	self.jobChannel = make(chan *jobIn, self.QueueLength)
	self.blocks = make(blockMap)
	self.dirtyBlocks = make(blockObjectMap)
	self.dirtyStorageRefBlocks = make(blockObjectMap)
	self.jobCounts = make(map[jobType]int)

	if self.Codec != nil {
		// No need to care about encoding elsewhere with this
		// (except server part)
		self.Backend = codecBackend{Codec: self.Codec}.SetBackend(self.Backend)
	} else {
		// Similarly provide nop Codec so code can always
		// assume Codec is present
		self.Codec = codec.CodecChain{}.Init()
	}

	self.Backend = mapRunnerBackend{}.SetBackend(self.Backend)

	go func() { // ok, singleton per storage
		self.run()
	}()
	return &self
}

func (self *Storage) Close() {
	// Implicitly also flush; storage that persists randomly seems bad
	if self.Backend != nil {
		self.Flush()
	}

	out := make(chan *jobOut)
	self.jobChannel <- &jobIn{jobType: jobQuit, out: out}
	<-out

	if self.Backend != nil {
		mlog.Printf2("storage/storage", "Storage also closing Backend")
		self.Backend.Close()
	}
}

func (self *Storage) TransientCount() int {
	// mlog.Printf2("storage/storage", "TransientCount")
	transient := 0
	for _, b := range self.blocks {
		if b.RefCount == 0 {
			// mlog.Printf2("storage/storage", " %v", b)
			transient++
		}
	}
	return transient
}

func (self *Storage) setNameToBlockId(name, bid string) {
	n := self.getName(name)
	if bid != "" {
		self.getBlockById(bid).addStorageRefCount(1)
	}
	if n.gotStorageRef {
		self.getBlockById(n.newValue).addStorageRefCount(-1)
	}
	n.newValue = bid
	n.gotStorageRef = true
}

func (self *Storage) getName(name string) *oldNewStruct {
	n, ok := self.names[name]
	if ok {
		return n
	}
	id := self.Backend.GetBlockIdByName(name)
	n = &oldNewStruct{oldValue: id, newValue: id}
	self.names[name] = n
	return n
}

func (self *Storage) flushBlockName(k string, v *oldNewStruct) {
	mlog.Printf2("storage/storage", "flushBlockName %s=%x", k, v.newValue)
	self.Backend.SetNameToBlockId(k, v.newValue)
	if v.newValue != "" {
		b := self.getBlockById(v.newValue)
		b.addRefCount(1)
		b.addStorageRefCount(-1)
		v.gotStorageRef = false
	}
	if v.oldValue != "" {
		b := self.getBlockById(v.oldValue)
		b.addRefCount(-1)
	}
	v.oldValue = v.newValue
}

func (self *Storage) flushBlockNames() int {
	ops := 0
	for k, v := range self.names {
		if v.oldValue != v.newValue {
			self.flushBlockName(k, v)
			ops++
		}
	}
	return ops
}

func (self *Storage) flush() int {
	mlog.Printf2("storage/storage", "st.Flush")
	var c [NUM_C]int64
	for i := 0; i < NUM_C; i++ {
		c[i] = self.counters[i].Get()
	}
	if c[C_READ] > 0 {
		mlog.Printf2("storage/storage", " reads since last flush: %d - %d k", c[C_READ], c[C_READBYTES]/1024)
	}
	if c[C_WRITE] > 0 {
		mlog.Printf2("storage/storage", " writes since last flush: %d - %d k", c[C_WRITE], c[C_WRITEBYTES]/1024)
	}
	if c[C_DELETE] > 0 {
		mlog.Printf2("storage/storage", " deletes since last flush: %d", c[C_DELETE])
	}
	mlog.Printf2("storage/storage", " blocks:%d (%d dirty, %d transient)",
		len(self.blocks),
		len(self.dirtyBlocks),
		self.TransientCount())
	if mlog.IsEnabled() {
		total := 0
		for _, v := range self.jobCounts {
			total += v
		}
		for k, v := range self.jobCounts {
			if v > 0 {
				mlog.Printf2("storage/storage", " %v %d", k, v)
			}
		}
	}
	for i := 0; i < NUM_C; i++ {
		self.counters[i].Set(0)
	}

	// _flush_names in Python prototype
	ops := self.flushBlockNames()

	// flush_dirty_stored_blocks in Python
	for len(self.dirtyBlocks) > 0 {
		oops := ops
		mlog.Printf2("storage/storage", " flushing %d dirty", len(self.dirtyBlocks))
		// first nonzero refcounts as they may add references;
		// then zero refcounts as they reduce references
		for b, _ := range self.dirtyBlocks {
			if b.RefCount != 0 {
				ops += b.flush()
			}
		}
		if ops != oops {
			continue
		}

		// only removals left
		mlog.Printf2("storage/storage", " flushing refcnt=0")
		for b, _ := range self.dirtyBlocks {
			if b.RefCount != 0 {
				break
			}
			ops += b.flush()
		}
	}

	// similarly handle the storageRefCounts
	for len(self.dirtyStorageRefBlocks) > 0 {
		oops := ops
		for b, _ := range self.dirtyStorageRefBlocks {
			if b.storageRefCount != 0 {
				ops += b.flushStorageRef()
			}
		}
		if ops != oops {
			continue
		}
		for b, _ := range self.dirtyStorageRefBlocks {
			if b.storageRefCount != 0 {
				break
			}
			ops += b.flushStorageRef()
		}
	}

	if self.Backend != nil && (ops > 0 || c[C_WRITE] > 0 || c[C_DELETE] > 0) {
		self.Backend.Flush()
	}

	mlog.Printf2("storage/storage", " ops:%v", ops)
	return ops
}

func (self *Storage) ReferOrStoreBlockBytes0(status BlockStatus, b []byte, deps *util.StringList) *StorageBlock {
	h := sha256.Sum256(b)
	bid := h[:]
	id := string(bid)
	bl := self.ReferOrStoreBlock0(id, status, b, deps)
	return bl
}
