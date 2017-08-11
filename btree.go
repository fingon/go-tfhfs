/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2017 Markus Stenberg
 *
 * Created:       Fri Aug 11 11:00:14 2017 mstenber
 * Last modified: Fri Aug 11 16:06:44 2017 mstenber
 * Edit time:     228 min
 *
 */

/* Go version of the btree.py in original tfhfs.

   Not very idiomatic Go yet, but everyone has to start somewhere.
   (My first own Go module, yay.)
*/

package btree

import (
	"bytes"
	"fmt"
	"hash/fnv"
	"sort"
)

const hash_size = 8
const name_max_size = 256
const paranoid = true

// Node is interface fulfilled by all btree nodes, both leaf and non-leaf.
type Node interface {
	// Size returns estimated encoded size of the node
	Size() int

	// Key returns the key that can be used for e.g. sorting
	Key() []byte

	Parent() *TreeNode
	SetParent(*TreeNode)
}

type NodeBase struct {
	parent *TreeNode
	key    []byte
}

func (self *NodeBase) Key() []byte {
	return self.key
}

func (self *NodeBase) Parent() *TreeNode {
	return self.parent
}

func (self *NodeBase) SetParent(tn *TreeNode) {
	if paranoid {
		if tn == nil {
			if self.parent == nil {
				panic("SetParent=nil without parent set")
			}
		} else {
			if self.parent != nil {
				panic("SetParent non-nil with parent set")
			}
		}
	}
	self.parent = tn
}

// LeafNode is a node which contains concrete data
type LeafNode struct {
	NodeBase
	name  []byte // actual name of the node
	Value interface{}
}

func NewLeafNode(name []byte, value interface{}) *LeafNode {
	n := new(LeafNode)
	n.name = name
	n.Value = value
	h := fnv.New64a()
	h.Write(name)
	n.key = append(h.Sum([]byte("")), name...)
	return n
}

func (self *LeafNode) Size() int {
	return len(self.key)
}

type TreeNode struct {
	NodeBase
	children   []Node
	child_keys [][]byte
	csize      int // (current) child size
	msize      int // maximum size
}

func (self TreeNode) Size() int {
	return hash_size + name_max_size // TBD: why?
}

func (self *TreeNode) childIndex(n Node, fun func(int) bool) int {
	index := sort.Search(len(self.children), fun)
	return index
}

func (self *TreeNode) childIndexGT(n Node) int {
	k := n.Key()
	return self.childIndex(n, func(i int) bool {
		return bytes.Compare(self.child_keys[i], k) > 0
	})
}

func (self *TreeNode) childIndexGE(n Node) int {
	k := n.Key()
	return self.childIndex(n, func(i int) bool {
		return bytes.Compare(self.child_keys[i], k) >= 0
	})
}

// AddChildNoCheck adds child to the TreeNode at index (if >= 0)
func (self *TreeNode) AddChildNoCheck(n Node, index int) int {
	s := n.Size()
	self.csize += s
	if index < 0 {
		index = self.childIndexGE(n)
	}
	k := n.Key()
	self.children = append(self.children, n)
	self.child_keys = append(self.child_keys, k)
	if index+1 < len(self.children) {
		copy(self.children[index+1:], self.children[index:])
		self.children[index] = n
		copy(self.child_keys[index+1:], self.child_keys[index:])
		self.child_keys[index] = k
	}
	n.SetParent(self)
	return index
}

func (self *TreeNode) MarkDirty() {
	// TBD
}

func NewTreeNode(msize int) *TreeNode {
	return &TreeNode{msize: msize}
}

func (self *TreeNode) new() *TreeNode {
	return NewTreeNode(self.msize)
}

// AddChild adds child to TreeNode and updates keys/splits as needed
func (self *TreeNode) AddChild(n Node) {
	idx := self.AddChildNoCheck(n, -1)
	if idx == 0 {
		self.updateKey(false)
	}
	self.MarkDirty()
	if self.csize <= self.msize {
		return
	}
	tn := self.new()
	for tn.csize < self.csize {
		tn.AddChildNoCheck(self.popChild(-1), 0)
	}
	tn.key = tn.child_keys[0]
	if self.parent != nil {
		// fmt.Println("recursed to parent to add")
		self.parent.AddChild(tn)
		tn.MarkDirty()
		return
	}
	// fmt.Printf("root - spawning children - idx was:%d\n", idx)
	// We're root, we have to just have two new children and no
	// content of our own
	tn2 := self.new()
	for len(self.children) > 0 {
		tn2.AddChildNoCheck(self.popChild(-1), 0)
	}
	tn2.key = tn2.child_keys[0]
	self.AddChildNoCheck(tn2, 0)
	self.AddChildNoCheck(tn, 1)
	// fmt.Printf("post split %d / %d\n", len(tn2.children), len(tn.children))
	tn2.MarkDirty()
	tn.MarkDirty()
}

// RemoveChildNoCheck removes child from the TreeNode at index (if >= 0)
func (self *TreeNode) RemoveChildNoCheck(n Node, index int) {
	if index < 0 {
		index = self.childIndexGE(n)
	}
	self.csize -= n.Size()
	if index == 0 {
		self.children = self.children[1:]
		self.child_keys = self.child_keys[1:]
	} else if index+1 < len(self.children) {
		self.children = append(self.children[:index],
			self.children[index+1:]...)
		self.child_keys = append(self.child_keys[:index],
			self.child_keys[index+1:]...)
	} else {
		self.children = self.children[:index]
		self.child_keys = self.child_keys[:index]
	}
	n.SetParent(nil)
}

