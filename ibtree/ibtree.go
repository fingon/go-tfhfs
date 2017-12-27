/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2017 Markus Stenberg
 *
 * Created:       Mon Dec 25 01:08:16 2017 mstenber
 * Last modified: Wed Dec 27 17:19:18 2017 mstenber
 * Edit time:     534 min
 *
 */

// ibtree package provides a functional b+ tree that consists of
// nodes, with N children each, that are either leaves or other nodes.
//
// It has built-in persistence, and Merkle tree-style hash tree
// behavior; root is defined by simply root node's hash, and similarly
// also all the children.
//
// ONLY root and dirty IBNodes are kept in memory; the rest count on (caching)
// storage backend + persistence layer being 'fast enough'.
package ibtree

import (
	"errors"
	"fmt"
	"log"
	"strings"
)

const hashSize = 32

var ErrEmptyTree = errors.New("empty tree")

type BlockId string

type IBTreeBackend interface {
	// LoadNode loads node based on backend id.
	LoadNode(id BlockId) *IBNodeData

	// SaveNode persists the node, and returns the backend id for it.
	SaveNode(nd IBNodeData) BlockId
}

// IBTree represents static configuration that can be used over
// multiple B+ trees. If this is changed with existing trees, bad
// things WILL happen.
type IBTree struct {
	// Can be provided externally
	NodeMaximumSize int
	halfSize        int
	smallSize       int

	// Internal stuff
	// backend is mandatory and therefore Init argument.
	backend IBTreeBackend

	// Used to represent references to nodes
	placeholderValue string
}

const minimumNodeMaximumSize = 1024
const maximumTreeDepth = 10

func (self IBTree) Init(backend IBTreeBackend) *IBTree {
	maximumSize := self.NodeMaximumSize
	if maximumSize < minimumNodeMaximumSize {
		maximumSize = minimumNodeMaximumSize
	}
	self.setNodeMaximumSize(self.NodeMaximumSize)
	self.backend = backend
	self.placeholderValue = fmt.Sprintf("hash-%s",
		strings.Repeat("x", hashSize-5))
	return &self
}

// NewRoot creates a new node; by default, it is essentially new tree
// of its own. IBTree is really just factory for root nodes.
func (self *IBTree) NewRoot() *IBNode {
	return &IBNode{tree: self, IBNodeData: IBNodeData{Leafy: true}}
}

func (self *IBTree) setNodeMaximumSize(maximumSize int) {
	self.NodeMaximumSize = maximumSize
	self.halfSize = self.NodeMaximumSize / 2
	self.smallSize = self.halfSize / 2
}

// IBNode represents single node in single tree.
type IBNode struct {
	IBNodeData
	blockId BlockId // on disk, if any
	tree    *IBTree
}

func (self *IBNode) copy() *IBNode {
	return &IBNode{tree: self.tree, blockId: self.blockId,
		IBNodeData: self.IBNodeData}
}

func (self *IBNode) Delete(key IBKey) *IBNode {
	var st ibStack
	err := self.search(key, &st)
	if err != nil {
		log.Panic(err)
	}
	c := st.child()
	if c.Key != key {
		log.Panic("ibp.Delete: Key missing ", key)
	}
	st.rewriteAtIndex(true, nil)
	node := st.node()
	if node.Msgsize() <= self.tree.smallSize {
		st.smallCount++
	}
	return st.commit()
}

func (self *IBNode) DeleteRange(key1, key2 IBKey) *IBNode {
	var st ibStack
	err := self.search(key1, &st)
	if err != nil {
		log.Panic(err)
	}
	var st2 ibStack = st
	st2.top = 0
	err = st2.search(key2)
	if err != nil {
		log.Panic(err)
	}
	if st2.child().Key == key2 {
		st2.indexes[st2.top]++
	}
	// No matches at all?
	if st == st2 {
		return self
	}
	common := 0
	for i := 1; i < st.top && st.indexes[i-1] == st2.indexes[i-1]; i++ {
		common = i
	}
	// nodes [ .. common] and indexes[.. common-1] are same

	// Make the st2 match st, one level at a time.
	// top = the level we are currently editing
	for st2.top >= common {

	}
	st2.top++
	return st2.commit()
}

func (self *IBNode) Get(key IBKey) *string {
	var st ibStack
	err := self.search(key, &st)
	if err != nil {
		log.Panic(err)
	}
	c := st.child()
	if c == nil || c.Key != key {
		//log.Printf("non-matching child:%v for %v", c, key)
		return nil
	}
	return &c.Value

}

func (self *IBNode) Set(key IBKey, value string) *IBNode {
	var st ibStack
	err := self.search(key, &st)
	if err != nil {
		log.Panic(err)
	}
	child := &IBNodeDataChild{Key: key, Value: value}
	c := st.child()
	if c == nil || c.Key != key {
		// now at next -> insertion point is where it pointing at
		st.addChildAt(child)
		return st.commit()
	}
	if st.child().Value == value {
		return self
	}
	st.rewriteAtIndex(true, child)
	return st.commit()

}

func (self *IBNode) search(key IBKey, stack *ibStack) error {
	stack.nodes[0] = self
	return stack.search(key)
}

func (self *IBNode) childNode(idx int) *IBNode {
	// Get the corresponding child node.
	c := self.Children[idx]
	if c.childNode != nil {
		return c.childNode
	}
	// Uh oh. Not dirty. Have to load from somewhere.
	// TBD: We could cache this, maybe, but probably not worth it.
	if self.tree.backend == nil {
		return nil
	}
	nd := self.tree.backend.LoadNode(BlockId(c.Value))
	if nd == nil {
		return nil
	}
	return &IBNode{tree: self.tree, IBNodeData: *nd}
}

func (self *IBNode) print(indent int) {
	// Sanity check - could someday get rid of this
	prefix := strings.Repeat("  ", indent)
	for i, v := range self.Children {
		fmt.Printf("%s[%d]: %s\n", prefix, i, v.Key)
		if !self.Leafy && v.childNode != nil {
			v.childNode.print(indent + 2)
		}
	}

}

func (self *IBNode) iterateLeafFirst(fun func(n *IBNode)) {
	for _, c := range self.Children {
		if c.childNode != nil {
			c.childNode.iterateLeafFirst(fun)
		}
	}
	fun(self)
}

func (self *IBNode) checkTreeStructure() {
	self.iterateLeafFirst(func(n *IBNode) {
		for i, c := range n.Children {
			if i > 0 {
				if n.Children[i-1].Key >= c.Key {
					self.print(0)
					log.Panic("tree broke!")
				}
			}
		}

	})
}
