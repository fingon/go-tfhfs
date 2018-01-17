/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2017 Markus Stenberg
 *
 * Created:       Thu Dec 28 17:05:05 2017 mstenber
 * Last modified: Fri Jan 12 10:13:17 2018 mstenber
 * Edit time:     17 min
 *
 */

package ibtree

import (
	"log"

	"github.com/fingon/go-tfhfs/mlog"
)

// Transaction is convenience API for dealing with the chaining of
// things in the ibtree. While API wise it is same as dealing with
// Nodes (and manual IBStack allocation), there is no need to track
// the most recent access, and it is slightly more efficient as it
// maintains its own cache of most recent accesses within the built-in
// IBStack.
type Transaction struct {
	original *Node
	stack    IBStack
}

func NewTransaction(root *Node) *Transaction {
	t := &Transaction{original: root}
	t.stack.nodes[0] = root
	if root == nil {
		log.Panicf("nil root")
	}
	return t
}

func (self *Transaction) Delete(key IBKey) {
	mlog.Printf2("ibtree/ibtransaction", "tr.Delete %x", key)
	self.Root().Delete(key, &self.stack)
}

func (self *Transaction) Commit() (*Node, BlockId) {
	mlog.Printf2("ibtree/ibtransaction", "tr.Commit")
	return self.Root().Commit()
}

func (self *Transaction) CommitTo(backend TreeSaver) (*Node, BlockId) {
	mlog.Printf2("ibtree/ibtransaction", "tr.CommitTo")
	return self.Root().CommitTo(backend)
}

func (self *Transaction) DeleteRange(key1, key2 IBKey) {
	mlog.Printf2("ibtree/ibtransaction", "tr.DeleteRange %x-%x", key1, key2)
	self.Root().DeleteRange(key1, key2, &self.stack)
}

func (self *Transaction) Get(key IBKey) *string {
	mlog.Printf2("ibtree/ibtransaction", "tr.Get")
	return self.Root().Get(key, &self.stack)
}

func (self *Transaction) NextKey(key IBKey) *IBKey {
	mlog.Printf2("ibtree/ibtransaction", "tr.NextKey")
	return self.Root().NextKey(key, &self.stack)
}

func (self *Transaction) Root() *Node {
	return self.stack.node()
}

func (self *Transaction) Set(key IBKey, value string) {
	mlog.Printf2("ibtree/ibtransaction", "tr.Set %x %d bytes", key, len(value))
	self.Root().Set(key, value, &self.stack)
}
