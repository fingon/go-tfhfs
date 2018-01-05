/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2018 Markus Stenberg
 *
 * Created:       Fri Jan  5 16:40:08 2018 mstenber
 * Last modified: Fri Jan  5 22:34:06 2018 mstenber
 * Edit time:     30 min
 *
 */

package fs

import (
	"log"

	"github.com/fingon/go-tfhfs/ibtree"
	"github.com/fingon/go-tfhfs/mlog"
	"github.com/fingon/go-tfhfs/storage"
)

type fsTreeRoot struct {
	node  *ibtree.IBNode
	block *storage.StorageBlock
}

type fsTransaction struct {
	fs     *Fs
	root   fsTreeRoot
	t      *ibtree.IBTransaction
	blocks []*storage.StorageBlock
}

func newFsTransaction(fs *Fs) *fsTransaction {
	root := *fs.root.Get()
	// +1 ref when transaction starts
	if root.block != nil {
		bid := root.block.Id()
		mlog.Printf2("fs/fstransaction", "newFsTransaction - root id:%x", bid)
		fs.storage.ReferStorageBlockId(bid)
	}
	return &fsTransaction{fs, root,
		ibtree.NewTransaction(root.node), make([]*storage.StorageBlock, 0)}
}

func (self *fsTransaction) Commit() {
	mlog.Printf("fst.Commit")
	self.fs.lock.AssertLocked()
	node, bid := self.t.Commit()
	// +1 ref for new root
	block := self.fs.storage.GetBlockById(string(bid))
	if block == nil {
		log.Panicf("immediate commit + get = nil for %x", string(bid))
	}
	self.fs.storage.SetNameToBlockId(self.fs.rootName, string(bid))
	self.Close()
	// TBD: Should maybe do actual delta thing here, but this is
	// low-probability occurence hopefully..
	root := &fsTreeRoot{node, block}
	for {
		old := self.fs.root.Get()
		if self.fs.root.SetIfEqualTo(root, old) {
			if old.block != nil {
				// -1 ref for old root
				old.block.Close()
			}
			return
		}
	}
}

func (self *fsTransaction) Close() {
	mlog.Printf("fst.Close")
	// -1 ref when transaction expires
	if self.root.block == nil {
		mlog.Printf(" no root")
		return
	}
	for _, v := range self.blocks {
		v.Close()
	}
	self.fs.storage.ReleaseStorageBlockId(self.root.block.Id())
	self.root.block = nil
}

// getStorageBlock is convenience wrapper over the getStorageBlock in Fs.
// This one expires the blocks when the transaction is gone.
func (self *fsTransaction) getStorageBlock(b []byte, nd *ibtree.IBNodeData) *storage.StorageBlock {
	bl := self.fs.getStorageBlock(b, nd)
	if bl != nil {
		self.blocks = append(self.blocks, bl)
	}
	return bl
}
