/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2017 Markus Stenberg
 *
 * Created:       Wed Dec 27 17:19:12 2017 mstenber
 * Last modified: Tue Jan  9 19:59:56 2018 mstenber
 * Edit time:     175 min
 *
 */
package ibtree

import (
	"fmt"
	"log"
	"sort"

	"github.com/fingon/go-tfhfs/mlog"
)

// Stack is internal utility class which is used to keep trace about
// stack of nodes on the current immutable tree path (parents mainly).
//
// If using lowlevel API (= direct calls to Node), passing empty one
// may be necessary. Otherwise Transaction should be used.
type Stack struct {
	// Static arrays that are used to store the 'trace' of our
	// walk in the tre. By backtracking it at 'commit', we can
	// handle COW of the recursive data structure.

	nodes   [maximumTreeDepth]*Node
	indexes [maximumTreeDepth]int

	// The highest index of the nodes/indexes arrays with the values set.
	top, maxtop int

	// How many nodes have turned small during lifetime of the stack.
	smallCount int
}

// Reset rests the Stack to ~factory defaults. It is still tied to
// particular tree though, and also calling it mid-operation is fatal
// error.
func (self *Stack) Reset() {
	if self.nodes[0] == nil {
		log.Panic("Reset() on uninitialized Stack")
	}
	if self.top > 0 {
		log.Panic("uncommitted state in reset")
	}
	self.setIndex(0)
}

func (self *Stack) setIndex(idx int) {
	if idx == self.indexes[self.top] {
		return
	}
	self.indexes[self.top] = idx
	self.invalidateSubNodes()
}

func (self *Stack) invalidateSubNodes() {
	for i := self.top + 1; i <= self.maxtop; i++ {
		self.nodes[i] = nil
	}
	self.maxtop = self.top

}
func (self *Stack) rewriteAtIndex(replace bool, child *NodeDataChild) {
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
	// mlog.Printf2("ibtree/ibstack", "rewriteAtIndex top:%d idx:%d oidx:%d nidx:%d => %d items", self.top, idx, oidx, nidx, nl)

	c := make([]*NodeDataChild, nl)
	if idx > 0 {
		copy(c, n.Children[:idx])
	}
	if child != nil {
		c[idx] = child
	}
	copy(c[nidx:], n.Children[oidx:])
	self.rewriteNodeChildren(c)
}

func (self *Stack) rewriteNodeChildren(children []*NodeDataChild) {
	// mlog.Printf2("ibtree/ibstack", "rewriteNodeChildren")
	n := self.node().copy()
	n.blockId = nil
	n.Children = children
	self.nodes[self.top] = n
	// This invalidates sub-trees (if any)
	self.invalidateSubNodes()
	if n.Leafy && n.Msgsize() <= n.tree.smallSize {
		self.smallCount++
	}
	// This could be skipped in an emergency but for now it is cheap way to ensure tree stays sane
	n.CheckNodeStructure()
}

func (self *Stack) rewriteNodeChildrenWithCopyOf(ochildren []*NodeDataChild) {
	children := make([]*NodeDataChild, len(ochildren))
	copy(children, ochildren)
	self.rewriteNodeChildren(children)
}

func (self *NodeDataChild) String() string {
	return fmt.Sprintf("ibnc<%x,%x,%v>", self.Key, self.Value, self.childNode)
}

func (self *Stack) child() *NodeDataChild {
	cl := self.node().Children
	index := self.index()
	if index < 0 || index >= len(cl) {
		return nil
	}
	return cl[index]
}

func (self *Stack) childNode(idx int) *Node {
	n := self.node()
	if idx < 0 || idx >= len(n.Children) {
		mlog.Printf2("ibtree/ibstack", "childNode out of bounds (%d out of %d)", idx, len(n.Children))
		return nil
	}
	if self.indexes[self.top] == idx && self.nodes[self.top+1] != nil {
		return self.nodes[self.top+1]
	}
	return self.node().childNode(idx)
}

func (self *Stack) index() int {
	return self.indexes[self.top]
}

func (self *Stack) node() *Node {
	// mlog.Printf2("ibtree/ibstack", "node() from %v [%d]", self.nodes, self.top)
	return self.nodes[self.top]
}

func (self *Stack) popNode() *Node {
	n := self.node()
	self.top--
	if self.top < 0 {
		log.Panic("popped beyond top! madness!")
	}
	return n
}

func (self *Stack) pop() {
	n := self.popNode()
	if len(n.Children) > 0 {
		key := n.Children[0].Key
		c := &NodeDataChild{Key: key,
			Value:     n.tree.placeholderValue,
			childNode: n}
		self.rewriteAtIndex(true, c)
	} else {
		self.rewriteAtIndex(true, nil)
	}
}

func (self *Stack) push(index int, node *Node) {
	self.setIndex(index)
	self.top++
	if self.maxtop < self.top {
		self.maxtop = self.top
	}
	self.nodes[self.top] = node
	self.setIndex(0)
}

