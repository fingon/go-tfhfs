/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2017 Markus Stenberg
 *
 * Created:       Mon Dec 25 01:08:16 2017 mstenber
 * Last modified: Wed Jan  3 10:46:24 2018 mstenber
 * Edit time:     694 min
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
	"fmt"
	"log"
	"strings"

	"github.com/fingon/go-tfhfs/mlog"
)

const hashSize = 32

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
	self.setNodeMaximumSize(maximumSize)
	self.backend = backend
	self.placeholderValue = fmt.Sprintf("hash-%s",
		strings.Repeat("x", hashSize-5))
	return &self
}

func (self *IBTree) LoadRoot(bid BlockId) *IBNode {
	mlog.Printf2("ibtree/ibtree", "t.LoadRoot %x", bid)
	data := self.backend.LoadNode(bid)
	if data == nil {
		return nil
	}
	return &IBNode{tree: self, IBNodeData: *data, blockId: &bid}
}

// NewRoot creates a new node; by default, it is essentially new tree
// of its own. IBTree is really just factory for root nodes.
func (self *IBTree) NewRoot() *IBNode {
	mlog.Printf2("ibtree/ibtree", "t.NewRoot")
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
	blockId *BlockId // on disk, if any
	tree    *IBTree
}

func (self *IBNode) Delete(key IBKey, st *IBStack) *IBNode {
	self.search(key, st)
	c := st.child()
	if c.Key != key {
		log.Panic("ibp.Delete: Key missing ", key)
	}
	st.rewriteAtIndex(true, nil)
	return st.commit()
}

// Commit pushes dirty IBNode to disk, returning the new root.
func (self *IBNode) Commit() (*IBNode, BlockId) {
	// Iterate through the tree, updating the nodes as we go.

	if self.blockId != nil {
		mlog.Printf2("ibtree/ibtree", "in.Commit, unchanged")
		return self, *self.blockId
	}

	cl := self.Children
	if !self.Leafy {
		// Need to copy children
		cl = make([]*IBNodeDataChild, len(self.Children))
		for i, c := range self.Children {
			if c.childNode != nil {
				_, bid := c.childNode.Commit()
				c = &IBNodeDataChild{Key: c.Key, Value: string(bid)}
			}
			cl[i] = c
		}
	}

	n := self.copy()
	n.Children = cl

	bid := self.tree.backend.SaveNode(n.IBNodeData)
	n.blockId = &bid
	mlog.Printf2("ibtree/ibtree", "in.Commit, new bid %x..", bid[:10])
	return n, bid
}

func (self *IBNode) DeleteRange(key1, key2 IBKey, st2 *IBStack) *IBNode {
	if key1 > key2 {
		log.Panic("ibt.DeleteRange: first key more than second key", key1, key2)
	}
	mlog.Printf2("ibtree/ibtree", "DeleteRange [%v..%v]", key1, key2)
	st2.top = 0
	st := *st2
	self.searchLesser(key1, &st)
	mlog.Printf2("ibtree/ibtree", "c1:%v @%v", st.child(), st.indexes)
	self.searchGreater(key2, st2)
	mlog.Printf2("ibtree/ibtree", "c2:%v @%v", st2.child(), st2.indexes)
	// No matches at all?
	if st == *st2 {
		return self
	}
	unique := 0
	for i := 0; i < st.top && st.indexes[i] == st2.indexes[i]; i++ {
		unique = i + 1
	}
	if st.indexes == st2.indexes {
		return self
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
	ncl := make([]*IBNodeDataChild, len(cl)-(idx2-idx1)+1)
	copy(ncl, cl[:idx1])
	ncl[idx1] = st.child()
	if len(cl) > idx2 {
		copy(ncl[(idx1+1):], cl[idx2:])
	}
	st2.rewriteNodeChildren(ncl)
	return st2.commit()
}

func (self *IBNode) Get(key IBKey, st *IBStack) *string {
	self.search(key, st)
	c := st.child()
	st.top = 0
	if c == nil || c.Key != key {
		return nil
	}
	return &c.Value

}

func (self *IBNode) NextKey(key IBKey, st *IBStack) *IBKey {
	self.searchGreater(key, st)
	c := st.child()
	st.top = 0
	if c == nil {
		return nil
	}
	return &c.Key
}

func (self *IBNode) Set(key IBKey, value string, st *IBStack) *IBNode {
	self.search(key, st)
	child := &IBNodeDataChild{Key: key, Value: value}
	c := st.child()
	if c == nil || c.Key != key {
		// now at next -> insertion point is where it pointing at
		st.addChildAt(child)
	} else {
		if st.child().Value == value {
			return self
		}
		st.rewriteAtIndex(true, child)
	}
	return st.commit()

}

func (self *IBNode) search(key IBKey, st *IBStack) {
	if st.nodes[0] == nil {
		st.nodes[0] = self
	} else if self != st.nodes[0] {
		log.Panic("historic/wrong self:", self, " != ", st.nodes[0])
	}
	if st.top > 0 {
		log.Panic("leftover stack")
	}
	st.search(key)
}

func (self *IBNode) copy() *IBNode {
	return &IBNode{tree: self.tree, blockId: self.blockId,
		IBNodeData: self.IBNodeData}
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
	bid := BlockId(c.Value)
	nd := self.tree.backend.LoadNode(bid)
	if nd == nil {
		return nil
	}
	return &IBNode{tree: self.tree, blockId: &bid, IBNodeData: *nd}
}

func (self *IBNode) PrintToMLogDirty() {
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

func (self *IBNode) PrintToMLogAll() {
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
				k0 := n.Children[i-1].Key
				if k0 >= c.Key {
					defer mlog.SetPattern(".")()
					self.PrintToMLogDirty()
					log.Panic("tree broke: ", k0, " >= ", c.Key)
				}
			}
		}

	})
}

func (self *IBNode) nestedNodeCount() int {
	cnt := 0
	self.iterateLeafFirst(func(n *IBNode) {
		cnt++
	})
	return cnt
}

func (self *IBNode) searchLesser(key IBKey, st *IBStack) {
	self.search(key, st)
	c := st.child()
	if c != nil && c.Key == key {
		mlog.Printf2("ibtree/ibtree", "moving to previous leaf from %v", st.indexes)
		st.goPreviousLeaf()
	}
}

func (self *IBNode) searchGreater(key IBKey, st *IBStack) {
	self.search(key, st)
	c := st.child()
	if c != nil && c.Key == key {
		mlog.Printf2("ibtree/ibtree", "moving to next leaf from %v", st.indexes)
		st.goNextLeaf()
	}
}
