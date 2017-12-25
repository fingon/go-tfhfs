/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2017 Markus Stenberg
 *
 * Created:       Mon Dec 25 01:08:16 2017 mstenber
 * Last modified: Mon Dec 25 03:51:27 2017 mstenber
 * Edit time:     116 min
 *
 */

// ibtree package provides a functional b-tree that consists of nodes,
// with N children each, that are either leaves or other nodes.
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

	// Internal stuff
	// backend is mandatory and therefore Init argument.
	backend IBTreeBackend
}

const minimumNodeMaximumSize = 1024
const maximumTreeDepth = 10

func (self IBTree) Init(backend IBTreeBackend) *IBTree {
	if self.NodeMaximumSize < minimumNodeMaximumSize {
		self.NodeMaximumSize = minimumNodeMaximumSize
	}
	self.backend = backend
	return &self
}

type ibStack struct {
	nodes   [maximumTreeDepth]*IBNode
	indexes [maximumTreeDepth]int
	top     int
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
		c = append(c, child)
	}
	copy(c[idx+1:], n.Children[ni:])
	self.rewriteNodeChildren(c)
}

func (self *ibStack) rewriteNodeChildren(children []*IBNodeDataChild) {
	n := self.node().copy()
	n.Children = children
	self.nodes[self.top] = n
}

func (self *ibStack) node() *IBNode {
	return self.nodes[self.top]
}

func (self *ibStack) index() int {
	return self.indexes[self.top]

}

func (self *ibStack) pop() {
	n := self.node()
	self.top = self.top - 1
	c := &IBNodeDataChild{Key: n.Children[0].Key,
		childNode: n}
	self.rewriteAtIndex(false, c)
}

// Pop rest of the stack, creating new Nodes as need be, and return
// the top node.
func (self *ibStack) commit() *IBNode {
	for self.top > 0 {
		self.pop()
	}
	return self.node()
}

// NewNode creates a new node; by default, it is essentially new tree
// of its own. IBTree is really just factory for root nodes.
func (self *IBTree) NewNode() *IBNode {
	return &IBNode{tree: self, IBNodeData: IBNodeData{Leafy: true}}
}

func (self *IBNode) copy() *IBNode {
	return &IBNode{tree: self.tree, blockId: self.blockId,
		IBNodeData: self.IBNodeData}
}

func (self *IBNode) AddChild(child *IBNodeDataChild) (n *IBNode) {
	if len(self.Children) == 0 {
		n := self.copy()
		n.Children = []*IBNodeDataChild{child}
		return n
	}
	var st ibStack
	err := self.searchPrevOrEq(child.Key, &st)
	if err != nil {
		log.Panic(err)
	}

	st.addChildAt(child)
	return st.commit()
}

func (self *ibStack) addChildAt(child *IBNodeDataChild) {
	// Insert child where it belongs
	self.rewriteAtIndex(true, child)

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
	if len(self.Children) == 0 {
		return ErrEmptyTree
	}
	n := self
	stack.top = 0
	for {
		stack.nodes[stack.top] = n
		idx := n.childIndexWithKeyGT(key)
		if idx > 0 {
			idx = idx - 1
		}
		stack.indexes[stack.top] = idx
		if n.Leafy {
			break
		}
		n = n.childNode(idx)
		stack.top++
	}
	return nil
}

func (self *IBNode) childSearch(fun func(int) bool) int {
	index := sort.Search(len(self.Children), fun)
	return index
}

func (self *IBNode) childIndexWithKeyGT(key IBKey) int {
	return self.childSearch(func(i int) bool {
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
	n := &IBNode{tree: self.tree, IBNodeData: *nd}
	return n
}
