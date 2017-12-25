/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2017 Markus Stenberg
 *
 * Created:       Mon Dec 25 01:08:16 2017 mstenber
 * Last modified: Mon Dec 25 17:26:49 2017 mstenber
 * Edit time:     217 min
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
	"sort"
)

var ErrEmptyTree = errors.New("empty tree")

type BlockId string

type IBNode struct {
	IBNodeData
	blockId BlockId // on disk, if any
	tree    *IBTree
}

type IBTreeBackend interface {
	// LoadNode loads node based on backend id.
	LoadNode(id BlockId) *IBNodeData

	// SaveNode persists the node, and returns the backend id for it.
	SaveNode(nd IBNodeData) BlockId
}

type IBTree struct {
	// Can be provided externally
	NodeMaximumSize int
	halfSize        int
	smallSize       int

	// Internal stuff
	// backend is mandatory and therefore Init argument.
	backend IBTreeBackend
}

const minimumNodeMaximumSize = 512
const maximumTreeDepth = 10

func (self IBTree) Init(backend IBTreeBackend) *IBTree {
	if self.NodeMaximumSize < minimumNodeMaximumSize {
		self.NodeMaximumSize = minimumNodeMaximumSize
	}
	self.halfSize = self.NodeMaximumSize / 2
	self.smallSize = self.halfSize / 2
	self.backend = backend
	return &self
}

type ibStack struct {
	// Static arrays that are used to store the 'trace' of our
	// walk in the tre. By backtracking it at 'commit', we can
	// handle COW of the recursive data structure.

	nodes   [maximumTreeDepth]*IBNode
	indexes [maximumTreeDepth]int

	// The highest index of the nodes/indexes arrays with the values set.
	top int

	// How many nodes have turned small during lifetime of the stack.
	smallCount int
}

func (self *ibStack) rewriteAtIndex(replace bool, child *IBNodeDataChild) {
	n := self.node()
	idx := self.index()
	nl := len(n.Children) + 1
	ni := idx
	if replace {
		nl--
		ni++
	}
	if child == nil {
		nl--
	}
	c := make([]*IBNodeDataChild, nl)
	copy(c, n.Children[:idx])
	if child != nil {
		c[idx] = child
	}
	fmt.Printf("rewriteAtIndex idx:%d ni:%d => %d items\n", idx, ni, nl)
	copy(c[idx+1:], n.Children[ni:])
	self.rewriteNodeChildren(c)
}

func (self *ibStack) rewriteNodeChildren(children []*IBNodeDataChild) {
	n := self.node().copy()
	n.Children = children
	self.nodes[self.top] = n
	// This invalidates sub-trees (if any)
	self.nodes[self.top+1] = nil
	self.indexes[self.top] = -1
}

func (self *ibStack) child() *IBNodeDataChild {
	cl := self.node().Children
	index := self.index()
	if index < 0 || index >= len(cl) {
		return nil
	}
	return cl[index]
}

func (self *ibStack) index() int {
	return self.indexes[self.top]
}

func (self *ibStack) node() *IBNode {
	return self.nodes[self.top]
}

func (self *ibStack) nodeSib(ofs int) *IBNode {
	if self.top == 0 {
		return nil
	}
	idx := self.indexes[self.top-1]
	nidx := idx + ofs
	if nidx < 0 || nidx >= len(self.nodes[self.top-1].Children) {
		return nil
	}
	return self.nodes[self.top-1].childNode(nidx)
}

func (self *ibStack) pop() {
	n := self.node()
	self.top--
	c := &IBNodeDataChild{Key: n.Children[0].Key,
		childNode: n}
	self.rewriteAtIndex(true, c)
}

// Pop rest of the stack, creating new Nodes as need be, and return
// the top node.
func (self *ibStack) commit() *IBNode {
	for self.top > 0 {
		self.pop()
	}
	if self.smallCount > 0 {
		// TBD: make the nodes smaller

		// TBD: cut root down a level if its children are together small enough
	}
	return self.node()
}

// NewRoot creates a new node; by default, it is essentially new tree
// of its own. IBTree is really just factory for root nodes.
func (self *IBTree) NewRoot() *IBNode {
	return &IBNode{tree: self, IBNodeData: IBNodeData{Leafy: true}}
}

