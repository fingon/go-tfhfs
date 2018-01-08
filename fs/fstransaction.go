/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2018 Markus Stenberg
 *
 * Created:       Fri Jan  5 16:40:08 2018 mstenber
 * Last modified: Mon Jan  8 22:52:40 2018 mstenber
 * Edit time:     81 min
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
	closed       bool
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
	return &fsTransaction{fs: fs, originalRoot: originalRoot, root: root,
		t: ibtree.NewTransaction(root.node)}
}

// CommitUntilSucceeds repeats commit until it the transaction goes
// through. This should be done only if the resource under question is
// locked by other means, as otherwise conflicting writes can occur.
// In general, using e.g. fs.Update() should be done in all cases.
func (self *fsTransaction) CommitUntilSucceeds() {
	defer self.Close()
	self.commit(true, false)
}

// TryCommit attempts to commit once, but if the tree has changed
// underneath, it will not hold.
func (self *fsTransaction) TryCommit() bool {
	return self.commit(false, false)
}

func (self *fsTransaction) commit(retryUntilSucceeds, recursed bool) bool {
	mlog.Printf2("fs/fstransaction", "fst.Commit")
	if self.closed {
		log.Panicf("Trying to commit closed transaction")
	}
	node, bid := self.t.Commit()
	if node == self.originalRoot.node {
		mlog.Printf2("fs/fstransaction", " no changes for fst.Commit")
		return true
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

		if !retryUntilSucceeds {
			return false
		}

		if !recursed {
			defer self.fs.transactionRetryLock.Locked()()
			mlog.Printf2("fs/fstransaction", " retrying")
		}

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
		tr.commit(true, true)
		return true
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
	return true
}

func (self *fsTransaction) Close() {
	// mlog.Printf2("fs/fstransaction", "fst.Close")
	if self.closed {
		mlog.Printf2("fs/fstransaction", " duplicate but it is ok")
		return
	}
	self.closed = true
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
		if self.blocks == nil {
			self.blocks = make([]*storage.StorageBlock, 0)
		}
		self.blocks = append(self.blocks, bl)
	}
	return bl
}

// Update (repeatedly) calls cb until it manages to update the global
// state with the content of the transaction. Therefore cb should be
// idempotent.
func (self *Fs) Update(cb func(tr *fsTransaction)) {
	mlog.Printf2("fs/fstransaction", "fs.Update")
	first := true
	for {
		// Initial one we will try without lock, as cb() may
		// take awhile.
		tr := self.GetTransaction()
		defer tr.Close()
		cb(tr)

		if tr.TryCommit() {
			return
		}
		if first {
			// Subsequent ones we want lock for, as we do
			// not want there to be a race that
			// potentially never ends to update the global
			// root node.
			defer self.fs.transactionRetryLock.Locked()()
			first = false
		}
		mlog.Printf2("fs/fstransaction", " retrying fs.Update")
	}
}
