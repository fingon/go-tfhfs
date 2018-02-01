/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2018 Markus Stenberg
 *
 * Created:       Fri Jan  5 16:40:08 2018 mstenber
 * Last modified: Thu Feb  1 17:47:03 2018 mstenber
 * Edit time:     225 min
 *
 */

package hugger

import (
	"log"

	"github.com/fingon/go-tfhfs/ibtree"
	"github.com/fingon/go-tfhfs/mlog"
	"github.com/fingon/go-tfhfs/storage"
	"github.com/fingon/go-tfhfs/util"
)

type treeRoot struct {
	node  *ibtree.Node
	block *storage.StorageBlock
}

type Transaction struct {
	hugger *Hugger

	// root is the root we based this transaction on
	root *treeRoot

	// rootBlock is our copy of root.block (if any)
	rootBlock *storage.StorageBlock

	t         *ibtree.Transaction
	blocks    map[string]*storage.StorageBlock
	blockLock util.MutexLocked
	closed    bool
}

func newTransaction(h *Hugger, force bool) *Transaction {
	defer h.lock.Locked()()
	for !force && h.flushing {
		h.flushed.Wait()
	}
	root := h.root.Get()
	var rootBlock *storage.StorageBlock
	// +1 ref when transaction starts
	if root.block != nil {
		mlog.Printf2("ibtree/hugger/htransaction", "newTransaction - root:%v", root.block)
		rootBlock = root.block.Open()
	} else {
		mlog.Printf2("ibtree/hugger/htransaction", "newTransaction - no root block")
	}
	tr := &Transaction{hugger: h, root: root, rootBlock: rootBlock,
		t: ibtree.NewTransaction(root.node)}
	h.transactions[tr] = true
	return tr
}

func (self *Transaction) IB() *ibtree.Transaction {
	return self.t
}

// ibtree.TreeSaver API
func (self *Transaction) SaveNode(nd *ibtree.NodeData) ibtree.BlockId {
	b := NodeDataToBytes(nd)
	mlog.Printf2("ibtree/hugger/htransaction", "SaveNode %d bytes", len(b))
	sl := &util.StringList{}
	if self.hugger.IterateReferencesCallback != nil {
		self.hugger.IterateReferencesCallback(nd,
			func(s string) {
				sl.PushFront(s)
			})
	} else {
		// Fall back to storage-level reference iteration
		sl = nil
	}
	bl := self.GetStorageBlock(storage.BS_NORMAL, b, nd, sl)
	bid := ibtree.BlockId(bl.Id())
	return bid
}

// CommitUntilSucceeds repeats commit until it the transaction goes
// through. This should be done only if the resource under question is
// locked by other means, as otherwise conflicting writes can occur.
// In general, using e.g. fs.Update() should be done in all cases.
func (self *Transaction) CommitUntilSucceeds() {
	defer self.Close()
	self.commit(true, false)
}

// TryCommit attempts to commit once, but if the tree has changed
// underneath, it will not hold.
func (self *Transaction) TryCommit() bool {
	return self.commit(false, false)
}

func (self *Transaction) commit(retryUntilSucceeds, recursed bool) bool {
	mlog.Printf2("ibtree/hugger/htransaction", "ht.Commit")
	if self.closed {
		log.Panicf("Trying to commit closed transaction")
	}
	node, bid := self.t.CommitTo(self)
	if node == self.root.node {
		mlog.Printf2("ibtree/hugger/htransaction", " no changes for ht.Commit")
		return true
	}
	// +1 ref (essentially) for new root (that we are about to
	// store); if it winds up as new fs.root, the reference is
	// kept there and so we take it out of self.blocks.
	block := self.blocks[string(bid)]
	if block == nil {
		mlog.Panicf("Did not store what we claimed to?")
	}
	delete(self.blocks, string(bid))

	root := &treeRoot{node, block}
	if !self.hugger.root.SetIfEqualTo(root, self.root) {
		// -1 ref at end of scope as block did not make it to the fs.root
		defer block.Close()

		if !retryUntilSucceeds {
			return false
		}

		if !recursed {
			defer self.hugger.transactionRetryLock.Locked()()
			mlog.Printf2("ibtree/hugger/htransaction", " retrying")
		}

		mlog.Printf2("ibtree/hugger/htransaction", " root has changed under us; doing delta")
		tr := newTransaction(self.hugger, true)
		defer tr.Close()
		self.hugger.MergeCallback(tr, self.root.node, node, true)
		mlog.Printf2("ibtree/hugger/htransaction", " delta done")
		return tr.commit(true, true)
	}

	// In thory there is a race here; in practise I doubt it very
	// much it matters (as we next update will anyway have us
	// sticking in the updated version of the tree, as it was
	// correctly updated)
	self.hugger.Storage.SetNameToBlockId(self.hugger.RootName, string(bid))

	// Paranoia stuff
	if self.hugger.nodeDataCache == nil {
		mlog.Printf2("ibtree/hugger/htransaction", " after successful commit, tree rooted at %x:", string(bid))
		node.PrintToMLogAll()
	}

	// We replaced this with our own pointer, after this it will
	// not be reachable (and self.rootBlock is just our own
	// read-reference to the root which will be taken care of in
	// the transaction Close).
	if self.root.block != nil {
		defer self.hugger.lock.Locked()()
		self.hugger.oldRoots[self.root.block] = true
	}

	return true
}

func (self *Transaction) Close() {
	if self.closed {
		return
	}
	mlog.Printf2("ibtree/hugger/htransaction", "ht.Close")
	self.closed = true

	// Remove all temporary blocks acquired during the transaction
	for _, v := range self.blocks {
		v.Close()
	}

	self.hugger.closedTransaction(self)
}

// getStorageBlock block ids for given bytes/data.
// This one expires the blocks when the transaction is gone.
// nd and deps are optional, but may speed up processing (or not).
func (self *Transaction) GetStorageBlock(st storage.BlockStatus, b []byte, nd *ibtree.NodeData, deps *util.StringList) *storage.StorageBlock {
	if self.closed {
		mlog.Panicf("GetStorageBlock in closed transaction")
	}
	bl := self.hugger.Storage.ReferOrStoreBlockBytes0(st, b, deps)
	if nd != nil {
		self.hugger.SetCachedNodeData(ibtree.BlockId(bl.Id()), nd)
	}
	defer self.blockLock.Locked()()
	if self.blocks == nil {
		self.blocks = make(map[string]*storage.StorageBlock)
	}
	self.blocks[bl.Id()] = bl
	return bl
}
