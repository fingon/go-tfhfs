/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2017 Markus Stenberg
 *
 * Created:       Fri Aug 11 11:00:14 2017 mstenber
 * Last modified: Wed Aug 16 14:34:27 2017 mstenber
 * Edit time:     309 min
 *
 */

/* Go version of the btree.py in original tfhfs.

   Not very idiomatic Go yet, but everyone has to start somewhere.
   (My first own Go module, yay.)
*/

package btree

import (
	"bytes"
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

var _ Node = &LeafNode{} // Ensure pointer of it it fulfills the Node interface

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

type NodeToTreeNodeCallback func(n Node) *TreeNode
type NewTreeNodeCallback func(t *Tree) *TreeNode

type Tree struct {
	maximumSize            int // maximum size
	halfSize               int // half of maximum size
	smallSize              int // the size below which we try not to get smaller
	nodeToTreeNodeCallback NodeToTreeNodeCallback
	newTreeNodeCallback    NewTreeNodeCallback
	root                   *TreeNode
}

func NewTree(maximumSize int,
	n2tn NodeToTreeNodeCallback,
	ntn NewTreeNodeCallback) *Tree {
	if n2tn == nil {
		n2tn = func(n Node) *TreeNode {
			v, _ := n.(*TreeNode)
			// ok is ignored for now
			return v
		}
	}
	t := &Tree{maximumSize,
		maximumSize / 2,
		maximumSize / 4,
		n2tn, ntn,
		nil}
	t.root = ntn(t)
	return t
}

func (self *Tree) Add(n Node) {
	n2 := self.root.searchPrevOrEq(n)
	if n2 != nil {
		n2.Parent().AddChild(n)
	} else {
		self.root.AddChild(n)
	}
}

type NodeCallback func(n Node) bool

func (self *Tree) nextLeaf(n Node, ofs int) (Node, int) {
	// Fast common case - handle the 'has more leaves' first
	p := n.Parent()
	if ofs < 0 {
		ofs = p.childIndexGE(n)
	}
	nofs := ofs + 1
	if nofs < len(p.children) {
		return p.children[nofs], nofs
	}

	// There were no leaves. So we have to recursively try to find
	// the parent node which has next sibling.
	for p.parent != nil {
		sib := p.getSib(1)
		if sib != nil {
			return sib.firstLeaf(), 0
		}
		p = p.parent
	}
	return nil, -1
}

func (self *Tree) IterateLeaves(startAt Node, it NodeCallback) {
	ln := startAt
	i := 0
	if ln == nil {
		ln = self.root.firstLeaf()
	} else {
		ln, i = self.nextLeaf(ln, -1)
	}
	for ln != nil {
		if !it(ln) {
			return
		}
		ln, i = self.nextLeaf(ln, i)
	}
}

func (self *Tree) Remove(n Node) {
	root := self.root
	n2 := root.searchEq(n)
	n2.Parent().RemoveChild(n2)
	if root.isLeafy() {
		return
	}
	ts := 0
	for i := range root.children {
		ts += root.getChildTreeNode(i).childSize
		if ts >= self.smallSize {
			return
		}
	}

	// Out of children -> should remove everything from the children
	// and add it to us
	original_children := make([]Node, len(root.children))
	copy(original_children, root.children)

	for _, c := range original_children {
		root.RemoveChildNoCheck(c, 0)
		ct := self.nodeToTreeNodeCallback(c)
		for _, c2 := range ct.children {
			ct.RemoveChildNoCheck(c2, -1)
			root.AddChildNoCheck(c2, -1)
		}
	}

}

type TreeNode struct {
	NodeBase
	tree       *Tree // tree we belong to
	children   []Node
	child_keys [][]byte
	childSize  int // how big are the children
}

var _ Node = &TreeNode{} // Ensure pointer of it fulfills the Node interface

func (self *TreeNode) Size() int {
	return hash_size + name_max_size // TBD: why?
}

func (self *TreeNode) childIndex(n Node, fun func(int) bool) int {
	index := sort.Search(len(self.children), fun)
	return index
}

func (self *TreeNode) getChildTreeNode(ofs int) *TreeNode {
	if ofs < 0 || ofs >= len(self.children) {
		return nil
	}
	return self.tree.nodeToTreeNodeCallback(self.children[ofs])
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
	self.childSize += s
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

func NewTreeNode(tree *Tree) *TreeNode {
	return &TreeNode{tree: tree}
}

// AddChild adds child to TreeNode and updates keys/splits as needed
func (self *TreeNode) AddChild(n Node) {
	idx := self.AddChildNoCheck(n, -1)
	if idx == 0 {
		self.updateKey(false)
	}
	self.MarkDirty()
	if self.childSize <= self.tree.maximumSize {
		return
	}
	tn := self.tree.newTreeNodeCallback(self.tree)
	for tn.childSize < self.childSize {
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
	tn2 := self.tree.newTreeNodeCallback(self.tree)
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
	self.childSize -= n.Size()
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

func (self *TreeNode) getSib(ofs int) *TreeNode {
	if self.parent == nil {
		return nil
	}
	idx := self.parent.childIndexGE(self) + ofs
	return self.parent.getChildTreeNode(idx)
}

func (self *TreeNode) RemoveChild(n Node) {
	self.RemoveChildNoCheck(n, -1)
	if self.childSize >= self.tree.smallSize {
		return
	}
	equalize := func(sib *TreeNode, idx int) bool {
		if sib.childSize < self.tree.halfSize {
			return false
		}
		for sib.childSize >= self.childSize {
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
		if sib.childSize > sib2.childSize {
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

func (self *TreeNode) searchPrevOrEq(n Node) Node {
	if len(self.children) == 0 {
		return nil
	}
	idx := self.childIndexGT(n)
	if idx > 0 {
		idx -= 1
	}
	cn := self.children[idx]
	tn := self.tree.nodeToTreeNodeCallback(cn)
	if tn == nil {
		return cn
	}
	return tn.searchPrevOrEq(n)
}

func (self *TreeNode) searchEq(n Node) Node {
	sc := self.searchPrevOrEq(n)
	if sc != nil && bytes.Equal(sc.Key(), n.Key()) {
		return sc
	}
	return nil
}

func (self *TreeNode) isLeafy() bool {
	// empty node by definition is leafy, but otherwise,
	// if it has (first) child which is a TreeNode, it is not leafy
	return len(self.children) == 0 || self.getChildTreeNode(0) == nil
}

func (self *TreeNode) depth() int {
	tn := self.getChildTreeNode(0)
	if tn == nil {
		return 1
	}
	return 1 + tn.depth()
}

func (self *TreeNode) firstLeaf() Node {
	if len(self.children) == 0 {
		return nil
	}
	cn := self.children[0]
	tn := self.tree.nodeToTreeNodeCallback(cn)
	if tn == nil {
		return cn
	}
	return tn.firstLeaf()
}
