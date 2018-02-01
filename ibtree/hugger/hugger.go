/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2018 Markus Stenberg
 *
 * Created:       Wed Jan 17 12:37:08 2018 mstenber
 * Last modified: Thu Feb  1 17:46:44 2018 mstenber
 * Edit time:     54 min
 *
 */

package hugger

import (
	"log"
	"sync"

	"github.com/bluele/gcache"
	"github.com/fingon/go-tfhfs/ibtree"
	"github.com/fingon/go-tfhfs/mlog"
	"github.com/fingon/go-tfhfs/storage"
	"github.com/fingon/go-tfhfs/util"
)

type BlockDataType byte

const (
	BDT_NODE BlockDataType = 7
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
	root          treeRootAtomicPointer
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

	// roots is the map of potentially active root blocks (we
	// clear them only at flush time, if there are zero active
	// transactions)
	oldRoots map[*storage.StorageBlock]bool
}

func (self *Hugger) Init(cacheSize int) *Hugger {
	self.tree = ibtree.Tree{NodeMaximumSize: 4096}.Init(self)
	self.transactions = make(map[*Transaction]bool)
	self.oldRoots = make(map[*storage.StorageBlock]bool)
	self.flushed.L = &self.lock
	self.transactionClosed.L = &self.lock
	if cacheSize > 0 {
		self.nodeDataCache = gcache.New(cacheSize).
			ARC().
			Build()
	}
	return self
}

func (self *Hugger) GetTransaction() *Transaction {
	// mlog.Printf2("ibtree/hugger/hugger", "GetTransaction of %p", self.treeRoot)
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
		nd := BytesToNodeData(b.Data())
		if nd == nil {
			log.Panicf("Unable to find node %x", id)
		}
		self.SetCachedNodeData(id, nd)
		return nd
	}
	mlog.Printf2("ibtree/hugger/hugger", "fs.LoadNode found %x in cache: %p", id, v)
	return v.(*ibtree.NodeData)
}

// ibtree.TreeBackend API
func (self *Hugger) SaveNode(nd *ibtree.NodeData) ibtree.BlockId {
	log.Panicf("should be always used via Transaction.SaveNode")
	return ibtree.BlockId("")
}

func (self *Hugger) Flush() {
	defer self.lock.Locked()()
	self.flushing = true
	for len(self.transactions) > 0 {
		self.transactionClosed.Wait()
	}
	nroots := len(self.oldRoots)
	if nroots > 0 {
		for b, _ := range self.oldRoots {
			b.Close()
		}
		self.oldRoots = make(map[*storage.StorageBlock]bool)
		mlog.Printf2("ibtree/hugger/hugger", " cleared %d roots", nroots)
	}
	self.flushing = false
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
	return self.root.Get().block
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

	// -1 ref when transaction expires (old root)
	if tr.rootBlock != nil {
		self.oldRoots[tr.rootBlock] = true
	}
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

func BytesToNodeData(bd []byte) *ibtree.NodeData {
	dt := BlockDataType(bd[0])
	if dt != BDT_NODE {
		log.Panicf("BytesToNodeData - wrong dt:%v", dt)
	}
	nd := &ibtree.NodeData{}
	_, err := nd.UnmarshalMsg(bd[1:])
	if err != nil {
		log.Panic(err)
	}
	return nd
}

func NodeDataToBytes(nd *ibtree.NodeData) []byte {
	bb := make([]byte, nd.Msgsize()+1)
	bb[0] = byte(BDT_NODE)
	b, err := nd.MarshalMsg(bb[1:1])
	if err != nil {
		log.Panic(err)
	}
	return bb[0 : 1+len(b)]
}