func (self *IBNode) copy() *IBNode {
	return &IBNode{tree: self.tree, blockId: self.blockId,
		IBNodeData: self.IBNodeData}
}

func (self *IBNode) Delete(key IBKey) *IBNode {
	var st ibStack
	err := self.searchPrevOrEq(key, &st)
	if err != nil {
		log.Panic(err)
	}
	c := st.child()
	if c.Key != key {
		log.Panic("ibp.Delete: Key missing", key)
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
	err := self.searchPrevOrEq(key1, &st)
	if err != nil {
		log.Panic(err)
	}
	var st2 ibStack = st
	st2.top = 0
	err = st2.searchPrevOrEq(key2)
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
	err := self.searchPrevOrEq(key, &st)
	if err != nil {
		log.Panic(err)
	}
	c := st.child()
	if c == nil || c.Key != key {
		return nil
	}
	return &c.Value

}

func (self *IBNode) Set(key IBKey, value string) *IBNode {
	var st ibStack
	err := self.searchPrevOrEq(key, &st)
	if err != nil {
		log.Panic(err)
	}
	child := &IBNodeDataChild{Key: key, Value: value}
	c := st.child()
	if c == nil || c.Key != key {
		st.addChildAt(child)
		return st.commit()
	}
	if st.child().Value == value {
		return self
	}
	st.rewriteAtIndex(false, child)
	return st.commit()

}

func (self *ibStack) addChildAt(child *IBNodeDataChild) {
	// Insert child where it belongs
	self.rewriteAtIndex(false, child)

	node := self.node()

	if node.Msgsize() <= node.tree.NodeMaximumSize {
		return
	}

	s := 0
	i := 0
	for s < node.Msgsize()/2 {
		s += node.Children[i].Msgsize()
		i++
	}
	nodec := node.Children[:i]
	nextc := node.Children[i:]

	// Remove children from this
	self.rewriteNodeChildren(nodec)

	// And create next node that will have them
	next := &IBNode{tree: node.tree,
		IBNodeData: IBNodeData{Leafy: node.Leafy,
			Children: nextc}}
	nextchild := &IBNodeDataChild{Key: nextc[0].Key,
		childNode: next}
	if self.top > 0 {
		self.pop()
		self.indexes[self.top]++
		self.addChildAt(nextchild)
		return
	}

	// Uh oh. Didn't fit to root level. Have to create new root
	// with two children instead.
	node = self.node()
	mechild := &IBNodeDataChild{Key: nodec[0].Key, childNode: node}
	self.nodes[0] = &IBNode{tree: node.tree,
		IBNodeData: IBNodeData{
			Children: []*IBNodeDataChild{mechild, nextchild}}}
}

func (self *IBNode) searchPrevOrEq(key IBKey, stack *ibStack) error {
	stack.nodes[0] = self
	return stack.searchPrevOrEq(key)
}

func (self *ibStack) searchPrevOrEq(key IBKey) error {
	n := self.nodes[0]
	self.top = 0
	for {
		idx := n.childIndexWithKeyGT(key)
		if idx > 0 {
			idx = idx - 1
		}
		on := self.nodes[self.top+1]
		if idx != self.indexes[self.top] {
			on = nil
		}
		self.indexes[self.top] = idx
		if n.Leafy {
			break
		}
		if on != nil {
			n = on
		} else {
			n = n.childNode(idx)
			self.nodes[self.top] = n
		}
		self.top++
	}
	return nil
}

func (self *IBNode) childSearch(fun func(int) bool) int {
	index := sort.Search(len(self.Children), fun)
	return index
}

func (self *IBNode) childIndexWithKeyGT(key IBKey) int {
	return self.childSearch(func(i int) bool {
		fmt.Printf("childSearch i:%d of %d\n", i, len(self.Children))
		return self.Children[i].Key > key
	})
}

func (self *IBNode) childNode(idx int) *IBNode {
	// Get the corresponding child node.
	c := self.Children[idx]
	if c.childNode != nil {
		return c.childNode
	}
	// Uh oh. Not dirty. Have to load from somewhere.
	// TBD: We could cache this, maybe, but probably not worth it.
	nd := self.tree.backend.LoadNode(BlockId(c.Value))
	if nd == nil {
		return nil
	}
	return &IBNode{tree: self.tree, IBNodeData: *nd}
}
