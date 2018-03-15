/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2018 Markus Stenberg
 *
 * Created:       Fri Jan  5 16:40:08 2018 mstenber
 * Last modified: Thu Mar 15 12:31:13 2018 mstenber
 * Edit time:     264 min
 *
 */

package hugger

import (
	"fmt"
	"log"
	"runtime"

	"github.com/fingon/go-tfhfs/ibtree"
	"github.com/fingon/go-tfhfs/mlog"
	"github.com/fingon/go-tfhfs/storage"
)

type treeRoot struct {
	node  *ibtree.Node
	block *storage.StorageBlock
}

type Transaction struct {
	hugger *Hugger

	// root is the root we based this transaction on
	root *treeRoot

	t              *ibtree.Transaction
	nested, closed bool

	// only figured if debugging is enabled; where was this
	// transaction created?
	createdBy string
}

func (self *Transaction) String() string {
	return fmt.Sprintf("t<%s>", self.createdBy)
}

func newTransaction(h *Hugger, ignoreFlushing bool) *Transaction {
	defer h.lock.Locked()()
	var createdBy string
	if mlog.IsEnabled() {
		_, file, line, ok := runtime.Caller(2)
		// ^newTransaction is called in hugger; however, who
		// calls THAT is interesting.
		if ok {
			createdBy = fmt.Sprintf("%s:%d", file, line)
		}
	}
	for !ignoreFlushing && h.flushing {
		h.flushed.Wait()
	}
	root := h.root.Get()
	t := ibtree.NewTransaction(root.node)
	tr := &Transaction{hugger: h, root: root,
		t: t, nested: ignoreFlushing, createdBy: createdBy}
	h.transactions[tr] = true
	return tr
}

func (self *Transaction) IB() *ibtree.Transaction {
	return self.t
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
	node := self.t.Root()
	if node == self.root.node {
		mlog.Printf2("ibtree/hugger/htransaction", " no changes for ht.Commit")
		return true
	}

	root := &treeRoot{node, nil}
	if !self.hugger.root.SetIfEqualTo(root, self.root) {

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

	return true
}

func (self *Transaction) Close() {
	if self.closed {
		return
	}
	mlog.Printf2("ibtree/hugger/htransaction", "ht.Close")
	self.closed = true

	self.hugger.closedTransaction(self)
}
