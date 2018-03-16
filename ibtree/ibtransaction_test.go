/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2018 Markus Stenberg
 *
 * Created:       Tue Jan  9 18:29:10 2018 mstenber
 * Last modified: Fri Mar 16 13:57:15 2018 mstenber
 * Edit time:     34 min
 *
 */

package ibtree_test

import (
	"fmt"
	"math/rand"
	"sort"
	"testing"

	"github.com/fingon/go-tfhfs/ibtree"
	"github.com/fingon/go-tfhfs/mlog"
	"github.com/fingon/go-tfhfs/util"
	"github.com/stvp/assert"
)

func ProdTree(t *testing.T, rng *rand.Rand) {
	be := ibtree.DummyBackend{}.Init()
	tree := ibtree.Tree{}.Init(be)
	root := tree.NewRoot()
	iter := 1000

	key2value := make(map[string]string)
	keys := make([]string, 0)

	tr := ibtree.NewTransaction(root)
	randomKey := func() string {
		n := len(keys)
		if n > 0 {
			i := rng.Int() % n
			return keys[i]
		}
		return ""
	}
	check := func() {
		troot := tr.Root()
		mlog.Printf2("ibtree/ibtransaction_test", ".. check sanity in %v..", troot)
		troot.PrintToMLogAll()
		for k, v := range key2value {
			vp := tr.Get(ibtree.Key(k))
			assert.True(t, vp != nil, fmt.Sprintf("missing key %x", k))
			assert.Equal(t, v, *vp)
		}
	}
	for i := 0; i < iter; i++ {
		p := rng.Int() % 100
		if p < 50 {
			// mostly grow the tree
			b := make([]byte, 16)
			rng.Read(b)
			key := string(b)
			value := fmt.Sprintf("%v", i)
			mlog.Printf2("ibtree/ibtransaction_test", "PT #%d: Set %x=%v", i, key, value)
			tr.Set(ibtree.Key(key), value)
			if key2value[key] == "" {
				keys = append(keys, key)
				sort.Strings(keys)
			}
			key2value[key] = value
			check()
			continue
		} else if p < 90 {
			// delete
			key := randomKey()
			if key != "" {
				mlog.Printf2("ibtree/ibtransaction_test", "PT #%d: Delete %x", i, key)
				idx := sort.SearchStrings(keys, key)
				if len(keys) != (idx - 1) {
					keys[idx] = keys[len(keys)-1]
				}
				keys = keys[:len(keys)-1]
				sort.Strings(keys)
				tr.Delete(ibtree.Key(key))
				delete(key2value, key)
			}
			check()
		} else {
			mlog.Printf2("ibtree/ibtransaction_test", "PT #%d: Commit", i)
			// commit transaction
			nroot, _ := tr.Commit()
			root = nroot
			tr = ibtree.NewTransaction(root)
		}
	}
}

func TestTransactionRandomTree(t *testing.T) {
	t.Parallel()

	rng := util.GetSeededRng()
	ProdTree(t, rng)
}
