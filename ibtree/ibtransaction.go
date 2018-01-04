/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2017 Markus Stenberg
 *
 * Created:       Thu Dec 28 17:05:05 2017 mstenber
 * Last modified: Fri Jan  5 00:10:25 2018 mstenber
 * Edit time:     14 min
 *
 */

package ibtree

import (
	"log"

	"github.com/fingon/go-tfhfs/mlog"
)

// IBTransaction is convenience API for dealing with the chaining of
// things in the ibtree. While API wise it is same as dealing with
// IBNodes (and manual IBStack allocation), there is no need to track
// the most recent access, and it is slightly more efficient as it
// maintains its own cache of most recent accesses within the built-in
// IBStack.
type IBTransaction struct {
	original *IBNode
	stack    IBStack
}

func NewTransaction(root *IBNode) *IBTransaction {
	t := &IBTransaction{original: root}
	t.stack.nodes[0] = root
	if root == nil {
		log.Panicf("nil root")
	}
	return t
}

func (self *IBTransaction) Delete(key IBKey) {
	mlog.Printf2("ibtree/ibtransaction", "tr.Delete %x", key)
	self.stack.node().Delete(key, &self.stack)
}
func (self *IBTransaction) Commit() (*IBNode, BlockId) {
	return self.stack.node().Commit()
}

func (self *IBTransaction) DeleteRange(key1, key2 IBKey) {
	mlog.Printf2("ibtree/ibtransaction", "tr.DeleteRange %x-%x", key1, key2)
	self.stack.node().DeleteRange(key1, key2, &self.stack)
}

func (self *IBTransaction) Get(key IBKey) *string {
	return self.stack.node().Get(key, &self.stack)
}

func (self *IBTransaction) NextKey(key IBKey) *IBKey {
	return self.stack.node().NextKey(key, &self.stack)
}

func (self *IBTransaction) Set(key IBKey, value string) {
	mlog.Printf2("ibtree/ibtransaction", "tr.Set %x %d bytes", key, len(value))
	self.stack.node().Set(key, value, &self.stack)
}
