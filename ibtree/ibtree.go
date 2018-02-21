/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2017 Markus Stenberg
 *
 * Created:       Mon Dec 25 01:08:16 2017 mstenber
 * Last modified: Wed Feb 21 15:29:45 2018 mstenber
 * Edit time:     771 min
 *
 */

// ibtree package provides a functional b+ tree that consists of
// nodes, with N children each, that are either leaves or other nodes.
//
// It has built-in persistence, and Merkle tree-style hash tree
// behavior; root is defined by simply root node's hash, and similarly
// also all the children.
//
// ONLY root and dirty Nodes are kept in memory; the rest count on (caching)
// storage backend + persistence layer being 'fast enough'.
package ibtree

import (
	"fmt"
	"strings"

	"github.com/fingon/go-tfhfs/mlog"
)

const hashSize = 32

type BlockId string

type TreeSaver interface {
	// SaveNode persists the node, and returns the backend id for it.
	SaveNode(nd *NodeData) BlockId
}

type TreeLoader interface {
	// LoadNode loads node based on backend id.
	LoadNode(id BlockId) *NodeData
}

type TreeBackend interface {
	TreeLoader
	TreeSaver
}

// Tree represents static configuration that can be used over
// multiple B+ trees. If this is changed with existing trees, bad
// things WILL happen.
type Tree struct {
	// Can be provided externally
	NodeMaximumSize int
	halfSize        int
	smallSize       int

	// Internal stuff
	// backend is mandatory and therefore Init argument.
	backend TreeBackend

	// Used to represent references to nodes
	placeholderValue string
}

const minimumNodeMaximumSize = 512
const maximumTreeDepth = 10

func (self Tree) Init(backend TreeBackend) *Tree {
	maximumSize := self.NodeMaximumSize
	if maximumSize < minimumNodeMaximumSize {
		maximumSize = minimumNodeMaximumSize
	}
	self.setNodeMaximumSize(maximumSize)
	self.backend = backend
	self.placeholderValue = fmt.Sprintf("hash-%s",
		strings.Repeat("x", hashSize-5))
	return &self
}

func (self *Tree) LoadRoot(bid BlockId) *Node {
	mlog.Printf2("ibtree/ibtree", "t.LoadRoot %x", bid)
	data := self.backend.LoadNode(bid)
	if data == nil {
		return nil
	}
	return &Node{tree: self, NodeData: *data, blockId: &bid}
}

// NewRoot creates a new node; by default, it is essentially new tree
// of its own. Tree is really just factory for root nodes.
func (self *Tree) NewRoot() *Node {
	mlog.Printf2("ibtree/ibtree", "t.NewRoot")
	return &Node{tree: self, NodeData: NodeData{Leafy: true}}
}

func (self *Tree) setNodeMaximumSize(maximumSize int) {
	self.NodeMaximumSize = maximumSize
	self.halfSize = self.NodeMaximumSize / 2
	self.smallSize = self.halfSize / 2
}

// Node represents single node in single tree.
type Node struct {
	NodeData
	blockId *BlockId // on disk, if any
	tree    *Tree
}

func (self *Node) String() string {
	return fmt.Sprintf("ibn{%p}", self)
}

func (self *Node) Delete(key Key, st *Stack) *Node {
	self.search(key, st)
	c := st.child()
	if c.Key != key {
		mlog.Panicf("ibp.Delete: Key missing: %x", key)
	}
	st.rewriteAtIndex(true, nil)
	return st.commit()
}