func (self *Stack) moveFrom(ofs int, sib *Node) {
	// Keep track of which child we are really at
	node := self.child().childNode
	oi := self.index()
	si := ofs + oi

	// Grab one child from start/end of sib
	var cl []*NodeDataChild
	var c *NodeDataChild
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
		self.setIndex(0)
	} else {
		self.setIndex(len(node.Children))
	}
	self.rewriteAtIndex(false, c)

	// Update the node with updated child content
	self.pop()

}

func (self *Stack) mergeTo(ofs int, sib *Node) {
	clen1 := len(self.node().Children)
	oi := self.index()
	si := ofs + oi

	mycl := self.child().childNode.Children
	cl := sib.Children

	// Delete our own node
	self.rewriteAtIndex(true, nil)

	clen2 := len(self.node().Children)
	if clen1 <= clen2 {
		defer mlog.SetPattern(".")()
		self.node().PrintToMLogDirty()
		log.Panic("broken mergeTo, bad! (p1)")
	}

	// Then handle the node we're sticking things to
	if ofs > 0 {
		si--
	}
	// mlog.Printf2("ibtree/ibstack", "rewriting %d %s (%d)", si, sib, ofs)
	self.push(si, sib)
	ncl := make([]*NodeDataChild, len(mycl)+len(cl))
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
		defer mlog.SetPattern(".")()
		self.node().PrintToMLogDirty()
		log.Panic("broken mergeTo, bad!")
	}
}

// Pop rest of the stack, creating new Nodes as need be, and return
// the top node.
func (self *Stack) commit() *Node {
	for self.top > 0 {
		self.pop()
	}

	// Nothing small was encountered -> we're still good to go
	if self.smallCount == 0 {
		return self.node()
	}

	// Check tree for small nodes
	self.iterateMutatingChildLeafFirst(func() {
		c := self.child()
		// mlog.Printf2("ibtree/ibstack", "iterating @%d[%d] %v", self.top, self.index(), self.child())
		n := c.childNode
		s := n.Msgsize()
		if s >= n.tree.smallSize {
			return
		}
		// Try to look for neighbor with spare nodes to borrow.
		idx := self.index()
		n1 := self.childNode(idx - 1)
		n2 := self.childNode(idx + 1)
		ofs := -1
		mlog.Printf2("ibtree/ibstack", "s:%x n1:%s n2:%s", s, n1, n2)
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
			mlog.Printf2("ibtree/ibstack", "mergeTo %d (%d)", ofs, s1)
			self.mergeTo(ofs, n1)
			return
		}
		mlog.Printf2("ibtree/ibstack", "moveFrom %d", ofs)
		// Borrow from that sibling
		self.moveFrom(ofs, n1)
	})

	// Check root
	n := self.node()
	if !n.Leafy && n.Msgsize() < n.tree.smallSize {
		ts := 0
		cc := 0
		for i := range n.Children {
			cn := self.childNode(i)
			ts += cn.Msgsize()
			cc += len(cn.Children)
		}
		if ts <= n.tree.NodeMaximumSize {
			mlog.Printf2("ibtree/ibstack", "Decreasing depth by 1")
			cl := make([]*NodeDataChild, 0, cc)
			leafy := false
			for i := range n.Children {
				cn := self.childNode(i)
				if cn.Leafy {
					leafy = true
				}
				ts += cn.Msgsize()
				cl = append(cl, cn.Children...)
			}
			self.rewriteNodeChildren(cl)
			n = self.node()
			if leafy {
				// Just copied the node, it is fresh
				n.Leafy = true
			}
		}
	}

	self.smallCount = 0

	mlog.Printf2("ibtree/ibstack", "is.commit done")
	self.node().PrintToMLogDirty()

	return n
}

// Go to the first leaf that has been set, going down from the current
// node.
func (self *Stack) goDownLeft() {
	n := self.node()
	for i := 0; i < len(n.Children); i++ {
		cn := n.Children[i].childNode
		if cn != nil && !cn.Leafy {
			self.push(i, cn)
			self.goDownLeft()
			return
		}
	}
}

// goDownLeftAny goes down any leaf, including clean ones, that are
// loaded from disk if need be.
func (self *Stack) goDownLeftAny() {
	idx := self.index()
	for {
		n := self.node()
		if n.Leafy {
			return
		}
		v := self.childNode(idx)
		self.push(idx, v)
		idx = 0
	}
}

func (self *Stack) pushIndex(idx int) {
	n := self.childNode(idx)
	self.push(idx, n)
}

func (self *Stack) pushCurrentIndex() {
	idx := self.index()
	self.pushIndex(idx)
}

func (self *Stack) goPreviousLeaf() bool {
	for {
		idx := self.index() - 1
		n := self.node()
		if idx >= 0 {
			if !n.Leafy {
				for !n.Leafy {
					self.pushIndex(idx)
					n = self.node()
					idx = len(n.Children) - 1
				}
			}
			self.setIndex(idx)
			return true
		}
		if self.top == 0 {
			if self.indexes[self.top] == 0 {
				self.setIndex(-1)
				return true
			}
			return false
		}
		self.popNode()
	}

}

