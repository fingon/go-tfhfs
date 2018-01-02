/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2017 Markus Stenberg
 *
 * Created:       Mon Dec 25 17:07:23 2017 mstenber
 * Last modified: Tue Jan  2 20:35:35 2018 mstenber
 * Edit time:     252 min
 *
 */

package ibtree

import (
	"crypto/sha256"
	"fmt"
	"log"
	"testing"

	"github.com/fingon/go-tfhfs/mlog"
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

func (self *DummyTree) checkNextKey(t *testing.T, r *IBNode, n int) {
	// check NextKey works
	lkv := IBKey("")
	lk := &lkv
	cnt := 0
	r.iterateLeafFirst(func(n *IBNode) {
		if !n.Leafy {
			return
		}
		for _, c := range n.Children {
			cnt++
			nk := r.NextKey(*lk, &IBStack{})
			assert.Equal(t, *nk, c.Key)
			lk = nk
		}

	})
	assert.Equal(t, cnt, n)
	nk := r.NextKey(*lk, &IBStack{})
	assert.True(t, *lk != "", "did not even get anything?")
	assert.Nil(t, nk, "weird next for", *lk, nk)
}

func (self *DummyTree) checkTree2(t *testing.T, r *IBNode, n, s int) {
	if debug > 1 {
		r.PrintToMLogDirty()
		mlog.Printf2("ibtree/ibtree_test", "checkTree [%d..%d[\n", s, n)
	}
	for i := s - 2; i <= n+1; i++ {
		var st IBStack
		if debug > 1 {
			mlog.Printf2("ibtree/ibtree_test", " #%d\n", i)
		}
		ss := self.idcb(i)
		v := r.Get(ss, &st)
		if i < s || i >= n {
			assert.Nil(t, v)
		} else {
			assert.True(t, v != nil, "missing key ", ss)
			assert.Equal(t, *v, fmt.Sprintf("v%d", i))
		}
	}
	r.checkTreeStructure()

}

func (self *DummyTree) checkTree(t *testing.T, r *IBNode, n int) {
	self.checkTree2(t, r, n, 0)
}

func nonpaddedIBKey(n int) IBKey {
	return IBKey(fmt.Sprintf("nk%d", n))
}

func paddedIBKey(n int) IBKey {
	return IBKey(fmt.Sprintf("pk%08d", n))
}

func EnsureDelta(t *testing.T, old, new *IBNode, del, upd, add int) {
	var got_upd, got_add, got_del int
	var previous *IBNodeDataChild
	new.IterateDelta(old, func(c1, c2 *IBNodeDataChild) {
		mlog.Printf2("ibtree/ibtree_test", "c0:%v c:%v", c1, c2)
		c := c1
		if c1 == nil {
			c = c2
			got_add++
		} else if c2 == nil {
			got_del++
		} else {
			assert.Equal(t, c1.Key, c2.Key)
			got_upd++
		}
		if previous != nil {
			assert.True(t, previous.Key < c.Key, "broken iteration at key", previous, c)
		}
		previous = c

	})
	assert.Equal(t, got_add, add)
	assert.Equal(t, got_upd, upd)
	assert.Equal(t, got_del, del)
}

func EnsureDelta2(t *testing.T, old, new *IBNode, del, upd, add int) {
	EnsureDelta(t, old, new, del, upd, add)
	EnsureDelta(t, new, old, add, upd, del)
}
func (self *DummyTree) CreateIBTree(t *testing.T, n int) *IBNode {
	var st IBStack
	r0 := self.NewRoot()
	r := r0
	v := r0.Get(IBKey("foo"), &st)
	assert.Nil(t, v)
	for i := 0; i < n; i++ {
		if debug > 1 {
			self.checkTree(t, r, i) // previous tree should be ok
			mlog.Printf2("ibtree/ibtree_test", "Inserting #%d\n", i)
		}
		cnt := 0
		if r != st.nodes[0] {
			log.Panic("asdf1", r, st.nodes[0])
		}
		r = st.iterateMutatingChildLeafFirst(func() {
			if debug > 1 {
				mlog.Printf2("ibtree/ibtree_test", "(iterating) Child %v %v", st.indexes, st.child())
			}
			cnt++
		})
		assert.Equal(t, cnt, r.nestedNodeCount()-1, "iterateMutatingChildLeafFirst broken")
		k := self.idcb(i)
		v := fmt.Sprintf("v%d", i)
		if r != st.nodes[0] {
			log.Panic("asdf2", r, st.nodes[0])
		}
		r = r.Set(k, v, &st)
		if r != st.nodes[0] {
			log.Panic("asdf3", r, st.nodes[0])
		}
	}
	//EnsureDelta2(t, r0, r, 0, 0, n)
	//r2 := r0.Set(IBKey("z"), "42")
	//EnsureDelta2(t, r2, r, 1, 0, n)

	return r
}

func EmptyIBTreeForward(t *testing.T, dt *DummyTree, r *IBNode, n int) *IBNode {
	var st IBStack
	for i := 0; i < n; i++ {
		if debug > 1 {
			dt.checkTree2(t, r, n, i)
		}
		if debug > 1 {
			mlog.Printf2("ibtree/ibtree_test", "Deleting #%d\n", i)
		}
		k := dt.idcb(i)
		r = r.Delete(k, &st)
	}
	return r
}

func EmptyIBTreeBackward(t *testing.T, dt *DummyTree, r *IBNode, n int) *IBNode {
	var st IBStack
	for i := n - 1; i > 0; i-- {
		if debug > 2 {
			dt.checkTree2(t, r, i+1, 0)
		}
		if debug > 1 {
			mlog.Printf2("ibtree/ibtree_test", "Deleting #%d\n", i)
		}
		k := dt.idcb(i)
		r = r.Delete(IBKey(k), &st)
	}
	return r
}

func ProdIBTree(t *testing.T, tree *DummyTree, n int) *IBNode {
	r := tree.CreateIBTree(t, n)
	// Check forward and backwards iteration
	var st IBStack
	st.nodes[0] = r
	st.setIndex(-1)
	c1 := 0

	st2 := st
	st2.goNextLeaf()

	st3 := st
	st3.Reset()
	st3.goDownLeftAny()
	assert.Equal(t, st2, st3)

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
	k := tree.idcb(0)
	rr := r.Set(k, "z", &st)
	assert.Equal(t, "z", *rr.Get(k, &st))
	assert.Equal(t, "v0", *r.Get(k, &IBStack{}))
	tree.checkTree(t, r, n)
	EmptyIBTreeForward(t, tree, r, n)
	EmptyIBTreeBackward(t, tree, r, n)
	return r
}

func TestIBTree(t *testing.T) {
	n := 10000
	t.Parallel()
	tree := DummyTree{}.Init(nil)
	tree.setNodeMaximumSize(nodeSize) // more depth = smaller examples that blow up
	r := ProdIBTree(t, tree, n)
	tree.checkNextKey(t, r, n)
}

func TestIBTreeDeleteRange(t *testing.T) {
	t.Parallel()
	tree := DummyTree{idcb: paddedIBKey}.Init(nil)
	n := 1000
	r := tree.CreateIBTree(t, n)
	mlog.Printf2("ibtree/ibtree_test", "TestIBTreeDeleteRange start")
	r1 := r.DeleteRange(paddedIBKey(-1), paddedIBKey(-1), &IBStack{})
	assert.Equal(t, r1, r)
	r2 := r.DeleteRange(IBKey("z"), IBKey("z"), &IBStack{})
	assert.Equal(t, r2, r)

	// We attempt to remove higher bits, as they offend us.
	for i := 4; i < n; i = i * 4 {
		var st IBStack
		i0 := i * 3 / 4
		removed := i - i0 + 1
		r.checkTreeStructure()
		if debug > 1 {
			r.PrintToMLogDirty()
			mlog.Printf2("ibtree/ibtree_test", "DeleteRange %d-%d\n", i0, i)
		}
		s1 := tree.idcb(i0)
		s2 := tree.idcb(i)
		or := r
		r = r.DeleteRange(s1, s2, &st)
		if debug > 1 {
			r.PrintToMLogDirty()
		}
		EnsureDelta2(t, or, r, removed, 0, 0)

		r.checkTreeStructure()

		for j := 4; j <= i; j = j * 4 {
			j0 := j*3/4 - 1
			s0 := tree.idcb(j0)
			r0 := r.Get(s0, &st)
			if debug > 0 {
				mlog.Printf2("ibtree/ibtree_test", "Checking %d-%d\n", j0, j)

			}
			if j0 < n {
				assert.True(t, r0 != nil, "missing index:", j0)
				assert.Equal(t, *r0, fmt.Sprintf("v%d", j0))
			} else {
				assert.Nil(t, r0)
			}
			s1 := tree.idcb(j * 3 / 4)
			r1 := r.Get(s1, &st)
			assert.Nil(t, r1)

			s2 := tree.idcb(j)
			r2 := r.Get(s2, &st)
			assert.Nil(t, r2)

			j3 := j + 1
			s3 := tree.idcb(j3)
			r3 := r.Get(s3, &st)
			if j3 < n {
				assert.True(t, r3 != nil, "missing index:", j3)
				assert.Equal(t, *r3, fmt.Sprintf("v%d", j3))
			} else {
				assert.Nil(t, r3)
			}

		}
	}
}

func TestIBTreeStorage(t *testing.T) {
	t.Parallel()
	n := 1000
	be := DummyBackend{}.Init()
	tree := DummyTree{}.Init(be)
	r, bid := tree.CreateIBTree(t, n).Commit()
	assert.True(t, string(bid) != "")
	c1 := r.nestedNodeCount()
	assert.True(t, r.blockId != nil)
	assert.Equal(t, be.loads, 0)
	r = tree.LoadRoot(*r.blockId)
	assert.Equal(t, be.loads, 1)
	tree.checkTree(t, r, n)
	c2 := r.nestedNodeCount()
	assert.Equal(t, c1, c2)
	//assert.Equal(t, c2, be.loads)
	// loads is 'shotload', checkTree does .. plenty.
	os := be.saves
	r, bid = r.Commit()
	assert.True(t, string(bid) != "")
	assert.Equal(t, os, be.saves)
}

func TestIBTransaction(t *testing.T) {
	t.Parallel()
	n := 100
	be := DummyBackend{}.Init()
	tree := DummyTree{}.Init(be)
	r, bid := tree.CreateIBTree(t, n).Commit()
	assert.True(t, string(bid) != "")
	tr := NewTransaction(r)
	r2, bid2 := tr.Commit()
	assert.Equal(t, r, r2)
	assert.Equal(t, bid, bid2)
	k := tree.idcb(42)
	assert.Equal(t, *tr.Get(k), "v42")
	tr.Delete(k)
	assert.Nil(t, tr.Get(k))
	tr.DeleteRange(tree.idcb(7), tree.idcb(9))
	assert.Nil(t, tr.Get(tree.idcb(7)))
	assert.Nil(t, tr.Get(tree.idcb(9)))
	assert.True(t, tr.Get(tree.idcb(6)) != nil)
	assert.True(t, tr.Get(tree.idcb(10)) != nil)
	tr.Set(k, "42")
	assert.Equal(t, *tr.Get(k), "42")

}
