/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2017 Markus Stenberg
 *
 * Created:       Mon Dec 25 17:07:23 2017 mstenber
 * Last modified: Wed Dec 27 17:11:05 2017 mstenber
 * Edit time:     57 min
 *
 */

package ibtree

import (
	"fmt"
	"log"
	"testing"

	"github.com/stvp/assert"
)

const debug = 0
const n = 10000
const nodeSize = 256

func checkTree2(t *testing.T, r *IBNode, n int, st int) {
	if debug > 1 {
		r.print(0)
		log.Printf("checkTree [%d..%d[\n", st, n)
	}
	for i := st - 1; i <= n; i++ {
		if debug > 1 {
			log.Printf(" #%d\n", i)
		}
		s := fmt.Sprintf("%d", i)
		v := r.Get(IBKey(s))
		if i == (st-1) || i == n {
			assert.Nil(t, v)
		} else {
			assert.True(t, v != nil)
			assert.Equal(t, s, *v)
		}
	}
	r.checkTreeStructure()
}

func checkTree(t *testing.T, r *IBNode, n int) {
	checkTree2(t, r, n, 0)
}

func treeNodeCount(n *IBNode) int {
	cnt := 0
	n.iterateLeafFirst(func(n *IBNode) {
		cnt++
	})
	return cnt
}

func TestIBTree(t *testing.T) {
	tree := IBTree{}.Init(nil)
	tree.setNodeMaximumSize(nodeSize) // more depth = smaller examples that blow up
	r := tree.NewRoot()
	v := r.Get(IBKey("foo"))
	assert.Nil(t, v)
	for i := 0; i < n; i++ {
		if debug > 1 {
			checkTree(t, r, i) // previous tree should be ok
			log.Printf("Inserting #%d\n", i)
		}
		cnt := 0
		var st ibStack
		st.nodes[0] = r
		st.iterateMutatingChildLeafFirst(func() {
			if debug > 1 {
				log.Printf("Child %v %v", st.indexes, st.child())
			}
			cnt++
		})
		assert.Equal(t, cnt, treeNodeCount(r)-1, "iterateMutatingChildLeafFirst broken")
		s := fmt.Sprintf("%d", i)
		r = r.Set(IBKey(s), s)
	}

	// Ensure in-place mutate works fine as well and does not change r
	rr := r.Set(IBKey("0"), "z")
	assert.Equal(t, "z", *rr.Get(IBKey("0")))
	assert.Equal(t, "0", *r.Get(IBKey("0")))

	checkTree(t, r, n)
	r2 := r
	for i := 0; i < n; i++ {
		if debug > 0 {
			checkTree2(t, r2, n, i)
		}
		if debug > 0 {
			log.Printf("Deleting #%d\n", i)
		}
		r2 = r2.Delete(IBKey(fmt.Sprintf("%d", i)))
	}
	r3 := r
	for i := n - 1; i > 0; i-- {
		if debug > 0 {
			checkTree2(t, r3, i+1, 0)
		}
		if debug > 0 {
			log.Printf("Deleting #%d\n", i)
		}
		r3 = r3.Delete(IBKey(fmt.Sprintf("%d", i)))
	}
}
