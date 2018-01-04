/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2017 Markus Stenberg
 *
 * Created:       Thu Dec 28 14:31:48 2017 mstenber
 * Last modified: Thu Jan  4 13:28:23 2018 mstenber
 * Edit time:     20 min
 *
 */

package fs

import (
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"testing"

	"github.com/fingon/go-tfhfs/ibtree"
	"github.com/fingon/go-tfhfs/storage"
	"github.com/stvp/assert"
)

func TestblockKey(t *testing.T) {
	t.Parallel()
	ino := uint64(42)
	bst := BST_META
	bstd := "foo"
	k := NewblockKey(ino, bst, bstd)
	assert.Equal(t, k.Ino(), ino)
	assert.Equal(t, k.SubType(), bst)
	assert.Equal(t, k.SubTypeData(), bstd)
}

func BenchmarkBadgerFs(b *testing.B) {
	dir, _ := ioutil.TempDir("", "badger")
	defer os.RemoveAll(dir)

	// Add some items we can access/delete/set
	n := 100000
	backend := storage.BadgerBlockBackend{}.Init(dir)
	st := NewCryptoStorage("assword", "alt", backend)
	fs := NewFs(st, "toor")

	tr := fs.GetTransaction()
	for i := 0; i < n; i++ {
		k := ibtree.IBKey(NewblockKey(uint64(i), BST_META, ""))
		tr.Set(k, fmt.Sprintf("v%d", i))
	}
	fs.CommitTransaction(tr)

	b.Run("Get1", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			tr := fs.GetTransaction()
			j := rand.Int() % n
			k := ibtree.IBKey(NewblockKey(uint64(j), BST_META, ""))
			tr.Get(k)
		}
	})

	b.Run("GetN", func(b *testing.B) {
		tr := fs.GetTransaction()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			j := rand.Int() % n
			k := ibtree.IBKey(NewblockKey(uint64(j), BST_META, ""))
			tr.Get(k)
		}
	})

	b.Run("Set", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			tr := fs.GetTransaction()
			j := rand.Int() % n
			k := ibtree.IBKey(NewblockKey(uint64(j), BST_META, ""))
			tr.Set(k, fmt.Sprintf("V%d%d", j, i))
			tr.Commit()
		}
	})

	b.Run("Delete", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			tr := ibtree.NewTransaction(fs.treeRoot.Get())
			j := rand.Int() % n
			k := ibtree.IBKey(NewblockKey(uint64(j), BST_META, ""))
			tr.Delete(k)
			tr.Commit()
		}
	})

}
