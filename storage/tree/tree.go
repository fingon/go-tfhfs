/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2018 Markus Stenberg
 *
 * Created:       Fri Feb 16 10:11:10 2018 mstenber
 * Last modified: Wed Feb 21 11:44:16 2018 mstenber
 * Edit time:     133 min
 *
 */

package tree

import (
	"encoding/binary"
	"fmt"
	"log"
	"os"

	"github.com/fingon/go-tfhfs/ibtree"
	"github.com/fingon/go-tfhfs/mlog"
	"github.com/fingon/go-tfhfs/storage"
	"github.com/fingon/go-tfhfs/util"
)

const blockSize = 1 << 9
const treeNodeMaximumSize = 1 << 12
const superBlockSize = 1 << 16

// treeBackend provides storage on top of flat 'device'; in practise
// it may be in truth a number of files, or single raw disk device, or
// something else.
//
// It has its own ibtree subtree for:
// - free space (actually two; offset -> size, size -> offset)
// - block name => data + location mapping
type treeBackend struct {
	storage.DirectoryBackendBase
	storage.NameInBlockBackend
	lock                util.MutexLocked
	f                   *os.File
	tree                *ibtree.Tree
	oldRoot             *ibtree.Node
	t                   *ibtree.Transaction
	freeSize2OffsetTree *ibtree.SubTree // (size,offset)
	freeOffset2SizeTree *ibtree.SubTree // (offset, size)
	blockTree           *ibtree.SubTree // (block id => block data)
	recentMap           map[ibtree.BlockId]bool
	pendingFree         LocationSlice
	super               Superblock
	superIndex          int
}

var _ storage.Backend = &treeBackend{}

func (self *treeBackend) Init(config storage.BackendConfiguration) {
	self.DirectoryBackendBase.Init(config)
	self.NameInBlockBackend.Init("names", self)
	filepath := fmt.Sprintf("%s/db", config.Directory)
	f, err := os.OpenFile(filepath, os.O_RDWR|os.O_CREATE, 0755)
	if err != nil {
		mlog.Panicf("Unable to open %s: %s", filepath, err)
	}
	self.f = f
	fi, err := f.Stat()
	if err != nil {
		mlog.Panicf("Unable to stat %s: %s", filepath, err)
	}

	self.tree = ibtree.Tree{NodeMaximumSize: treeNodeMaximumSize}.Init(self)
	if fi.Size() < superBlockSize {
		// New tree
		self.oldRoot = self.tree.NewRoot()
	} else {
		// Old tree
		// TBD load most recent superblock, load root from there
	}
	self.t = ibtree.NewTransaction(self.oldRoot)
	self.freeSize2OffsetTree = self.t.NewSubTree(ibtree.Key("s"))
	self.freeOffset2SizeTree = self.t.NewSubTree(ibtree.Key("o"))
	self.blockTree = self.t.NewSubTree(ibtree.Key("b"))
}

func (self *treeBackend) Close() {
	// assume we've been flushed..
	self.f.Close()
}

func (self *treeBackend) appendLocation(sl *LocationSlice, data LocationSlice) {
	for _, v := range data {
		*sl = append(*sl, v)
	}
}

func (self *treeBackend) getBlockData(id string) *BlockData {
	var bd BlockData
	v := self.blockTree.Get(ibtree.Key(id))
	if v == nil {
		mlog.Panicf("Nonexistent block: %v", id)
	}
	bv := []byte(*v)
	_, err := bd.UnmarshalMsg(bv)
	if err != nil {
		mlog.Panicf("Unable to read %v: %s", id, err)
	}
	return &bd
}

func (self *treeBackend) readData(location LocationSlice) []byte {
	l := uint64(0)
	for _, v := range location {
		l += v.Size
	}
	b := make([]byte, l)
	ofs := uint64(0)
	for _, v := range location {
		_, err := self.f.Seek(int64(v.Offset), 0)
		if err != nil {
			log.Panic(err)
		}
		_, err = self.f.Read(b[ofs : ofs+v.Size])
		if err != nil {
			log.Panic(err)
		}
		ofs += v.Size
	}
	return b
}

func (self LocationEntry) ToKeySO() ibtree.Key {
	return ibtree.Key(util.ConcatBytes(util.Uint64Bytes(self.Size),
		util.Uint64Bytes(self.Offset)))
}

func (self LocationEntry) ToKeyOS() ibtree.Key {
	return ibtree.Key(util.ConcatBytes(util.Uint64Bytes(self.Offset),
		util.Uint64Bytes(self.Size)))
}

func NewLocationEntryFromKeySO(key ibtree.Key) LocationEntry {
	b := []byte(key)
	s := binary.BigEndian.Uint64(b)
	o := binary.BigEndian.Uint64(b[8:])
	return LocationEntry{Size: s, Offset: o}
}

func (self *treeBackend) addFree(le LocationEntry) {
	self.freeSize2OffsetTree.Set(le.ToKeySO(), "")
	self.freeOffset2SizeTree.Set(le.ToKeyOS(), "")
}

