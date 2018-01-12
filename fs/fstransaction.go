/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2018 Markus Stenberg
 *
 * Created:       Fri Jan  5 16:40:08 2018 mstenber
 * Last modified: Fri Jan 12 11:52:00 2018 mstenber
 * Edit time:     166 min
 *
 */

package fs

import (
	"log"

	"github.com/minio/sha256-simd"

	"github.com/fingon/go-tfhfs/ibtree"
	"github.com/fingon/go-tfhfs/mlog"
	"github.com/fingon/go-tfhfs/storage"
	"github.com/fingon/go-tfhfs/util"
)

type fsTreeRoot struct {
	node  *ibtree.IBNode
	block *storage.StorageBlock
}

type fsTransaction struct {
	fs *Fs

	// root is the root we based this transaction on
	root *fsTreeRoot

	// rootBlock is our copy of root.block (if any)
	rootBlock *storage.StorageBlock

	t         *ibtree.IBTransaction
	blocks    map[string]*storage.StorageBlock
	blockLock util.MutexLocked
	closed    bool
}

func newFsTransaction(fs *Fs) *fsTransaction {
	defer fs.transactionsLock.Locked()()
	root := fs.root.Get()
	var rootBlock *storage.StorageBlock
	// +1 ref when transaction starts
	if root.block != nil {
		mlog.Printf2("fs/fstransaction", "newFsTransaction - root:%v", root.block)
		rootBlock = root.block.Open()
	} else {
		mlog.Printf2("fs/fstransaction", "newFsTransaction - no root block")
	}
	tr := &fsTransaction{fs: fs, root: root, rootBlock: rootBlock,
		t: ibtree.NewTransaction(root.node)}
	fs.transactions[tr] = true
	return tr
}

// ibtree.IBTreeSaver API
func (self *fsTransaction) SaveNode(nd *ibtree.IBNodeData) ibtree.BlockId {
	if self.fs.nodeDataCache == nil {
		// In unit tests, ensure we're not saving garbage
		nd.CheckNodeStructure()

	}

	bb := make([]byte, nd.Msgsize()+1)
	bb[0] = byte(BDT_NODE)
	b, err := nd.MarshalMsg(bb[1:1])
	if err != nil {
		log.Panic(err)
	}
	b = bb[0 : 1+len(b)]
	mlog.Printf2("fs/fstransaction", "SaveNode %d bytes", len(b))
	bl := self.getStorageBlock(b, nd)
	bid := ibtree.BlockId(bl.Id())
	return bid
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
	node, bid := self.t.CommitTo(self)
	if node == self.root.node {
		mlog.Printf2("fs/fstransaction", " no changes for fst.Commit")
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

	root := &fsTreeRoot{node, block}
	if !self.fs.root.SetIfEqualTo(root, self.root) {
		// -1 ref at end of scope as block did not make it to the fs.root
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
		defer tr.Close()
		setIfNewer := func(newC *ibtree.IBNodeDataChild) {
			bk := blockKey(newC.Key)
			if bk.SubType() == BST_META {
				ourMeta := decodeInodeMeta(newC.Value)
				op := tr.t.Get(ibtree.IBKey(newC.Key))
				if op != nil {
					theirMeta := decodeInodeMeta(*op)
					if theirMeta.StAtimeNs > ourMeta.StAtimeNs {
						return
					}
				}
			}
			tr.t.Set(newC.Key, newC.Value)
		}
		node.IterateDelta(self.root.node,
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
					setIfNewer(newC)
				} else {
					// Update
					mlog.Printf2("fs/fstransaction", " update %x", newC.Key)
					setIfNewer(newC)
				}
			})
		mlog.Printf2("fs/fstransaction", " delta done")
		return tr.commit(true, true)
	}

	// In thory there is a race here; in practise I doubt it very
	// much it matters (as we next update will anyway have us
	// sticking in the updated version of the tree, as it was
	// correctly updated)
	self.fs.storage.SetNameToBlockId(self.fs.rootName, string(bid))

	// Paranoia stuff
	if self.fs.nodeDataCache == nil {
		mlog.Printf2("fs/fstransaction", " after successful commit, tree rooted at %x:", string(bid))
		node.PrintToMLogAll()

		// Ensure root metadata is still there
		k := ibtree.IBKey(NewblockKey(uint64(1), BST_META, ""))
		tr := self.fs.GetTransaction()
		defer tr.Close()
		v := tr.t.Get(k)
		if v == nil {
			mlog.Panicf("root metadata is gone")
		}
	}

	// We replaced this with our own pointer, after this it will
	// not be reachable (and self.rootBlock is just our own
	// read-reference to the root which will be taken care of in
	// the transaction Close).
	if self.root.block != nil {
		defer self.fs.transactionsLock.Locked()()
		self.fs.oldRoots[self.root.block] = true
	}

	return true
}

func (self *fsTransaction) Close() {
	if self.closed {
		return
	}
	mlog.Printf2("fs/fstransaction", "fst.Close")
	self.closed = true

	// Remove all temporary blocks acquired during the transaction
	for _, v := range self.blocks {
		v.Close()
	}

	defer self.fs.transactionsLock.Locked()()

	// -1 ref when transaction expires (old root)
	if self.rootBlock != nil {
		self.fs.oldRoots[self.rootBlock] = true
	}
	delete(self.fs.transactions, self)
}

// getStorageBlock block ids for given bytes/data.
// This one expires the blocks when the transaction is gone.
func (self *fsTransaction) getStorageBlock(b []byte, nd *ibtree.IBNodeData) *storage.StorageBlock {
	if self.closed {
		mlog.Panicf("getStorageBlock in closed transaction")
	}
	h := sha256.Sum256(b)
	bid := h[:]
	id := string(bid)
	if nd != nil && self.fs.nodeDataCache != nil {
		self.fs.nodeDataCache.Set(ibtree.BlockId(id), nd)
	}
	bl := self.fs.storage.ReferOrStoreBlock0(id, b)
	if bl == nil {
		return nil
	}
	defer self.blockLock.Locked()()
	if self.blocks == nil {
		self.blocks = make(map[string]*storage.StorageBlock)
	}
	self.blocks[bl.Id()] = bl
	return bl
}

// Update2 (repeatedly) calls cb until it manages to update the global
// state with the content of the transaction. Therefore cb should be
// idempotent. If cb returns false, the transaction will not be committed.
func (self *Fs) Update2(cb func(tr *fsTransaction) bool) {
	mlog.Printf2("fs/fstransaction", "fs.Update")
	first := true
	for {
		// Initial one we will try without lock, as cb() may
		// take awhile.
		tr := self.GetTransaction()
		defer tr.Close()
		if !cb(tr) {
			break
		}

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

// Update is the lazy variant in which the transaction supposedly
// always works. That may not be really the case in real world though,
// so Update2 should be used.
func (self *Fs) Update(cb func(tr *fsTransaction)) {
	self.Update2(func(tr *fsTransaction) bool {
		cb(tr)
		return true
	})
}
