/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2018 Markus Stenberg
 *
 * Created:       Fri Jan  5 16:40:08 2018 mstenber
 * Last modified: Fri Jan  5 23:31:23 2018 mstenber
 * Edit time:     49 min
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
	fs           *Fs
	originalRoot *fsTreeRoot
	root         fsTreeRoot
	t            *ibtree.IBTransaction
	blocks       []*storage.StorageBlock
}

func newFsTransaction(fs *Fs) *fsTransaction {
	originalRoot := fs.root.Get()
	root := *originalRoot
	// +1 ref when transaction starts (old root copy we got)
	if root.block != nil {
		bid := root.block.Id()
		mlog.Printf2("fs/fstransaction", "newFsTransaction - root id:%x", bid)
		fs.storage.ReferStorageBlockId(bid)
	}
	return &fsTransaction{fs, originalRoot, root,
		ibtree.NewTransaction(root.node), make([]*storage.StorageBlock, 0)}
}

func (self *fsTransaction) Commit() {
	mlog.Printf2("fs/fstransaction", "fst.Commit")
	defer self.Close()
	node, bid := self.t.Commit()
	if node == self.originalRoot.node {
		mlog.Printf2("fs/fstransaction", " no changes for fst.Commit")
		return
	}
	// +1 ref for new root (that we are about to store)
	block := self.fs.storage.GetBlockById(string(bid))
	if block == nil {
		log.Panicf("immediate commit + get = nil for %x", string(bid))
	}
	root := &fsTreeRoot{node, block}
	if !self.fs.root.SetIfEqualTo(root, self.originalRoot) {
		// block not stored anywhere
		defer block.Close()

		mlog.Printf2("fs/fstransaction", " root has changed under us; doing delta")
		tr := newFsTransaction(self.fs)
		node.IterateDelta(self.originalRoot.node,
			func(oldC, newC *ibtree.IBNodeDataChild) {
				if newC == nil {
					// Delete
					v := tr.t.Get(oldC.Key)
					if v != nil {
						mlog.Printf2("fs/fstransaction", " delete %x", oldC.Key)
						tr.t.Delete(oldC.Key)
					}
				} else if oldC == nil {
					// Insert
					mlog.Printf2("fs/fstransaction", " insert %x", newC.Key)
					tr.t.Set(newC.Key, newC.Value)
				} else {
					// Update
					mlog.Printf2("fs/fstransaction", " update %x", newC.Key)
					tr.t.Set(newC.Key, newC.Value)
				}
			})
		mlog.Printf2("fs/fstransaction", " delta done")
		tr.Commit()
		return
	}
	// In thory there is a race here; in practise I doubt it very
	// much it matters (as we next update will anyway have us
	// sticking in the updated version of the tree, as it was
	// correctly updated)
	self.fs.storage.SetNameToBlockId(self.fs.rootName, string(bid))
	if self.originalRoot.block != nil {
		// -1 ref for old root
		self.originalRoot.block.Close()
	}
}

func (self *fsTransaction) Close() {
	mlog.Printf2("fs/fstransaction", "fst.Close")
	// -1 ref when transaction expires (old root)
	if self.root.block == nil {
		mlog.Printf2("fs/fstransaction", " no root")
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
