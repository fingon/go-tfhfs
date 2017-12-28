/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2017 Markus Stenberg
 *
 * Created:       Thu Dec 28 17:05:05 2017 mstenber
 * Last modified: Thu Dec 28 17:36:25 2017 mstenber
 * Edit time:     5 min
 *
 */

package ibtree

// IBTransaction is convenience API for dealing with the chaining of
// things in the ibtree. While API wise it is same as dealing with
// IBNodes, there is no need to track the most recent access, and it
// is slightly more efficient as it maintains its own cache of most
// recent accesses.
type IBTransaction struct {
	original *IBNode
	stack    IBStack
}

func NewTransaction(root *IBNode) *IBTransaction {
	t := &IBTransaction{original: root}
	t.stack.nodes[0] = root
	return t
}
