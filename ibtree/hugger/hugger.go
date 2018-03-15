/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2018 Markus Stenberg
 *
 * Created:       Wed Jan 17 12:37:08 2018 mstenber
 * Last modified: Thu Mar 15 12:26:06 2018 mstenber
 * Edit time:     147 min
 *
 */

package hugger

import (
	"fmt"
	"log"
	"sync"

	"github.com/bluele/gcache"
	"github.com/fingon/go-tfhfs/ibtree"
	"github.com/fingon/go-tfhfs/mlog"
	"github.com/fingon/go-tfhfs/storage"
	"github.com/fingon/go-tfhfs/util"
)

type MergeCallback func(t *Transaction, src, dst *ibtree.Node, local bool)
type NodeIterateReferencesCallback func(*ibtree.NodeData, storage.BlockReferenceCallback)

// Hugger, or treehugger, is an abstraction which provides its own
// transaction mechanism (on top of Tree's own), and atomically
// updated root pointer to Node root. It also handles persistence to
// storage. It used to be part of fs/, but as both server/ and fs/
// need distinct trees, this was born and fstransaction and parts of
// fs were moved to hugger submodule.
type Hugger struct {
	RootName string
	Storage  *storage.Storage

	// IterateReferencesCallback should be provided; it is much
	// more efficient than fallback on storage's
	// IterateReferencesCallback as it does unmarshal+marshal
	IterateReferencesCallback NodeIterateReferencesCallback

	// MergeCallback MUST be provided if
	// Transaction.CommitUntilSucceeds is used. Otherwise it is
	// optional if only Transaction.TryCommit or Hugger.Update or
	// hugger.Update2 are used.
	MergeCallback MergeCallback

	tree          *ibtree.Tree
	root, oldRoot treeRootAtomicPointer
	nodeDataCache gcache.Cache

	// transactionRetryLock ensures there is only one active
	// retrying transaction.
	transactionRetryLock util.MutexLocked

	// lock protects transactions and roots
	lock util.MutexLocked

	// when flushing, new transactions will stall and wait for
	// Cond
	flushing bool

	flushed, transactionClosed sync.Cond

	// transactions is the map of active transactions
	transactions map[*Transaction]bool

	blocks    map[string]*storage.StorageBlock // map of allocations
	blockLock util.MutexLocked                 // covers blocks

}

func (self *Hugger) String() string {
	return fmt.Sprintf("H{rn:%s}", self.RootName)
}

func (self *Hugger) Init(cacheSize int) *Hugger {
	self.tree = ibtree.Tree{NodeMaximumSize: 4096}.Init(self)
	self.blocks = make(map[string]*storage.StorageBlock)
	self.transactions = make(map[*Transaction]bool)
	self.flushed.L = &self.lock
	self.transactionClosed.L = &self.lock
	if cacheSize > 0 {
		self.nodeDataCache = gcache.New(cacheSize).
			ARC().
			Build()
	}
	return self
}

// GetNestableTransaction attempts to provide a transaction even if
// flush is pending. It should be used only for short-lived things
// that are done _within_ other transactions (if GetTransaction is
// used within transactions, deadlock may occur).
func (self *Hugger) GetNestableTransaction() *Transaction {
	return newTransaction(self, true)
}

func (self *Hugger) GetTransaction() *Transaction {
	return newTransaction(self, false)
}

// Update2 (repeatedly) calls cb until it manages to update the global
// state with the content of the transaction. Therefore cb should be
// idempotent. If cb returns false, the transaction will not be committed.
func (self *Hugger) Update2(cb func(tr *Transaction) bool) {
	mlog.Printf2("ibtree/hugger/hugger", "fs.Update")
	first := true
	for {
		// Initial one we will try without lock, as cb() may
		// take awhile.
		tr := self.GetNestableTransaction()
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
			defer self.transactionRetryLock.Locked()()
			first = false
		}
		mlog.Printf2("ibtree/hugger/hugger", " retrying fs.Update")
	}
}

// Update is the lazy variant in which the transaction supposedly
// always works. That may not be really the case in real world though,
// so Update2 should be used.
func (self *Hugger) Update(cb func(tr *Transaction)) {
	self.Update2(func(tr *Transaction) bool {
		cb(tr)
		return true
	})
}

// ibtree.TreeBackend API
func (self *Hugger) LoadNode(id ibtree.BlockId) *ibtree.NodeData {
	var v interface{}
	if self.nodeDataCache != nil {
		v, _ = self.nodeDataCache.Get(id)
	}
	if v == nil {
		b := self.Storage.GetBlockById(string(id))
		if b == nil {
			log.Panicf("Unable to find node %x", id)
		}
		defer b.Close()
		nd := ibtree.NewNodeDataFromBytes(b.Data())
		if nd == nil {
			log.Panicf("Unable to find node %x", id)
		}
		self.SetCachedNodeData(id, nd)
		return nd
	}
	mlog.Printf2("ibtree/hugger/hugger", "fs.LoadNode found %x in cache: %p", id, v)
	return v.(*ibtree.NodeData)
}

