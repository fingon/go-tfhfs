/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2018 Markus Stenberg
 *
 * Created:       Fri Feb 16 10:11:10 2018 mstenber
 * Last modified: Tue Mar 13 10:32:47 2018 mstenber
 * Edit time:     286 min
 *
 */

package tree

import (
	"fmt"
	"log"
	"os"

	"github.com/fingon/go-tfhfs/codec"
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
	Superblock

	storage.DirectoryBackendBase
	storage.NameInBlockBackend
	lock                util.MutexLocked
	f                   *os.File
	tree                *ibtree.Tree
	root                *ibtree.Node
	rootBlockId         ibtree.BlockId
	t                   *ibtree.Transaction
	freeSize2OffsetTree *ibtree.SubTree // (size,offset)
	freeOffset2SizeTree *ibtree.SubTree // (offset, size)
	blockTree           *ibtree.SubTree // (block id => block data)
	currentMap          map[ibtree.BlockId]bool
	superIndex          int
	flushing            bool
}

var _ storage.Backend = &treeBackend{}

func (self *treeBackend) Init(config storage.BackendConfiguration) {
	self.DirectoryBackendBase.Init(config)
	self.NameInBlockBackend.Init("names", self)

	if self.Codec == nil {
		self.Codec = codec.CodecChain{}.Init()
	}

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
	var best *Superblock
	for i := 0; i < numberOfSuperBlocks(uint64(fi.Size())); i++ {
		ofs := superBlockOffset(i)
		b := self.readData(LocationSlice{LocationEntry{Offset: ofs, Size: superBlockSize}})
		b, err := self.Codec.DecodeBytes(b, nil)
		if err != nil {
			// invalid superblocks are ignored
			continue
		}
		var sb Superblock
		_, err = sb.UnmarshalMsg(b)
		if err != nil {
			continue
		}
		if best == nil || sb.Generation > best.Generation {
			best = &sb
		}
	}
	if best == nil {
		// New tree
		self.root = self.tree.NewRoot()
		self.rootBlockId = ""
	} else {
		// Old tree
		self.rootBlockId = best.RootLocation.ToBlockId()
		self.root = self.tree.LoadRoot(self.rootBlockId)
		self.Superblock = *best

	}
	self.newTransaction(self.root)
	if best != nil {
		// Stick stuff in pending to tree, if any
		self.flushPending()
	}
}

func (self *treeBackend) newTransaction(root *ibtree.Node) {
	self.t = ibtree.NewTransaction(root)
	self.freeSize2OffsetTree = self.t.NewSubTree(ibtree.Key("s"))
	self.freeOffset2SizeTree = self.t.NewSubTree(ibtree.Key("o"))
	self.blockTree = self.t.NewSubTree(ibtree.Key("b"))
}

func (self *treeBackend) Close() {
	// assume we've been flushed..
	self.f.Close()
}

