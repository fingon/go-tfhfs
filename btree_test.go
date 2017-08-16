/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2017 Markus Stenberg
 *
 * Created:       Fri Aug 11 13:06:15 2017 mstenber
 * Last modified: Wed Aug 16 14:17:01 2017 mstenber
 * Edit time:     49 min
 *
 */

package btree

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/stvp/assert"
)

func ensureSane(self *TreeNode) {
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
			ensureSane(tn2)
		} else {
			//ln := n.(*LeafNode)
			//fmt.Printf("%s\n", ln.name)
		}
	}
}

func TestSimple(t *testing.T) {
	tt := NewTree(64, nil)
	tn := tt.root
	assert.Equal(t, tn.firstLeaf(), (*LeafNode)(nil))
	n1 := NewLeafNode([]byte("foo.txt"), nil)
	n2 := NewLeafNode([]byte("bar.txt"), nil)
	n3 := NewLeafNode([]byte("baz.txt"), nil)
	tn.AddChild(n1)
	tn.AddChild(n2)
	cnt := 0
	tt.IterateLeaves(nil, func(ln *LeafNode) bool {
		cnt++
		return true
	})
	assert.Equal(t, cnt, 2)
	// Iteration should stop if we return false -> just 1
	tt.IterateLeaves(nil, func(ln *LeafNode) bool {
		assert.Equal(t, ln, tn.firstLeaf())
		return false
	})
	// We should get _one_ result if we iterate non-first
	tt.IterateLeaves(tn.firstLeaf(), func(ln *LeafNode) bool {
		assert.True(t, ln != tn.firstLeaf())
		cnt++
		return true
	})
	assert.Equal(t, cnt, 3)
	assert.Equal(t, tn.searchEq(n3), nil)
	assert.Equal(t, tn.searchEq(n2), n2)
	assert.Equal(t, tn.searchEq(n1), n1)
	assert.True(t, tn.childIndexGE(n1) < len(tn.children),
		"n1 sane childIndex")
	assert.True(t, tn.childIndexGE(n2) < len(tn.children),
		"n2 sane childIndex")
	ensureSane(tn)
	tn.RemoveChild(n2)
	ensureSane(tn)
	assert.Equal(t, tn.searchEq(n2), nil,
		"RemoveChild(n2) should remove n2")
	tn.RemoveChild(n1)
}

func TestBig(t *testing.T) {
	tests := []struct {
		hash bool
	}{{false}, {true}}
	for _, m := range tests {
		tt := NewTree(2000, nil)
		tn := tt.root
		leaves := []*LeafNode{}
		assert.True(t, tn.depth() == 1)
		for i := 0; i < 1000; i++ {
			// fmt.Printf("%s: %d\n", m, i)
			name := []byte(fmt.Sprintf("%04d", i))
			ln := NewLeafNode(name, nil)
			if !m.hash {
				ln.key = ln.name
			}
			tt.Add(ln)
			leaves = append(leaves, ln)

			ensureSane(tn)

			// Then sanity checking starts..
			found := 0
			for i, n := range leaves {
				if tn.searchEq(n) == n {
					found++
				} else {
					fmt.Printf("missing %d %v\n", i, n.key)
				}
			}
			assert.Equal(t, found, len(leaves),
				"broken search ", i, " found only ", found)
		}
		assert.True(t, tn.depth() > 1)
		if !m.hash {
			assert.Equal(t, tn.firstLeaf().Key(), []byte("0000"))
		}
		for _, n := range leaves {
			assert.Equal(t, tn.searchEq(n), n)
			tt.Remove(n)
			assert.Equal(t, tn.searchEq(n), nil)
			ensureSane(tn)
		}
		assert.Equal(t, tn.depth(), 1)
	}
}
