/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2017 Markus Stenberg
 *
 * Created:       Mon Dec 25 17:07:23 2017 mstenber
 * Last modified: Wed Dec 27 18:05:06 2017 mstenber
 * Edit time:     80 min
 *
 */

package ibtree

import (
	"crypto/sha256"
	"fmt"
	"log"
	"testing"

	"github.com/stvp/assert"
)

type DummyBackend struct {
	h2nd  map[BlockId]*IBNodeData
	loads int
}

func (self DummyBackend) Init() *DummyBackend {
	self.h2nd = make(map[BlockId]*IBNodeData)
	return &self
}

func (self *DummyBackend) LoadNode(id BlockId) *IBNodeData {
	self.loads++
	// Create new copy of IBNodeData WITHOUT childNode's set
	ond := self.h2nd[id]

	nd := &IBNodeData{Leafy: ond.Leafy}
	for _, v := range ond.Children {
		nd.Children = append(nd.Children, &IBNodeDataChild{Key: v.Key, Value: v.Value})
	}
	return nd
}

func (self *DummyBackend) SaveNode(nd IBNodeData) BlockId {
	b, _ := nd.MarshalMsg(nil)
	h := sha256.Sum256(b)
	bid := BlockId(h[:])
	self.h2nd[bid] = &nd
	return bid
}

const debug = 0
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

func ProdIBTree(t *testing.T, r *IBNode, n int, commit int) *IBNode {
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
		assert.Equal(t, cnt, r.nestedNodeCount()-1, "iterateMutatingChildLeafFirst broken")
		s := fmt.Sprintf("%d", i)
		r = r.Set(IBKey(s), s)
	}

	rv := r
	if commit == 1 {
		r = r.Commit()
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
	if commit == 2 {
		rv = r2.Commit()

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
	return rv
}

func TestIBTree(t *testing.T) {
	tree := IBTree{}.Init(nil)
	tree.setNodeMaximumSize(nodeSize) // more depth = smaller examples that blow up
	r := tree.NewRoot()
	ProdIBTree(t, r, 10000, 0)
}

func TestIBTreeStorage(t *testing.T) {
	n := 1000
	be := DummyBackend{}.Init()
	tree := IBTree{}.Init(be)
	r := tree.NewRoot()
	r = ProdIBTree(t, r, n, 1)
	c1 := r.nestedNodeCount()
	assert.True(t, r.blockId != nil)
	assert.Equal(t, be.loads, 0)
	r = tree.LoadRoot(*r.blockId)
	assert.Equal(t, be.loads, 1)
	checkTree(t, r, n)
	c2 := r.nestedNodeCount()
	assert.True(t, c1 != c2)
	assert.Equal(t, c2, 1)
	//assert.Equal(t, c2, be.loads)
	// loads is 'shotload', checkTree does .. plenty.
}
