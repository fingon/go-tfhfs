/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2017 Markus Stenberg
 *
 * Created:       Mon Dec 25 17:07:23 2017 mstenber
 * Last modified: Wed Dec 27 19:00:16 2017 mstenber
 * Edit time:     112 min
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
	h2nd  map[BlockId][]byte
	loads int
	saves int
}

func (self DummyBackend) Init() *DummyBackend {
	self.h2nd = make(map[BlockId][]byte)
	return &self
}

func (self *DummyBackend) LoadNode(id BlockId) *IBNodeData {
	self.loads++
	// Create new copy of IBNodeData WITHOUT childNode's set
	nd := &IBNodeData{}
	_, err := nd.UnmarshalMsg(self.h2nd[id])
	if err != nil {
		log.Panic(err)
	}
	return nd
}

func (self *DummyBackend) SaveNode(nd IBNodeData) BlockId {
	b, _ := nd.MarshalMsg(nil)
	h := sha256.Sum256(b)
	bid := BlockId(h[:])
	self.h2nd[bid] = b
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

func CreateIBTree(t *testing.T, tree *IBTree, n int) *IBNode {
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
		assert.Equal(t, cnt, r.nestedNodeCount()-1, "iterateMutatingChildLeafFirst broken")
		s := fmt.Sprintf("%d", i)
		r = r.Set(IBKey(s), s)
	}
	return r
}

func EmptyIBTreeForward(t *testing.T, r *IBNode, n int) *IBNode {
	for i := 0; i < n; i++ {
		if debug > 0 {
			checkTree2(t, r, n, i)
		}
		if debug > 0 {
			log.Printf("Deleting #%d\n", i)
		}
		r = r.Delete(IBKey(fmt.Sprintf("%d", i)))
	}
	return r
}

func EmptyIBTreeBackward(t *testing.T, r *IBNode, n int) *IBNode {
	for i := n - 1; i > 0; i-- {
		if debug > 0 {
			checkTree2(t, r, i+1, 0)
		}
		if debug > 0 {
			log.Printf("Deleting #%d\n", i)
		}
		r = r.Delete(IBKey(fmt.Sprintf("%d", i)))
	}
	return r
}

func ProdIBTree(t *testing.T, tree *IBTree, n int) {
	r := CreateIBTree(t, tree, n)
	// Ensure in-place mutate works fine as well and does not change r
	rr := r.Set(IBKey("0"), "z")
	assert.Equal(t, "z", *rr.Get(IBKey("0")))
	assert.Equal(t, "0", *r.Get(IBKey("0")))
	checkTree(t, r, n)
	EmptyIBTreeForward(t, r, n)
	EmptyIBTreeBackward(t, r, n)
}

func TestIBTree(t *testing.T) {
	tree := IBTree{}.Init(nil)
	tree.setNodeMaximumSize(nodeSize) // more depth = smaller examples that blow up
	ProdIBTree(t, tree, 10000)
}

func TBDTestIBTreeDeleteRange(t *testing.T) {
	n := 1000
	tree := IBTree{}.Init(nil)
	r := CreateIBTree(t, tree, n)
	// We attempt to remove higher bits, as they offend us.
	for i := 4; i < n; i = i * 4 {
		i0 := i * 3 / 4
		r.checkTreeStructure()
		if debug > 0 {
			log.Printf("DeleteRange %d-%d\n", i0, i)
		}
		s1 := IBKey(fmt.Sprintf("%d", i0))
		s2 := IBKey(fmt.Sprintf("%d", i))
		r = r.DeleteRange(s1, s2)
	}
	for i := 4; i < n*4; i = i * 4 {
		i0 := i*3/4 - 1
		s0 := fmt.Sprintf("%d", i0)
		r0 := r.Get(IBKey(s0))
		if debug > 0 {
			log.Printf("Checking %d-%d\n", i0, i)

		}
		if i0 < n {
			assert.Equal(t, r0, s0)
		} else {
			assert.Nil(t, r0)
		}
		s1 := IBKey(fmt.Sprintf("%d", i*3/4))
		r1 := r.Get(IBKey(s1))
		assert.Nil(t, r1)

		s2 := IBKey(fmt.Sprintf("%d", i))
		r2 := r.Get(IBKey(s2))
		assert.Nil(t, r2)

		i3 := i + 1
		s3 := IBKey(fmt.Sprintf("%d", i3))
		r3 := r.Get(IBKey(s3))
		if i3 < n {
			assert.Equal(t, r3, s3)
		} else {
			assert.Nil(t, r3)
		}

	}
}

func TestIBTreeStorage(t *testing.T) {
	n := 1000
	be := DummyBackend{}.Init()
	tree := IBTree{}.Init(be)
	r := CreateIBTree(t, tree, n).Commit()
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
	os := be.saves
	r = r.Commit()
	assert.Equal(t, os, be.saves)
}