func (self *Stack) goNextLeaf() bool {
	mlog.Printf2("ibtree/ibstack", "goNextLeaf")
	for {
		idx := self.index() + 1
		n := self.node()
		if idx < len(n.Children) {
			if !n.Leafy {
				for !n.Leafy {
					self.pushIndex(idx)
					n = self.node()
					idx = 0
				}
			} else {
				self.setIndex(idx)
			}
			self.invalidateSubNodes()
			return true

		}
		if self.top == 0 {
			lidx := len(n.Children)
			// go to 'beyond last'
			if self.indexes[self.top] != lidx {
				self.setIndex(lidx)
				return true
			}
			return false
		}
		self.popNode()
	}
}

func (self *Stack) moveRight() bool {
	// Current node has been travelled.
	// Options: go right to node's next child, OR recurse to parent.
	n := self.node()
	for i := self.index() + 1; i < len(n.Children); i++ {
		cn := n.Children[i].childNode
		if cn != nil {
			if !cn.Leafy {
				self.pushIndex(i)
				self.goDownLeft()
			} else {
				self.setIndex(i)
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

func (self *Stack) iterateMutatingChildLeafFirst(fun func()) *Node {
	self.Reset()
	n := self.node()
	if n.Leafy {
		return n
	}
	self.goDownLeft()
	for {
		on := self.child()
		if on.childNode != nil {
			fun()
		}
		if self.child() == on {
			if !self.moveRight() {
				break
			}
			nn := self.child()
			if on == nn {
				log.Panic("moveRight returned same child")
			}
		} else {
			// Sanity check (can remove eventually)
			//self.node().checkTreeStructure()
		}
	}
	for self.top > 0 {
		self.pop()
	}
	return self.nodes[0]
}

func (self *Stack) addChildAt(child *NodeDataChild) {
	// Insert child where it belongs
	self.rewriteAtIndex(false, child)

	node := self.node()

	if node.Msgsize() <= node.tree.NodeMaximumSize {
		return
	}

	mlog.Printf2("ibtree/ibstack", "Leaf too big")
	i := len(node.Children) / 2
	nodec := node.Children[:i] // passed to rewriteNodeChildren; it will create new array
	nextc := make([]*NodeDataChild, len(node.Children)-i)
	copy(nextc, node.Children[i:])
	// mlog.Printf2("ibtree/ibstack", "c1:%d c2:%d", len(nodec), len(nextc))
	// Remove children from this
	self.rewriteNodeChildren(nodec)

	// And create next node that will have them
	next := &Node{tree: node.tree,
		NodeData: NodeData{Leafy: node.Leafy,
			Children: nextc}}
	nextchild := &NodeDataChild{Key: nextc[0].Key,
		Value:     node.tree.placeholderValue,
		childNode: next}
	if self.top > 0 {
		// mlog.Printf2("ibtree/ibstack", "Adding sibling leaf with key %v", nextc[0].Key)
		self.pop()
		self.nextIndex()
		// mlog.Printf2("ibtree/ibstack", "top:%d idx:%d", self.top, self.indexes[self.top])
		self.addChildAt(nextchild)
		return
	}

	mlog.Printf2("ibtree/ibstack", "Replacing root")
	// Uh oh. Didn't fit to root level. Have to create new root
	// with two children instead.
	node = self.node()
	mechild := &NodeDataChild{Key: nodec[0].Key,
		Value:     node.tree.placeholderValue,
		childNode: node}
	self.nodes[0] = &Node{tree: node.tree,
		NodeData: NodeData{
			Children: []*NodeDataChild{mechild, nextchild}}}
	self.invalidateSubNodes()
}

func (self *Stack) search(key Key) {
	n := self.nodes[0]
	self.top = 0
	mlog.Printf2("ibtree/ibstack", "search %x", key)
	for {
		var idx int
		cn := len(n.Children)
		if n.Leafy {
			// Look for insertion point
			idx = sort.Search(cn,
				func(i int) bool {
					k := n.Children[i].Key
					r := k >= key
					mlog.Printf2("ibtree/ibstack", "  check %d: %x >= %x = %v", i, k, key, r)
					return r
				})
			// Last one may point at len(children)
		} else {
			// We look for 'next' interior node, and use
			// the previous one.
			idx = sort.Search(cn,
				func(i int) bool {
					k := n.Children[i].Key
					r := k > key
					mlog.Printf2("ibtree/ibstack", "  check %d: %x > %x = %v", i, k, key, r)
					return r
				})
			idx--
			if idx < 0 {
				idx = 0
			}
			// Resulting interior nodes must always point
			// at valid next nodes as they are used for
			// subsequent calls, unless tree is empty.
		}
		mlog.Printf2("ibtree/ibstack", " [@%d] %v => %d/%d", self.top, n, idx, cn)
		if n.Leafy {
			self.setIndex(idx)
			mlog.Printf2("ibtree/ibstack", " top:%d, n:%v, idx:%d", self.top, n, idx)
			break
		}
		n = self.childNode(idx)
		if n == nil {
			log.Panicf("nil child node at depth %d idx %d", self.top, idx)
		}
		self.push(idx, n)
	}
}

func (self *Stack) nextIndex() {
	self.setIndex(self.indexes[self.top] + 1)
}