func (self *Hugger) Flush() {
	defer self.lock.Locked()()
	mlog.Printf2("ibtree/hugger/hugger", "%v.Flush", self)
	self.flushing = true
	for len(self.transactions) > 0 {
		mlog.Printf2("ibtree/hugger/hugger", "%s.Flush waiting %d transactions", self, len(self.transactions))
		if mlog.IsEnabled() {
			for t, _ := range self.transactions {
				mlog.Printf2("ibtree/hugger/hugger", " %v", t)
			}
		}
		self.transactionClosed.Wait()
	}

	or := self.oldRoot.Get()
	r := self.root.Get()
	if or == nil || or.node != r.node {
		mlog.Printf2("ibtree/hugger/hugger", " Flush calling CommitTo")
		// Houston, we have new root!
		node, bid := r.node.CommitTo(self)

		defer self.blockLock.Locked()()

		block, ok := self.blocks[string(bid)]

		if !ok {
			// Get block ref it refers to (if any)
			block = self.Storage.GetBlockById(string(bid))
			if block == nil {
				mlog.Printf2("ibtree/hugger/hugger", "Non-existent root block %x", bid)
			}
		} else {
			// Remove it from the blocks (sref owned by us)
			delete(self.blocks, string(bid))
		}

		r = &treeRoot{node: node, block: block}
		self.root.Set(r)
		self.oldRoot.Set(r)

		self.Storage.SetNameToBlockId(self.RootName, string(bid))

		// If we had 'old root', remove its reference (even if
		// it was same, CommitTo added one ref to it)
		if or != nil && or.block != nil {
			mlog.Printf2("ibtree/hugger/hugger", " Flush letting old root go")
			or.block.Close()
		}
	} else {
		mlog.Printf2("ibtree/hugger/hugger", " Flush has nothing to do")
		defer self.blockLock.Locked()()
	}
	if len(self.blocks) > 0 {
		// Remove all temporary blocks acquired during transactions
		for _, v := range self.blocks {
			mlog.Printf2("ibtree/hugger/hugger", " Flush closing %v", v)
			v.Close()
		}
		self.blocks = make(map[string]*storage.StorageBlock)
	}

	self.flushing = false
	mlog.Printf2("ibtree/hugger/hugger", "%s.Flush done", self)
	self.flushed.Broadcast()
}

func (self *Hugger) GetCachedNodeData(id ibtree.BlockId) (*ibtree.NodeData, bool) {
	if self.nodeDataCache == nil {
		return nil, false
	}
	v, err := self.nodeDataCache.GetIFPresent(id)
	if err != nil {
		return nil, false
	}
	return v.(*ibtree.NodeData), true
}

func (self *Hugger) SetCachedNodeData(id ibtree.BlockId, nd *ibtree.NodeData) {
	if self.nodeDataCache == nil {
		return
	}
	self.nodeDataCache.Set(id, nd)
}

func (self *Hugger) LoadNodeByName(name string) (*ibtree.Node, string, bool) {
	bid := self.Storage.GetBlockIdByName(name)
	if bid != "" {
		node := self.tree.LoadRoot(ibtree.BlockId(bid))
		if node == nil {
			log.Panicf("Loading of root block %x failed", bid)
		}
		return node, bid, true
	}
	return self.NewRootNode(), "", false
}

func (self *Hugger) RootBlock() *storage.StorageBlock {
	self.Flush()
	defer self.lock.Locked()()
	bl := self.root.Get().block
	if bl != nil {
		bl = bl.Open()
	}
	return bl
}

func (self *Hugger) RootIsNew() bool {
	node, bid, ok := self.LoadNodeByName(self.RootName)
	root := &treeRoot{node: node}
	if ok {
		root.block = self.Storage.GetBlockById(string(bid))
	}
	self.root.Set(root)
	return !ok
}

func (self *Hugger) NewRootNode() *ibtree.Node {
	return self.tree.NewRoot()
}

func (self *Hugger) closedTransaction(tr *Transaction) {
	defer self.lock.Locked()()
	delete(self.transactions, tr)
	if self.flushing {
		self.transactionClosed.Signal()
	}
}

func (self *Hugger) AssertNoTransactions() {
	defer self.lock.Locked()()
	if len(self.transactions) > 0 {
		log.Panicf("%d transactions left when assertion says none", len(self.transactions))
	}
}

// ibtree.TreeSaver API
func (self *Hugger) SaveNode(nd *ibtree.NodeData) ibtree.BlockId {
	b := nd.ToBytes()
	mlog.Printf2("ibtree/hugger/hugger", "SaveNode %d bytes", len(b))
	sl := &util.StringList{}
	if self.IterateReferencesCallback != nil {
		self.IterateReferencesCallback(nd,
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

// GetStorageBlock block ids for given bytes/data.
//
// The blocks are expired during flush.
//
// nd and deps are optional, but may speed up processing (or not).
func (self *Hugger) GetStorageBlock(st storage.BlockStatus, b []byte, nd *ibtree.NodeData, deps *util.StringList) *storage.StorageBlock {
	bl := self.Storage.ReferOrStoreBlockBytes0(st, b, deps)
	bid := string(bl.Id())
	mlog.Printf2("ibtree/hugger/hugger", "%v.GetStorageBlock => %x", self, bid)
	if nd != nil {
		self.SetCachedNodeData(ibtree.BlockId(bid), nd)
	}
	defer self.blockLock.Locked()()
	oldb, ok := self.blocks[bid]
	if ok {
		oldb.Close()
		mlog.Printf2("ibtree/hugger/hugger", " old one already existed")
	}
	self.blocks[bid] = bl
	return bl
}