func (self *treeBackend) getBlockData(id string) *BlockData {
	var bd BlockData
	k := ibtree.Key(id)
	v := self.blockTree.Get(k)
	if v == nil {
		return nil
	}
	bv := []byte(*v)
	mlog.Printf2("storage/tree/tree", "getBlockData %x", bv)
	_, err := bd.UnmarshalMsg(bv)
	if err != nil {
		mlog.Panicf("Unable to read %v: %s", k, err)
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

func (self *treeBackend) appendOp(le LocationEntry, free bool) {
	op := OpEntry{Location: le, Free: free}
	self.Pending = append(self.Pending, op)
	mlog.Printf2("storage/tree/tree", "appendOp %v", op)
}

func (self *treeBackend) appendFrees(ls LocationSlice) {
	for _, le := range ls {
		self.appendOp(le, true)
		self.BytesUsed -= le.BlockSize()
	}
}

func (self *treeBackend) addFreeTree(le LocationEntry) {
	self.freeSize2OffsetTree.Set(le.ToKeySO(), "")
	self.freeOffset2SizeTree.Set(le.ToKeyOS(), "")
}

func (self *treeBackend) addFree(le LocationEntry) {
	if self.flushing {
		self.appendOp(le, true)
		// the subsequent .Sets hit temporary tree; ^ is
		// what gets persisted later on
	}
	self.BytesUsed -= le.BlockSize()
	self.addFreeTree(le)
}

func (self *treeBackend) removeFreeTree(le LocationEntry) {
	self.freeSize2OffsetTree.Delete(le.ToKeySO())
	self.freeOffset2SizeTree.Delete(le.ToKeyOS())
}

func (self *treeBackend) removeFree(le LocationEntry) {
	if self.flushing {
		self.appendOp(le, false)
		// the subsequent .Deletes hit temporary tree; ^ is
		// what gets persisted later on
	}
	self.BytesUsed += le.BlockSize()
	self.removeFreeTree(le)
}

func (self *treeBackend) String() string {
	return fmt.Sprintf("tb{%p}", self)
}

func (self *treeBackend) allocate(size uint64) LocationSlice {
	mlog.Printf2("storage/tree/tree", "%v.allocate %v", self, size)
	sl := make(LocationSlice, 0, 1)
	for size > 0 {
		// [1] single existing allocation if possible
		want := LocationEntry{Size: size}
		bsize := want.BlockSize()
		wantkey := want.ToKeySO()
		kp := self.freeSize2OffsetTree.NextKey(wantkey)
		if kp != nil {
			mlog.Printf2("storage/tree/tree", " [1] found enough")
			le := NewLocationEntryFromKeySO(*kp)
			self.removeFree(le)
			left := le.Size - bsize
			if left > 0 {
				// Insert new, smaller entry
				self.addFree(LocationEntry{Size: left,
					Offset: le.Offset + bsize})
			}
			sl = append(sl, LocationEntry{Size: size,
				Offset: le.Offset})
			return sl
		}

		// [2] grow if possible
		if self.grow(bsize) {
			mlog.Printf2("storage/tree/tree", " [2] grew")
			continue
		}

		// [3] partial existing allocation (times N)
		kp = self.freeSize2OffsetTree.PrevKey(wantkey)
		if kp != nil {
			mlog.Printf2("storage/tree/tree", " [3] found fragment")
			le := NewLocationEntryFromKeySO(*kp)
			self.removeFree(le)
			sl = append(sl, le)
			size -= le.Size
			continue
		}

		// [4] failure (free done allocations)
		mlog.Printf2("storage/tree/tree", " [4] failure")
		self.appendFrees(sl)
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
	mlog.Printf2("storage/tree/tree", "%v.grow %v", self, asize)
	oldsbs := numberOfSuperBlocks(self.BytesTotal)
	nsize := self.BytesTotal + asize
	newsbs := numberOfSuperBlocks(nsize)
	// Simple case if even with new size we do not cross
	// superblock boundary.
	if oldsbs == newsbs {
		// addFree implicitly reduces used
		self.BytesUsed += asize
		self.addFree(LocationEntry{Offset: self.BytesTotal, Size: asize})
		self.BytesTotal += asize
		mlog.Printf2("storage/tree/tree", " BytesTotal=%v", self.BytesTotal)
		return true
	}

	// We do; add one superblock and recurse
	ofs := superBlockOffset(oldsbs)
	mlog.Printf2("storage/tree/tree", " adding superblock to %v", ofs)
	if ofs > self.BytesTotal {
		// Add small allocation up to the added superblock
		// (but not big enough for what was originally asked
		// for)
		mlog.Printf2("storage/tree/tree", " adding small free area %v", ofs-self.BytesTotal)
		ssize := ofs - self.BytesTotal
		// addFree implicitly reduces used
		self.BytesUsed += ssize
		self.BytesTotal += ssize
		self.addFree(LocationEntry{Offset: self.BytesTotal, Size: ssize})
	}
	self.BytesTotal += superBlockSize
	self.BytesUsed += superBlockSize
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

func (self *treeBackend) purgeNonCurrent(nd *ibtree.NodeData, bid ibtree.BlockId) {
	mlog.Printf2("storage/tree/tree", "purgeNonCurrent %x", bid)
	// if we don't know its bid, it is probably 'fresh'
	if bid == "" {
		return
	}

	// any subtree we have seen, we ignore
	_, ok := self.currentMap[bid]
	if ok {
		mlog.Printf2("storage/tree/tree", " current")
		return
	}

	if !nd.Leafy {
		// Recurse
		for _, c := range nd.Children {
			bid2 := ibtree.BlockId(c.Value)
			self.purgeNonCurrent(self.LoadNode(bid2), bid2)
		}
	}
	// This block id is redundant, remove it
	ls := NewLocationSliceFromBlockId(bid)
	self.appendFrees(ls)
	mlog.Printf2("storage/tree/tree", " freeing")
}

func (self *treeBackend) flushPending() {
	if self.Pending == nil {
		return
	}
	for _, op := range self.Pending {
		mlog.Printf2("storage/tree/tree", " flushing %v", op)
		if op.Free {
			self.addFreeTree(op.Location)
		} else {
			self.removeFreeTree(op.Location)
		}
	}
	self.Pending = nil
}

func (self *treeBackend) Flush() {
	defer self.lock.Locked()()
	mlog.Printf2("storage/tree/tree", "%v.Flush", self)

	// in flushing mode, we do bonus add-frees, but store those
	// only in superblock (and at end of flush stick them to the
	// fresh tree)
	root := self.t.Root()

	// if no change, just gtfo
	if root == self.root {
		return
	}

	self.flushing = true
	self.newTransaction(root)
	newRoot, bid := root.Commit()

	// determine delta in blocks, using currentMap entries as
	// 'interesting' border
	mlog.Printf2("storage/tree/tree", " purging old")
	self.purgeNonCurrent(&self.root.NodeData, self.rootBlockId)

	// update superblock
	self.Generation++
	self.RootLocation = NewLocationSliceFromBlockId(bid)

	// Write superblock
	self.superIndex++
	si := self.superIndex % numberOfSuperBlocks(self.BytesTotal)
	ofs := superBlockOffset(si)
	mlog.Printf2("storage/tree/tree", " writing superblock %d @%d", si, ofs)
	b, err := self.Superblock.MarshalMsg(nil)
	if err != nil {
		log.Panic(err)
	}
	b, err = self.Codec.EncodeBytes(b, nil)
	if err != nil {
		log.Panic(err)
	}
	if len(b) > superBlockSize {
		mlog.Panicf("Too large superblock: %v > %v", len(b), superBlockSize)
	}

	ls := LocationSlice{LocationEntry{Size: uint64(len(b)), Offset: ofs}}
	self.writeData(ls, b)

	self.rootBlockId = bid

	// Throw away the temporary root
	self.newTransaction(newRoot)

	self.flushing = false

	// Stick stuff in pending to tree, if any
	self.flushPending()

	// Clever bit: Use the post-flush root as base so we do not
	// cause subsequent flushes just based on flushPending
	self.root = self.t.Root()

	// Definition of 'current' is invalidated by this
	self.currentMap = make(map[ibtree.BlockId]bool)

}

func (self *treeBackend) DeleteBlock(b *storage.Block) {
	defer self.lock.Locked()()
	bd := self.getBlockData(b.Id)
	if bd == nil {
		mlog.Panicf("Nonexistent DeleteBlock: %v", b)
	}
	self.appendFrees(bd.Location)
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

func (self *treeBackend) setBlockData(id string, bdata *BlockData) {
	b, err := bdata.MarshalMsg(nil)
	if err != nil {
		log.Panic(err)
	}
	mlog.Printf2("storage/tree/tree", "setBlockData %x", b)
	self.blockTree.Set(ibtree.Key(id), string(b))
}

func (self *treeBackend) StoreBlock(bl *storage.Block) {
	defer self.lock.Locked()()
	b := *bl.Data.Get()
	ls := self.allocate(uint64(len(b)))
	self.writeData(ls, b)
	bdata := BlockData{Location: ls, BlockMetadata: bl.BlockMetadata}
	self.setBlockData(bl.Id, &bdata)
}

func (self *treeBackend) UpdateBlock(bl *storage.Block) int {
	defer self.lock.Locked()()
	bd := self.getBlockData(bl.Id)
	bd.BlockMetadata = bl.BlockMetadata
	self.setBlockData(bl.Id, bd)
	return 1
}

func NewTreeBackend() storage.Backend {
	self := &treeBackend{}
	self.currentMap = make(map[ibtree.BlockId]bool)
	return self
}

func (self *treeBackend) SaveNode(nd *ibtree.NodeData) ibtree.BlockId {
	if !nd.Leafy {
		// Note that intermediate nodes we refer to are also 'recent'
		for _, c := range nd.Children {
			self.currentMap[ibtree.BlockId(c.Value)] = true
		}
	}
	b := nd.ToBytes()
	b, err := self.Codec.EncodeBytes(b, nil)
	if err != nil {
		return ""
	}
	ls := self.allocate(uint64(len(b)))
	self.writeData(ls, b)
	bid := ls.ToBlockId()
	self.currentMap[bid] = true
	return bid
}

func (self *treeBackend) LoadNode(id ibtree.BlockId) *ibtree.NodeData {
	ls := NewLocationSliceFromBlockId(id)
	b := self.readData(ls)
	b, err := self.Codec.DecodeBytes(b, nil)
	if err != nil {
		return nil
	}
	return ibtree.NewNodeDataFromBytes(b)
}

func (self *treeBackend) GetBytesUsed() uint64 {
	return self.BytesUsed
}

func (self *treeBackend) GetBytesAvailable() uint64 {
	return self.DirectoryBackendBase.GetBytesAvailable() + self.BytesTotal - self.BytesUsed
}
