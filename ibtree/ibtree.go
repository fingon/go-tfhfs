/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2017 Markus Stenberg
 *
 * Created:       Mon Dec 25 01:08:16 2017 mstenber
 * Last modified: Wed Dec 27 17:11:38 2017 mstenber
 * Edit time:     527 min
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
	"strings"
)

const hashSize = 32

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

func (self *IBTree) setNodeMaximumSize(maximumSize int) {
	self.NodeMaximumSize = maximumSize
	self.halfSize = self.NodeMaximumSize / 2
	self.smallSize = self.halfSize / 2
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

	// default values are for insert (most common?)
	nl := len(n.Children) + 1 // length of new children
	oidx := idx               // old child index to copy from later part
	nidx := idx + 1           // new child index to copy to later part
	if replace {
		// replace
		nl--
		oidx++
		if child == nil {
			// delete
			nl--
			nidx--
		}
	} else if child == nil {
		log.Panic("nonsense argument - non-replace with nil?")
	}
	//log.Printf("rewriteAtIndex top:%d idx:%d oidx:%d nidx:%d => %d items",
	//self.top, idx, oidx, nidx, nl)

	c := make([]*IBNodeDataChild, nl)
	if idx > 0 {
		copy(c, n.Children[:idx])
	}
	if child != nil {
		c[idx] = child
	}
	copy(c[nidx:], n.Children[oidx:])
	self.rewriteNodeChildren(c)
}

func (self *ibStack) rewriteNodeChildren(children []*IBNodeDataChild) {
	//log.Printf("rewriteNodeChildren")
	n := self.node().copy()
	n.Children = children
	self.nodes[self.top] = n
	// This invalidates sub-trees (if any)
	self.nodes[self.top+1] = nil
}

func (self *ibStack) rewriteNodeChildrenWithCopyOf(children []*IBNodeDataChild) {
	ochildren := children
	children = make([]*IBNodeDataChild, len(ochildren))
	copy(children, ochildren)
	self.rewriteNodeChildren(children)
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
	//log.Printf("node() from %v [%d]", self.nodes, self.top)
	return self.nodes[self.top]
}

func (self *ibStack) childSibNode(ofs int) *IBNode {
	idx := self.index() + ofs
	n := self.node()
	if idx < 0 || idx >= len(n.Children) {
		return nil
	}
	return n.childNode(idx)
}

func (self *ibStack) pop() {
	n := self.node()
	self.indexes[self.top] = -1
	self.top--
	if self.top < 0 {
		log.Panic("popped beyond top! madness!")
	}
	c := &IBNodeDataChild{Key: n.Children[0].Key,
		Value:     n.tree.placeholderValue,
		childNode: n}
	self.rewriteAtIndex(true, c)
}

func (self *ibStack) push(index int, node *IBNode) {
	self.indexes[self.top] = index
	self.top++
	self.nodes[self.top] = node
	self.indexes[self.top] = 0
}

func (self *ibStack) moveFrom(ofs int, sib *IBNode) {
	// Keep track of which child we are really at
	node := self.child().childNode
	oi := self.index()
	si := ofs + oi

	// Grab one child from start/end of sib
	var cl []*IBNodeDataChild
	var c *IBNodeDataChild
	if ofs == -1 {
		cofs := len(sib.Children) - 1
		cl = sib.Children[:cofs]
		c = sib.Children[cofs]
	} else {
		cl = sib.Children[1:]
		c = sib.Children[0]
	}

	// Change active node to sib
	self.push(si, sib)
	self.rewriteNodeChildrenWithCopyOf(cl)
	self.pop()

	// Now rewrite current node
	self.push(oi, node)
	if ofs == -1 {
		self.indexes[self.top] = 0
	} else {
		self.indexes[self.top] = len(node.Children)
	}
	self.rewriteAtIndex(false, c)

	// Update the node with updated child content
	self.pop()

}