func (self *TreeNode) updateKey(force bool) {
	if self.parent == nil {
		return
	}
	nk := self.children[0].Key()
	if self.parent == nil || (!force && bytes.Compare(self.key, nk) <= 0) {
		return
	}
	idx := self.parent.childIndexGE(self)
	// fmt.Printf("updateKey @%d/%d\n", idx, len(self.parent.children))
	self.key = nk
	self.parent.child_keys[idx] = nk
	if idx == 0 {
		self.parent.updateKey(false)
	}
}

func (self *TreeNode) popChild(index int) Node {
	if index < 0 {
		index = len(self.children) + index
	}
	n := self.children[index]
	self.RemoveChildNoCheck(n, index)
	return n
}

func (self *TreeNode) AddToTree(n Node) {
	n2 := self.searchPrevOrEq(n)
	if n2 != nil {
		n2.Parent().AddChild(n)
	} else {
		self.AddChild(n)
	}
}

func (self *TreeNode) getSib(ofs int) *TreeNode {
	if self.parent == nil {
		return nil
	}
	idx := self.parent.childIndexGE(self) + ofs
	if idx >= 0 && idx < len(self.parent.children) {
		return self.parent.children[idx].(*TreeNode)
	}
	return nil
}

func (self *TreeNode) RemoveChild(n Node) {
	self.RemoveChildNoCheck(n, -1)
	if self.csize >= self.msize/4 {
		return
	}
	equalize := func(sib *TreeNode, idx int) bool {
		if sib.csize < self.msize/2 {
			return false
		}
		for sib.csize >= self.csize {
			self.AddChildNoCheck(sib.popChild(idx), -1)
		}
		if idx == -1 {
			// Moved stuff from left to us
			self.updateKey(true)
		} else {
			// Moved stuff from right to us
			sib.updateKey(true)
		}
		return true
	}
	sib := self.getSib(-1)
	if sib != nil && equalize(sib, -1) {
		return
	}

	sib2 := self.getSib(1)
	if sib2 != nil && equalize(sib2, 0) {
		return
	}
	// No siblings -> we're last one left
	if sib == nil && sib2 == nil {
		return
	}
	if sib != nil && sib2 != nil {
		if sib.csize > sib2.csize {
			sib = sib2
		}
	} else if sib2 != nil {
		sib = sib2
	}
	// Cannot trigger rebalance; have to stick our content to sib
	// (ideally smaller of the two) and delete us.
	for len(self.children) > 0 {
		sib.AddChildNoCheck(self.popChild(0), -1)
	}
	sib.updateKey(false)
	self.parent.RemoveChild(self)
}

func (self *TreeNode) RemoveFromTree(n Node) {
	n2 := self.searchEq(n)
	n2.Parent().RemoveChild(n2)
	if self.isLeafy() {
		return
	}
	ts := 0
	for _, c := range self.children {
		ts += c.(*TreeNode).csize
	}
	if ts >= self.msize/4 {
		return
	}

	// Out of children -> should remove everything from the children
	// and add it to us
	original_children := make([]Node, len(self.children))
	copy(original_children, self.children)

	for _, c := range original_children {
		self.RemoveChildNoCheck(c, 0)
		ct := c.(*TreeNode)
		for _, c2 := range ct.children {
			ct.RemoveChildNoCheck(c2, -1)
			self.AddChildNoCheck(c2, -1)
		}
	}

}

func (self *TreeNode) searchPrevOrEq(n Node) Node {
	if len(self.children) == 0 {
		return nil
	}
	idx := self.childIndexGT(n)
	if idx > 0 {
		idx -= 1
	}
	cn := self.children[idx]
	tcn, ok := cn.(*TreeNode)
	if !ok {
		return cn
	}
	return tcn.searchPrevOrEq(n)
}

func (self *TreeNode) searchEq(n Node) Node {
	sc := self.searchPrevOrEq(n)
	if sc != nil && bytes.Equal(sc.Key(), n.Key()) {
		return sc
	}
	return nil
}

func (self *TreeNode) ensureSane() {
	// Make sure this particular tree node seems sane; to the point:
	if len(self.children) == 0 {
		return
	}
	if self.parent != nil {
		idx := self.parent.childIndexGE(self)
		if self.parent.children[idx] != self {
			panic("unable to find self in parent")
		}
	}
	if bytes.Compare(self.key, self.child_keys[0]) > 0 {
		panic("broken own key")
	}
	for i, k := range self.child_keys {
		if !bytes.Equal(k, self.children[i].Key()) {
			panic(fmt.Sprintf("broken child key %d - cache:%v <> Key():%v",
				i, k, self.children[i].Key()))
		}
	}
	// all children are sane (if they are TreeNodes)
	for _, n := range self.children {
		if n.Parent() != self {
			panic("broken parent relation")
		}
		tn2, ok := n.(*TreeNode)
		if ok {
			tn2.ensureSane()
		} else {
			//ln := n.(*LeafNode)
			//fmt.Printf("%s\n", ln.name)
		}
	}
}

func (self *TreeNode) isLeafy() bool {
	if len(self.children) == 0 {
		return true
	}
	cn := self.children[0]
	_, ok := cn.(*TreeNode)
	if !ok {
		return true
	}
	return false
}

func (self *TreeNode) depth() int {
	if self.isLeafy() {
		return 1
	}
	return 1 + self.children[0].(*TreeNode).depth()
}

func (self *TreeNode) firstLeaf() Node {
	if len(self.children) == 0 {
		return nil
	}
	cn := self.children[0]
	tcn, ok := cn.(*TreeNode)
	if !ok {
		return cn
	}
	return tcn.firstLeaf()
}