func (self *treeBackend) allocate(size uint64) LocationSlice {
	sl := make(LocationSlice, 0, 1)
	for size > 0 {
		// [1] single existing allocation if possible
		// asize = allocation size
		asize := size
		if asize%blockSize != 0 {
			asize += blockSize - asize%blockSize
		}
		wantkey := LocationEntry{Size: asize}.ToKeySO()
		kp := self.freeSize2OffsetTree.NextKey(wantkey)
		if kp != nil {
			le := NewLocationEntryFromKeySO(*kp)
			self.freeSize2OffsetTree.Delete(*kp)
			self.freeOffset2SizeTree.Delete(le.ToKeyOS())
			if le.Size != asize {
				// Insert new, smaller entry
				self.addFree(LocationEntry{Size: le.Size - asize,
					Offset: le.Offset + asize})
			}
			sl = append(sl, LocationEntry{Size: size,
				Offset: le.Offset})
			return sl
		}

		// [2] grow if possible
		if self.grow(asize) {
			continue
		}

		// [3] partial existing allocation (times N)
		kp = self.freeSize2OffsetTree.PrevKey(wantkey)
		if kp != nil {
			le := NewLocationEntryFromKeySO(*kp)
			self.freeSize2OffsetTree.Delete(*kp)
			self.freeOffset2SizeTree.Delete(le.ToKeyOS())

			sl = append(sl, le)
			size -= le.Size
			continue
		}

		// [4] failure (free done allocations)
		self.appendLocation(&self.pendingFree, sl)
		return nil
	}
	return sl
}

func superBlockOffset(i int) uint64 {
	if i == 0 {
		return 0
	}
	i--
	ofs := uint64(1024 * 1024)
	for i > 0 {
		ofs = ofs * 16
		i--
	}
	return ofs
}

func numberOfSuperBlocks(s uint64) int {
	if s == 0 {
		return 0
	}
	i := 1
	for s >= superBlockOffset(i) {
		i++
	}
	return i
}

func (self *treeBackend) grow(asize uint64) bool {
	oldsbs := numberOfSuperBlocks(self.super.Size)
	nsize := self.super.Size + asize
	newsbs := numberOfSuperBlocks(nsize)
	// Simple case if even with new size we do not cross
	// superblock boundary.
	if oldsbs == newsbs {
		self.addFree(LocationEntry{Offset: self.super.Size, Size: asize})
		self.super.Size += asize
		return true
	}

	// We do; add one superblock and recurse
	ofs := superBlockOffset(oldsbs)
	if ofs > self.super.Size {
		// Add small allocation up to the added superblock
		// (but not big enough for what was originally asked
		// for)
		self.addFree(LocationEntry{Offset: self.super.Size,
			Size: ofs - self.super.Size})
	}
	self.super.Size = ofs + superBlockSize
	return self.grow(asize)
}

func (self *treeBackend) writeData(location LocationSlice, data []byte) {
	ofs := uint64(0)
	for _, v := range location {
		_, err := self.f.Seek(int64(v.Offset), 0)
		if err != nil {
			log.Panic(err)
		}
		_, err = self.f.Write(data[ofs : ofs+v.Size])
		if err != nil {
			log.Panic(err)
		}
		ofs += v.Size
	}
}

func (self *treeBackend) Flush() {
	// TBD: get enough space to flush tree
	// TBD: store pendingFree -> free tree
	// TBD: commit tree
	// TBD: update superblock (incl. bonus free+allocs)
	// TBD: add bonus allocs to tree
}

func (self *treeBackend) DeleteBlock(b *storage.Block) {
	defer self.lock.Locked()()
	bd := self.getBlockData(b.Id)
	if bd == nil {
		mlog.Panicf("Nonexistent DeleteBlock: %v", b)
	}
	self.appendLocation(&self.pendingFree, bd.Location)
	self.blockTree.Delete(ibtree.Key(b.Id))
}

func (self *treeBackend) GetBlockData(b *storage.Block) []byte {
	defer self.lock.Locked()()
	bd := self.getBlockData(b.Id)
	if bd == nil {
		return nil
	}
	return self.readData(bd.Location)
}

func (self *treeBackend) GetBlockById(id string) *storage.Block {
	defer self.lock.Locked()()
	bd := self.getBlockData(id)
	if bd == nil {
		return nil
	}
	b := &storage.Block{Backend: self, Id: id}
	b.RefCount = bd.RefCount
	b.Status = storage.BlockStatus(bd.Status)
	return b
}

func (self *treeBackend) StoreBlock(b *storage.Block) {
	defer self.lock.Locked()()
	// TBD
}

func (self *treeBackend) UpdateBlock(b *storage.Block) int {
	defer self.lock.Locked()()
	// TBD
	return 1
}

func NewTreeBackend() storage.Backend {
	self := &treeBackend{}
	self.recentMap = make(map[ibtree.BlockId]bool)
	return self
}

func (self *treeBackend) SaveNode(nd *ibtree.NodeData) ibtree.BlockId {
	// TBD
	return ""
}

func (self *treeBackend) LoadNode(id ibtree.BlockId) *ibtree.NodeData {
	// TBD
	return nil
}