func (self *ibStack) mergeTo(ofs int, sib *IBNode) {
	clen1 := len(self.node().Children)
	oi := self.index()
	si := ofs + oi

	mycl := self.child().childNode.Children
	cl := sib.Children

	// Delete our own node
	self.rewriteAtIndex(true, nil)

	clen2 := len(self.node().Children)
	if clen1 <= clen2 {
		self.node().print(0)
		log.Panic("broken mergeTo, bad! (p1)")
	}

	// Then handle the node we're sticking things to
	if ofs > 0 {
		si--
	}
	//log.Printf("rewriting %d %s (%d)", si, sib, ofs)
	self.push(si, sib)
	ncl := make([]*IBNodeDataChild, len(mycl)+len(cl))
	if ofs == -1 {
		// append to end
		copy(ncl, cl)
		copy(ncl[len(cl):], mycl)
	} else {
		// insert to beginning
		copy(ncl, mycl)
		copy(ncl[len(mycl):], cl)
	}
	self.rewriteNodeChildren(ncl)
	self.pop()
	clen3 := len(self.node().Children)
	if clen2 != clen3 {
		self.node().print(0)
		log.Panic("broken mergeTo, bad!")
	}
}

// Pop rest of the stack, creating new Nodes as need be, and return
// the top node.
func (self *ibStack) commit() *IBNode {
	for self.top > 0 {
		self.pop()
	}
	if self.smallCount > 0 {
		//log.Printf("commit - pruning")
		self.indexes[0] = 0
		self.iterateMutatingChildLeafFirst(func() {
			//log.Printf("iterating @%d[%d] %v", self.top, self.index(), self.child())
			n := self.child().childNode
			s := n.Msgsize()
			if s >= n.tree.smallSize {
				return
			}
			// Try to look for neighbor with spare nodes to borrow.
			n1 := self.childSibNode(-1)
			n2 := self.childSibNode(1)
			ofs := -1
			//log.Printf("s:%s n1:%s n2:%s", s, n1, n2)
			if n1 != nil && n2 != nil {
				if n1.Msgsize() < n2.Msgsize() {
					n1 = n2
					ofs = 1
				}
			} else if n1 == nil {
				ofs = 1
				n1 = n2
			}
			if n1 == nil {
				return
			}
			s1 := n1.Msgsize()
			if s1 < n.tree.halfSize {
				//log.Printf("mergeTo %d (%d)", ofs, s1)
				self.mergeTo(ofs, n1)
				return
			}
			//log.Printf("moveFrom %d", ofs)
			// Borrow from that sibling
			self.moveFrom(ofs, n1)
		})

		n := self.node()
		if !n.Leafy && n.Msgsize() < n.tree.smallSize {
			ts := 0
			cc := 0
			for i := range n.Children {
				cn := n.childNode(i)
				ts += cn.Msgsize()
				cc += len(cn.Children)
			}
			if ts <= n.tree.NodeMaximumSize {
				//log.Printf("Decreasing depth by 1")
				cl := make([]*IBNodeDataChild, 0, cc)
				leafy := false
				for i := range n.Children {
					cn := n.childNode(i)
					if cn.Leafy {
						leafy = true
					}
					ts += cn.Msgsize()
					cl = append(cl, cn.Children...)
				}
				self.rewriteNodeChildren(cl)
				if leafy {
					// Just copied the node, it is fresh
					self.node().Leafy = true
				}
			}
		}
	}
	return self.node()
}

// Go to the first leaf that has been set, going down from the current
// node.
func (self *ibStack) goDownLeft() {
	n := self.node()
	for i := 0; i < len(n.Children); i++ {
		v := n.Children[i]
		if v.childNode != nil && !v.childNode.Leafy {
			self.push(i, v.childNode)
			self.goDownLeft()
			return
		}
	}
}

func (self *ibStack) moveRight() bool {
	// Current node has been travelled.
	// Options: go right to node's next child, OR recurse to parent.
	n := self.node()
	for i := self.index() + 1; i < len(n.Children); i++ {
		cn := n.Children[i].childNode
		if cn != nil {
			if !cn.Leafy {
				self.push(i, cn)
				self.goDownLeft()
			} else {
				self.indexes[self.top] = i
			}
			return true
		}
	}
	// we are already at last one
	if self.top == 0 {
		return false
	}
	// Nothing found; just go up in the hierarchy.
	self.pop()
	return true

}

