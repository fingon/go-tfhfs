/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2017 Markus Stenberg
 *
 * Created:       Thu Dec 28 17:05:05 2017 mstenber
 * Last modified: Thu Feb 15 14:33:22 2018 mstenber
 * Edit time:     31 min
 *
 */

package ibtree

import (
	"fmt"
	"log"

	"github.com/fingon/go-tfhfs/mlog"
)

// Transaction is convenience API for dealing with the chaining of
// things in the ibtree. While API wise it is same as dealing with
// Nodes (and manual Stack allocation), there is no need to track
// the most recent access, and it is slightly more efficient as it
// maintains its own cache of most recent accesses within the built-in
// Stack.
type Transaction struct {
	stack Stack
}

func NewTransaction(root *Node) *Transaction {
	self := &Transaction{}
	self.stack.nodes[0] = root
	if root == nil {
		log.Panicf("nil root")
	}
	return self
}

func (self *Transaction) Delete(key Key) {
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

func (self *Transaction) DeleteRange(key1, key2 Key) {
	mlog.Printf2("ibtree/ibtransaction", "tr.DeleteRange %x-%x", key1, key2)
	self.Root().DeleteRange(key1, key2, &self.stack)
}

func (self *Transaction) Get(key Key) *string {
	mlog.Printf2("ibtree/ibtransaction", "tr.Get")
	return self.Root().Get(key, &self.stack)
}

func (self *Transaction) NextKey(key Key) *Key {
	mlog.Printf2("ibtree/ibtransaction", "tr.NextKey")
	return self.Root().NextKey(key, &self.stack)
}

func (self *Transaction) Root() *Node {
	return self.stack.node()
}

func (self *Transaction) Set(key Key, value string) {
	mlog.Printf2("ibtree/ibtransaction", "tr.Set %x %d bytes", key, len(value))
	self.Root().Set(key, value, &self.stack)
}

func (self *Transaction) NewSubTree(treePrefix Key) *SubTree {
	return &SubTree{transaction: self, treePrefix: treePrefix}
}

// SubTree is convenience API for handling subtree of entries with
// common key prefix. Using the object, the tree behaves just like
// normal transaction (modulo few API calls that root transaction has
// to be used for).
type SubTree struct {
	transaction *Transaction
	treePrefix  Key
}

func (self *SubTree) addTreePrefix(key Key) Key {
	return Key(fmt.Sprintf("%s%s", self.treePrefix, key))
}

func (self *SubTree) Delete(key Key) {
	mlog.Printf2("ibtree/ibtransaction", "st.Delete %x", key)
	key = self.addTreePrefix(key)
	self.transaction.Delete(key)
}

func (self *SubTree) DeleteRange(key1, key2 Key) {
	mlog.Printf2("ibtree/ibtransaction", "st.DeleteRange %x-%x", key1, key2)
	key1 = self.addTreePrefix(key1)
	key2 = self.addTreePrefix(key2)
	self.transaction.DeleteRange(key1, key2)
}

func (self *SubTree) Get(key Key) *string {
	mlog.Printf2("ibtree/ibtransaction", "st.Get")
	key = self.addTreePrefix(key)
	return self.transaction.Get(key)
}

func (self *SubTree) NextKey(key Key) *Key {
	mlog.Printf2("ibtree/ibtransaction", "st.NextKey")
	key = self.addTreePrefix(key)
	nk := self.transaction.NextKey(key)
	if nk != nil && string(*nk)[:len(self.treePrefix)] != string(self.treePrefix) {
		nk = nil
	}
	return nk
}

func (self *SubTree) Set(key Key, value string) {
	mlog.Printf2("ibtree/ibtransaction", "tr.Set %x %d bytes", key, len(value))
	key = self.addTreePrefix(key)
	self.transaction.Set(key, value)
}
