/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2017 Markus Stenberg
 *
 * Created:       Fri Aug 11 13:06:15 2017 mstenber
 * Last modified: Fri Aug 11 16:09:24 2017 mstenber
 * Edit time:     41 min
 *
 */

package btree

import (
	"fmt"
	"testing"

	"github.com/stvp/assert"
)

func TestSimple(t *testing.T) {
	tn := NewTreeNode(64)
	assert.Equal(t, tn.firstLeaf(), nil)
	n1 := NewLeafNode([]byte("foo.txt"), nil)
	n2 := NewLeafNode([]byte("bar.txt"), nil)
	n3 := NewLeafNode([]byte("baz.txt"), nil)
	tn.AddChild(n1)
	tn.AddChild(n2)
	assert.Equal(t, tn.searchEq(n3), nil)
	assert.Equal(t, tn.searchEq(n2), n2)
	assert.Equal(t, tn.searchEq(n1), n1)
	assert.True(t, tn.childIndexGE(n1) < len(tn.children),
		"n1 sane childIndex")
	assert.True(t, tn.childIndexGE(n2) < len(tn.children),
		"n2 sane childIndex")
	tn.ensureSane()
	tn.RemoveChild(n2)
	tn.ensureSane()
	assert.Equal(t, tn.searchEq(n2), nil,
		"RemoveChild(n2) should remove n2")
	tn.RemoveChild(n1)
}

func TestBig(t *testing.T) {
	tests := []struct {
		hash bool
	}{{false}, {true}}
	for _, m := range tests {
		tn := NewTreeNode(2000)
		leaves := []*LeafNode{}
		assert.True(t, tn.depth() == 1)
		for i := 0; i < 1000; i++ {
			// fmt.Printf("%s: %d\n", m, i)
			name := []byte(fmt.Sprintf("%04d", i))
			ln := NewLeafNode(name, nil)
			if !m.hash {
				ln.key = ln.name
			}
			tn.AddToTree(ln)
			leaves = append(leaves, ln)

			tn.ensureSane()

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
			tn.RemoveFromTree(n)
			assert.Equal(t, tn.searchEq(n), nil)
			tn.ensureSane()
		}
		assert.Equal(t, tn.depth(), 1)
	}
}
