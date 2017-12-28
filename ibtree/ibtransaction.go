/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2017 Markus Stenberg
 *
 * Created:       Thu Dec 28 17:05:05 2017 mstenber
 * Last modified: Thu Dec 28 20:56:39 2017 mstenber
 * Edit time:     9 min
 *
 */

package ibtree

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
	return t
}

func (self *IBTransaction) Delete(key IBKey) {
	self.stack.node().Delete(key, &self.stack)
}
func (self *IBTransaction) Commit() *IBNode {
	return self.stack.node().Commit()
}

func (self *IBTransaction) DeleteRange(key1, key2 IBKey) {
	self.stack.node().DeleteRange(key1, key2, &self.stack)
}

func (self *IBTransaction) Get(key IBKey) *string {
	return self.stack.node().Get(key, &self.stack)
}

func (self *IBTransaction) Set(key IBKey, value string) {
	self.stack.node().Set(key, value, &self.stack)
}
