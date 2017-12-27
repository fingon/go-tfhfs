/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2017 Markus Stenberg
 *
 * Created:       Wed Dec 27 17:19:12 2017 mstenber
 * Last modified: Wed Dec 27 17:19:16 2017 mstenber
 * Edit time:     0 min
 *
 */
package ibtree

import (
	"log"
	"sort"
)

// ibStack is internal utility class which is used to keep trace about
// stack of nodes on the current immutable tree path (parents mainly).
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