// CommitTo pushes dirty Node to the specified backend, returning the new root.
func (self *Node) CommitTo(backend TreeSaver) (*Node, BlockId) {
	// Iterate through the tree, updating the nodes as we go.

	if self.blockId != nil {
		mlog.Printf2("ibtree/ibtree", "in.Commit, unchanged")
		return self, *self.blockId
	}

	cl := self.Children
	if !self.Leafy {
		// Scary optimization: Do this in parallel if it does
		// seem worth it
		cc := 0
		var onlyi int
		for i, c := range self.Children {
			if c.childNode != nil {
				cc++
				onlyi = i
			}
		}

		if cc > 0 {

			// Need to copy children if not leafy; leafy children
			// are data-only, and therefore can be copied as is
			cl = make([]*NodeDataChild, len(self.Children))
			copy(cl, self.Children)

			handleOne := func(i int) {
				c := cl[i]
				_, bid := c.childNode.CommitTo(backend)
				cl[i] = &NodeDataChild{Key: c.Key, Value: string(bid)}
			}

			if cc == 1 {
				handleOne(onlyi)
			} else {
				//wg := &util.SimpleWaitGroup{}
				for i, c := range self.Children {
					if c.childNode != nil {
						i := i
						//wg.Go(func() {
						handleOne(i)
						//})
					}
				}
				//wg.Wait()
				// ^ while on paper this is nice idea,
				// in practise it is not significant
				// speedup and the goroutines will
				// hang around consuming plenty of
				// space for a long while
				// afterwards. so skip for now..
			}
		}
	}

	n := self.copy()
	n.Children = cl

	bid := backend.SaveNode(&n.NodeData)
	n.blockId = &bid
	mlog.Printf2("ibtree/ibtree", "in.Commit, new bid %x..", bid[:10])
	return n, bid
}

// Commit pushes dirty Node to default backend, returning the new root.
func (self *Node) Commit() (*Node, BlockId) {
	return self.CommitTo(self.tree.backend)
}

func (self *Node) DeleteRange(key1, key2 Key, st2 *Stack) *Node {
	if key1 > key2 {
		mlog.Panicf("ibt.DeleteRange: first key more than second key: %x > %x", key1, key2)
	}
	mlog.Printf2("ibtree/ibtree", "DeleteRange [%x..%x]", key1, key2)
	st2.top = 0
	st := *st2
	self.searchLesser(key1, &st)
	mlog.Printf2("ibtree/ibtree", "c1:%v @%v", st.child(), st.indexes)
	self.searchGreater(key2, st2)
	mlog.Printf2("ibtree/ibtree", "c2:%v @%v", st2.child(), st2.indexes)
	// No matches at all?
	if st == *st2 {
		return st2.commit()
	}
	unique := 0
	for i := 0; i < st.top && st.indexes[i] == st2.indexes[i]; i++ {
		unique = i + 1
	}
	if st.indexes == st2.indexes {
		return st2.commit()
	}
	mlog.Printf2("ibtree/ibtree", "unique:%d", unique)
	for st2.top > unique {
		idx := st2.index()
		if idx > 0 {
			mlog.Printf2("ibtree/ibtree", "removing @%d[<%d]", st2.top, idx)
			st2.rewriteNodeChildrenWithCopyOf(st2.node().Children[idx:])
		}
		st2.pop()
	}
	for st.top > unique {
		cl := st.node().Children
		idx := st.index()
		if idx < (len(cl) - 1) {
			mlog.Printf2("ibtree/ibtree", "removing @%d[>%d]", st.top, idx)
			st.rewriteNodeChildrenWithCopyOf(cl[:(idx + 1)])
		}
		st.pop()
	}

	// nodes[unique] should be same
	// indexes[unique] should differ
	cl := st2.node().Children
	idx1 := st.indexes[st.top]
	idx2 := st2.indexes[st.top]
	mlog.Printf2("ibtree/ibtree", "idx1:%d idx2:%d", idx1, idx2)
	var ncl []*NodeDataChild
	if idx1 < 0 {
		ncl = make([]*NodeDataChild, len(cl)-idx2)
		copy(ncl, cl[idx2:])
	} else {
		ncl = make([]*NodeDataChild, len(cl)-(idx2-idx1)+1)
		copy(ncl, cl[:idx1])
		ncl[idx1] = st.child()
		if len(cl) > idx2 {
			copy(ncl[(idx1+1):], cl[idx2:])
		}
	}
	st2.rewriteNodeChildren(ncl)
	return st2.commit()
}

func (self *Node) Get(key Key, st *Stack) *string {
	self.search(key, st)
	c := st.child()
	st.top = 0
	if c == nil || c.Key != key {
		return nil
	}
	return &c.Value

}

