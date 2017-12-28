/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2017 Markus Stenberg
 *
 * Created:       Mon Dec 25 01:08:16 2017 mstenber
 * Last modified: Thu Dec 28 19:45:57 2017 mstenber
 * Edit time:     652 min
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
	self.setNodeMaximumSize(maximumSize)
	self.backend = backend
	self.placeholderValue = fmt.Sprintf("hash-%s",
		strings.Repeat("x", hashSize-5))
	return &self
}

func (self *IBTree) LoadRoot(bid BlockId) *IBNode {
	data := self.backend.LoadNode(bid)
	if data == nil {
		return nil
	}
	return &IBNode{tree: self, IBNodeData: *data}
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
	blockId *BlockId // on disk, if any
	tree    *IBTree
}

func (self *IBNode) Delete(key IBKey, st *IBStack) *IBNode {
	err := self.search(key, st)
	if err != nil {
		log.Panic(err)
	}
	c := st.child()
	if c.Key != key {
		log.Panic("ibp.Delete: Key missing ", key)
	}
	st.rewriteAtIndex(true, nil)
	return st.commit()
}

// Commit pushes dirty IBNode to disk.
//
// After it has been called, the data will be persisted to disk and
// SHOULD NOT BE USED ANYMORE (nor any other related non-persisted
// copies). Instead, the new returned treenode pointer should be used.
func (self *IBNode) Commit() *IBNode {
	// Iterate through the tree, updating the nodes as we go.

	// TBD if this inplace mutation is nasty hack or not. It makes
	// this much faster AND anything being committed should come
	// from mutated version anyway.

	if self.blockId != nil {
		return self
	}

	self.iterateLeafFirst(func(n *IBNode) {
		// if it is already persisted, not interesting
		if n.blockId != nil {
			return
		}

		if !n.Leafy {
			// Update block ids if any
			for _, v := range n.Children {
				if v.childNode != nil {
					v.Value = string(*v.childNode.blockId)
				}
			}

		}
		bid := self.tree.backend.SaveNode(n.IBNodeData)
		n.blockId = &bid
	})
	return self
}

func (self *IBNode) DeleteRange(key1, key2 IBKey, st2 *IBStack) *IBNode {
	if key1 > key2 {
		log.Panic("ibt.DeleteRange: first key more than second key", key1, key2)
	}
	//log.Printf("DeleteRange [%v..%v]", key1, key2)
	st2.top = 0
	st := *st2
	err := self.searchLesser(key1, &st)
	if err != nil {
		log.Panic(err)
	}
	//log.Printf("c1:%v @%v", st.child(), st.indexes)
	err = self.searchGreater(key2, st2)
	if err != nil {
		log.Panic(err)
	}
	//log.Printf("c2:%v @%v", st2.child(), st2.indexes)
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
	//log.Printf("unique:%d", unique)
	for st2.top > unique {
		idx := st2.index()
		if idx > 0 {
			//log.Printf("removing @%d[<%d]", st2.top, idx)
			st2.rewriteNodeChildrenWithCopyOf(st2.node().Children[idx:])
		}
		st2.pop()
	}
	for st.top > unique {
		cl := st.node().Children
		idx := st.index()
		if idx < (len(cl) - 1) {
			//log.Printf("removing @%d[>%d]", st.top, idx)
			st.rewriteNodeChildrenWithCopyOf(cl[:(idx + 1)])
		}
		st.pop()
	}

	// nodes[unique] should be same
	// indexes[unique] should differ
	cl := st2.node().Children
	idx1 := st.indexes[st.top]
	idx2 := st2.indexes[st.top]
	//log.Printf("idx1:%d idx2:%d", idx1, idx2)
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
	err := self.search(key, st)
	if err != nil {
		log.Panic(err)
	}
	c := st.child()
	st.top = 0
	if c == nil || c.Key != key {
		return nil
	}
	return &c.Value

}

func (self *IBNode) Set(key IBKey, value string, st *IBStack) *IBNode {
	err := self.search(key, st)
	if err != nil {
		log.Panic(err)
	}
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

func (self *IBNode) search(key IBKey, st *IBStack) error {
	if st.nodes[0] == nil {
		st.nodes[0] = self
	} else if self != st.nodes[0] {
		log.Panic("historic/wrong self:", self, " != ", st.nodes[0])
	}
	if st.top > 0 {
		log.Panic("leftover stack")
	}
	return st.search(key)
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
				k0 := n.Children[i-1].Key
				if k0 >= c.Key {
					self.print(0)
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

func (self *IBNode) searchLesser(key IBKey, st *IBStack) (err error) {
	err = self.search(key, st)
	if err != nil {
		return
	}
	c := st.child()
	if c != nil && c.Key == key {
		//log.Printf("moving to previous leaf from %v", st.indexes)
		st.goPreviousLeaf()
	}
	return
}

func (self *IBNode) searchGreater(key IBKey, st *IBStack) (err error) {
	err = self.search(key, st)
	if err != nil {
		return
	}
	c := st.child()
	if c != nil && c.Key == key {
		//log.Printf("moving to next leaf from %v", st.indexes)
		st.goNextLeaf()
	}
	return
}
