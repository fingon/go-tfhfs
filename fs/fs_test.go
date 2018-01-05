/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2017 Markus Stenberg
 *
 * Created:       Thu Dec 28 14:31:48 2017 mstenber
 * Last modified: Fri Jan  5 23:32:08 2018 mstenber
 * Edit time:     31 min
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
	"github.com/fingon/go-tfhfs/storage/factory"
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

func TestFsTransaction(t *testing.T) {
	t.Parallel()

	rootName := "toor"
	backend := factory.New("inmemory", "")
	st := storage.Storage{Backend: backend}.Init()
	fs := NewFs(st, rootName)
	defer fs.lock.Locked()()

	// simulate 3 parallel operations

	tr1 := newFsTransaction(fs)
	tr1.t.Set("foo1", "v1")

	tr2 := newFsTransaction(fs)
	tr2.t.Set("foo2", "v2")

	tr3 := newFsTransaction(fs)
	tr3.t.Set("foo3", "v3")

	tr1.Commit()
	tr2.Commit()
	tr3.Commit()

	tr1 = newFsTransaction(fs)
	tr2 = newFsTransaction(fs)
	tr3 = newFsTransaction(fs)
	assert.Equal(t, *tr1.t.Get("foo1"), "v1")
	assert.Equal(t, *tr1.t.Get("foo2"), "v2")
	assert.Equal(t, *tr1.t.Get("foo3"), "v3")

	// Now tr1 updates, tr2 deletes one key, and second key vice versa
	tr1.t.Set("foo1", "v11")
	tr1.t.Delete("foo2")
	tr2.t.Delete("foo1")
	tr2.t.Set("foo2", "v21")
	tr1.Commit()
	tr2.Commit()

	// Most recent write wins in this case -> should have what tr2 did
	tr1 = newFsTransaction(fs)
	assert.Nil(t, tr1.t.Get("foo1"))
	assert.Equal(t, *tr1.t.Get("foo2"), "v21")
}

func BenchmarkBadgerFs(b *testing.B) {
	bename := "badger"
	dir, _ := ioutil.TempDir("", bename)
	defer os.RemoveAll(dir)

	// Add some items we can access/delete/set
	n := 100000
	backend := factory.New(bename, dir)
	st := NewCryptoStorage("assword", "alt", backend)
	fs := NewFs(st, "toor")

	tr := fs.GetTransaction()
	for i := 0; i < n; i++ {
		k := ibtree.IBKey(NewblockKey(uint64(i), BST_META, ""))
		tr.t.Set(k, fmt.Sprintf("v%d", i))
	}
	tr.Commit()

	b.Run("Get1", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			tr := fs.GetTransaction()
			j := rand.Int() % n
			k := ibtree.IBKey(NewblockKey(uint64(j), BST_META, ""))
			tr.t.Get(k)
			tr.Close()
		}
	})

	b.Run("GetN", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			tr := fs.GetTransaction()
			j := rand.Int() % n
			k := ibtree.IBKey(NewblockKey(uint64(j), BST_META, ""))
			tr.t.Get(k)
			tr.Close()
		}
	})

	b.Run("Set", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			tr := fs.GetTransaction()
			j := rand.Int() % n
			k := ibtree.IBKey(NewblockKey(uint64(j), BST_META, ""))
			tr.t.Set(k, fmt.Sprintf("V%d%d", j, i))
			tr.Commit()
		}
	})

	b.Run("Delete", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			tr := fs.GetTransaction()
			j := rand.Int() % n
			k := ibtree.IBKey(NewblockKey(uint64(j), BST_META, ""))
			tr.t.Delete(k)
			tr.Commit()
		}
	})

}