func (self *Node) NextKey(key Key, st *Stack) *Key {
	self.searchGreater(key, st)
	c := st.child()
	st.top = 0
	if c == nil {
		return nil
	}
	return &c.Key
}

func (self *Node) PrevKey(key Key, st *Stack) *Key {
	self.searchLesser(key, st)
	c := st.child()
	st.top = 0
	if c == nil {
		return nil
	}
	return &c.Key
}

func (self *Node) Set(key Key, value string, st *Stack) *Node {
	self.search(key, st)
	child := &NodeDataChild{Key: key, Value: value}
	c := st.child()
	if c == nil || c.Key != key {
		// now at next -> insertion point is where it pointing at
		st.addChildAt(child)
	} else {
		if st.child().Value != value {
			st.rewriteAtIndex(true, child)
		}
	}
	return st.commit()

}

func (self *Node) search(key Key, st *Stack) {
	if st.nodes[0] == nil {
		st.nodes[0] = self
	} else if self != st.nodes[0] {
		mlog.Panicf("historic/wrong self:%v != %v", self, st.nodes[0])
	}
	if st.top > 0 {
		mlog.Panicf("leftover stack")
	}
	st.search(key)
}

func (self *Node) copy() *Node {
	return &Node{tree: self.tree, blockId: self.blockId,
		NodeData: self.NodeData}
}

func (self *Node) childNode(idx int) *Node {
	// Get the corresponding child node.
	c := self.Children[idx]
	if c.childNode != nil {
		return c.childNode
	}
	// Uh oh. Not dirty. Have to load from somewhere.
	// TBD: We could cache this, maybe, but probably not worth it.
	if self.tree.backend == nil {
		mlog.Printf2("ibtree/ibtree", "childNode - backend not set")
		return nil
	}
	bid := BlockId(c.Value)
	nd := self.tree.backend.LoadNode(bid)
	if nd == nil {
		mlog.Panicf("childNode - backend LoadNode for %x failed", bid)
	}
	return &Node{tree: self.tree, blockId: &bid, NodeData: *nd}
}

func (self *Node) PrintToMLogDirty() {
	if !mlog.IsEnabled() {
		return
	}
	// Sanity check - could someday get rid of this
	for i, v := range self.Children {
		mlog.Printf2("ibtree/ibtree", "[%d]: %x", i, v.Key)
		if !self.Leafy {
			if v.childNode != nil {
				v.childNode.PrintToMLogDirty()
			} else {
				mlog.Printf2("ibtree/ibtree", "     bid:%x..", v.Value[:8])

			}
		}
	}

}

func (self *Node) PrintToMLogAll() {
	if !mlog.IsEnabled() {
		return
	}
	// Sanity check - could someday get rid of this
	for i, v := range self.Children {
		mlog.Printf2("ibtree/ibtree", "[%d]: %x", i, v.Key)
		if !self.Leafy {
			mlog.Printf2("ibtree/ibtree", "     bid:%x..", v.Value[:8])
			cn := self.childNode(i)
			if cn != nil {
				cn.PrintToMLogAll()
			}
		}
	}

}

func (self *Node) iterateLeafFirst(fun func(n *Node)) {
	for _, c := range self.Children {
		if c.childNode != nil {
			c.childNode.iterateLeafFirst(fun)
		}
	}
	fun(self)
}

func (self *Node) checkTreeStructure() {
	self.iterateLeafFirst(func(n *Node) {
		n.CheckNodeStructure()
	})
}

func (self *Node) nestedNodeCount() int {
	cnt := 0
	self.iterateLeafFirst(func(n *Node) {
		cnt++
	})
	return cnt
}

func (self *Node) searchLesser(key Key, st *Stack) {
	self.search(key, st)
	c := st.child()
	if c == nil || c.Key >= key {
		mlog.Printf2("ibtree/ibtree", "moving to previous leaf from %v", st.indexes)
		st.goPreviousLeaf()
	}
}

func (self *Node) searchGreater(key Key, st *Stack) {
	self.search(key, st)
	c := st.child()
	if c == nil || c.Key == key {
		mlog.Printf2("ibtree/ibtree", "moving to next leaf from %v", st.indexes)
		st.goNextLeaf()
	}
}