func (self *ibStack) iterateMutatingChildLeafFirst(fun func()) {
	if self.node().Leafy {
		return
	}
	self.goDownLeft()
	for {
		on := self.child()
		fun()
		if self.child() == on {
			// log.Printf("child stayed same")
			if !self.moveRight() {
				// log.Printf("no more children")
				return
			}
			nn := self.child()
			if on == nn {
				log.Panic("moveRight returned same child")
			}
		} else {
			//log.Printf("mutated, trying harder")

			// Sanity check (can remove eventually)
			self.node().checkTreeStructure()
		}
	}
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

func (self *ibStack) addChildAt(child *IBNodeDataChild) {
	// Insert child where it belongs
	self.rewriteAtIndex(false, child)

	node := self.node()

	if node.Msgsize() <= node.tree.NodeMaximumSize {
		return
	}

	//log.Printf("Leaf too big")
	s := 0
	i := 0
	for s < node.Msgsize()/2 {
		s += node.Children[i].Msgsize()
		i++
	}
	nodec := node.Children[:i] // passed to rewriteNodeChildren; it will create new array
	nextc := make([]*IBNodeDataChild, len(node.Children)-i)
	copy(nextc, node.Children[i:])
	//log.Printf("c1:%d c2:%d", len(nodec), len(nextc))
	// Remove children from this
	self.rewriteNodeChildren(nodec)

	// And create next node that will have them
	next := &IBNode{tree: node.tree,
		IBNodeData: IBNodeData{Leafy: node.Leafy,
			Children: nextc}}
	nextchild := &IBNodeDataChild{Key: nextc[0].Key,
		Value:     node.tree.placeholderValue,
		childNode: next}
	if self.top > 0 {
		//log.Printf("Adding sibling leaf with key %v", nextc[0].Key)
		old_index := self.indexes[self.top-1]
		self.pop()
		self.indexes[self.top] = old_index + 1
		// next.print(2)
		//log.Printf("top:%d idx:%d", self.top, self.indexes[self.top])
		self.addChildAt(nextchild)
		return
	}

	//log.Printf("Replacing root")
	// Uh oh. Didn't fit to root level. Have to create new root
	// with two children instead.
	node = self.node()
	mechild := &IBNodeDataChild{Key: nodec[0].Key,
		Value:     node.tree.placeholderValue,
		childNode: node}
	self.nodes[0] = &IBNode{tree: node.tree,
		IBNodeData: IBNodeData{
			Children: []*IBNodeDataChild{mechild, nextchild}}}
	self.indexes[0] = -1
}

func (self *IBNode) search(key IBKey, stack *ibStack) error {
	stack.nodes[0] = self
	return stack.search(key)
}

func (self *ibStack) search(key IBKey) error {
	n := self.nodes[0]
	self.top = 0
	//log.Printf("search %v", key)
	for {
		var idx int
		if n.Leafy {
			// Look for insertion point
			idx = sort.Search(len(n.Children),
				func(i int) bool {
					return n.Children[i].Key >= key
				})
			// Last one may point at len(children)
		} else {
			// We look for 'next' interior node, and use
			// the previous one.
			idx = sort.Search(len(n.Children),
				func(i int) bool {
					return n.Children[i].Key > key
				})
			idx--
			if idx < 0 {
				idx = 0
			}
			// Resulting interior nodes must always point
			// at valid next nodes as they are used for
			// subsequent calls, unless tree is empty.
		}
		//log.Printf(" @%d => %d", self.top, idx)
		var on *IBNode
		if idx == self.indexes[self.top] {
			on = self.nodes[self.top+1]
		}
		if n.Leafy {
			self.indexes[self.top] = idx
			//log.Printf(" top:%d, n:%v, idx:%d", self.top, n, idx)
			break
		}
		if on != nil {
			n = on
		} else {
			n = n.childNode(idx)
		}
		self.push(idx, n)
	}
	return nil
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
