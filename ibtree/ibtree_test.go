/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2017 Markus Stenberg
 *
 * Created:       Mon Dec 25 17:07:23 2017 mstenber
 * Last modified: Thu Dec 28 03:12:09 2017 mstenber
 * Edit time:     148 min
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

type N2IBKeyCallback func(n int) IBKey

type DummyTree struct {
	IBTree
	idcb N2IBKeyCallback
}

func (self DummyTree) Init(be *DummyBackend) *DummyTree {
	self.IBTree = *(self.IBTree.Init(be))
	if self.idcb == nil {
		self.idcb = nonpaddedIBKey
	}
	return &self
}

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

const debug = 1
const nodeSize = 256

func (self *DummyTree) checkTree2(t *testing.T, r *IBNode, n int, st int) {
	if debug > 1 {
		r.print(0)
		log.Printf("checkTree [%d..%d[\n", st, n)
	}
	for i := st - 1; i <= n; i++ {
		if debug > 1 {
			log.Printf(" #%d\n", i)
		}
		s := self.idcb(i)
		v := r.Get(s)
		if i == (st-1) || i == n {
			assert.Nil(t, v)
		} else {
			assert.True(t, v != nil)
			assert.Equal(t, *v, fmt.Sprintf("%d", i))
		}
	}
	r.checkTreeStructure()
}

func (self *DummyTree) checkTree(t *testing.T, r *IBNode, n int) {
	self.checkTree2(t, r, n, 0)
}

func nonpaddedIBKey(n int) IBKey {
	return IBKey(fmt.Sprintf("%d", n))
}

func paddedIBKey(n int) IBKey {
	return IBKey(fmt.Sprintf("%08d", n))
}

func (self *DummyTree) CreateIBTree(t *testing.T, n int) *IBNode {
	r := self.NewRoot()
	v := r.Get(IBKey("foo"))
	assert.Nil(t, v)
	for i := 0; i < n; i++ {
		if debug > 1 {
			self.checkTree(t, r, i) // previous tree should be ok
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
		k := self.idcb(i)
		v := fmt.Sprintf("%d", i)
		r = r.Set(k, v)
	}
	return r
}

func EmptyIBTreeForward(t *testing.T, dt *DummyTree, r *IBNode, n int) *IBNode {
	for i := 0; i < n; i++ {
		if debug > 1 {
			dt.checkTree2(t, r, n, i)
		}
		if debug > 0 {
			log.Printf("Deleting #%d\n", i)
		}
		k := dt.idcb(i)
		r = r.Delete(k)
	}
	return r
}

func EmptyIBTreeBackward(t *testing.T, dt *DummyTree, r *IBNode, n int) *IBNode {
	for i := n - 1; i > 0; i-- {
		if debug > 1 {
			dt.checkTree2(t, r, i+1, 0)
		}
		if debug > 0 {
			log.Printf("Deleting #%d\n", i)
		}
		k := dt.idcb(i)
		r = r.Delete(IBKey(k))
	}
	return r
}

func ProdIBTree(t *testing.T, tree *DummyTree, n int) {
	r := tree.CreateIBTree(t, n)
	// Check forward and backwards iteration
	var st ibStack
	st.nodes[0] = r
	st.indexes[0] = -1
	c1 := 0
	for st.goNextLeaf() {
		c1++
	}
	assert.Equal(t, c1, n+1)
	c2 := 0
	for st.goPreviousLeaf() {
		c2++
	}
	assert.Equal(t, c1, c2, "next count = previous count")

	// Ensure in-place mutate works fine as well and does not change r
	rr := r.Set(IBKey("0"), "z")
	assert.Equal(t, "z", *rr.Get(IBKey("0")))
	assert.Equal(t, "0", *r.Get(IBKey("0")))
	tree.checkTree(t, r, n)
	EmptyIBTreeForward(t, tree, r, n)
	EmptyIBTreeBackward(t, tree, r, n)
}

func TestIBTree(t *testing.T) {
	tree := DummyTree{}.Init(nil)
	tree.setNodeMaximumSize(nodeSize) // more depth = smaller examples that blow up
	ProdIBTree(t, tree, 10000)
}

func TestIBTreeDeleteRange(t *testing.T) {
	tree := DummyTree{idcb: paddedIBKey}.Init(nil)
	n := 1000
	r := tree.CreateIBTree(t, n)
	log.Printf("TestIBTreeDeleteRange start")
	r1 := r.DeleteRange(paddedIBKey(-1), paddedIBKey(-1))
	assert.Equal(t, r1, r)
	r2 := r.DeleteRange(IBKey("z"), IBKey("z"))
	assert.Equal(t, r2, r)

	// We attempt to remove higher bits, as they offend us.
	for i := 4; i < n; i = i * 4 {
		i0 := i * 3 / 4
		r.checkTreeStructure()
		if debug > 0 {
			r.print(0)
			log.Printf("DeleteRange %d-%d\n", i0, i)
		}
		s1 := tree.idcb(i0)
		s2 := tree.idcb(i)
		r = r.DeleteRange(s1, s2)

		r.checkTreeStructure()

		for j := 4; j <= i; j = j * 4 {
			j0 := j*3/4 - 1
			s0 := tree.idcb(j0)
			r0 := r.Get(s0)
			if debug > 0 {
				log.Printf("Checking %d-%d\n", j0, j)

			}
			if j0 < n {
				assert.True(t, r0 != nil, "missing index:", j0)
				assert.Equal(t, *r0, fmt.Sprintf("%d", j0))
			} else {
				assert.Nil(t, r0)
			}
			s1 := tree.idcb(j * 3 / 4)
			r1 := r.Get(s1)
			assert.Nil(t, r1)

			s2 := tree.idcb(j)
			r2 := r.Get(s2)
			assert.Nil(t, r2)

			j3 := j + 1
			s3 := tree.idcb(j3)
			r3 := r.Get(s3)
			if j3 < n {
				assert.True(t, r3 != nil, "missing index:", j3)
				assert.Equal(t, *r3, fmt.Sprintf("%d", j3))
			} else {
				assert.Nil(t, r3)
			}

		}
	}
}

func TestIBTreeStorage(t *testing.T) {
	n := 1000
	be := DummyBackend{}.Init()
	tree := DummyTree{}.Init(be)
	r := tree.CreateIBTree(t, n).Commit()
	c1 := r.nestedNodeCount()
	assert.True(t, r.blockId != nil)
	assert.Equal(t, be.loads, 0)
	r = tree.LoadRoot(*r.blockId)
	assert.Equal(t, be.loads, 1)
	tree.checkTree(t, r, n)
	c2 := r.nestedNodeCount()
	assert.True(t, c1 != c2)
	assert.Equal(t, c2, 1)
	//assert.Equal(t, c2, be.loads)
	// loads is 'shotload', checkTree does .. plenty.
	os := be.saves
	r = r.Commit()
	assert.Equal(t, os, be.saves)
}
