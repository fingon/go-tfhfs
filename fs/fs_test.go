/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2017 Markus Stenberg
 *
 * Created:       Thu Dec 28 14:31:48 2017 mstenber
 * Last modified: Wed Jan 17 11:28:25 2018 mstenber
 * Edit time:     34 min
 *
 */

package fs

import (
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"testing"

	"github.com/fingon/go-tfhfs/storage"
	"github.com/fingon/go-tfhfs/storage/factory"
	"github.com/stvp/assert"
)

func TestBlockKey(t *testing.T) {
	t.Parallel()
	ino := uint64(42)
	bst := BST_META
	bstd := "foo"
	k := NewBlockKey(ino, bst, bstd)
	assert.Equal(t, k.Ino(), ino)
	assert.Equal(t, k.SubType(), bst)
	assert.Equal(t, k.SubTypeData(), bstd)
}

func TestfsTransaction(t *testing.T) {
	t.Parallel()

	RootName := "toor"
	backend := factory.New("inmemory", "")
	st := storage.Storage{Backend: backend}.Init()
	fs := NewFs(st, RootName, 0)
	defer fs.Close()

	st.IterateReferencesCallback = nil

	// simulate 3 parallel operations

	tr1 := newfsTransaction(fs)
	tr1.t.Set("foo1", "v1")

	tr2 := newfsTransaction(fs)
	tr2.t.Set("foo2", "v2")

	tr3 := newfsTransaction(fs)
	tr3.t.Set("foo3", "v3")

	tr1.CommitUntilSucceeds()
	tr2.CommitUntilSucceeds()
	tr3.CommitUntilSucceeds()

	tr1 = newfsTransaction(fs)
	tr2 = newfsTransaction(fs)
	tr3 = newfsTransaction(fs)
	assert.Equal(t, *tr1.t.Get("foo1"), "v1")
	assert.Equal(t, *tr1.t.Get("foo2"), "v2")
	assert.Equal(t, *tr1.t.Get("foo3"), "v3")

	// Now tr1 updates, tr2 deletes one key, and second key vice versa
	tr1.t.Set("foo1", "v11")
	tr1.t.Delete("foo2")
	tr2.t.Delete("foo1")
	tr2.t.Set("foo2", "v21")
	tr1.CommitUntilSucceeds()
	tr2.CommitUntilSucceeds()

	// Most recent write wins in this case -> should have what tr2 did
	tr1 = newfsTransaction(fs)
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
	fs := NewFs(st, "toor", 0)
	defer fs.Close()

	tr := fs.GetTransaction()
	for i := 0; i < n; i++ {
		k := NewBlockKey(uint64(i), BST_META, "").IB()
		tr.t.Set(k, fmt.Sprintf("v%d", i))
	}
	tr.CommitUntilSucceeds()

	b.Run("Get1", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			tr := fs.GetTransaction()
			j := rand.Int() % n
			k := NewBlockKey(uint64(j), BST_META, "").IB()
			tr.t.Get(k)
			tr.Close()
		}
	})

	b.Run("GetN", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			tr := fs.GetTransaction()
			j := rand.Int() % n
			k := NewBlockKey(uint64(j), BST_META, "").IB()
			tr.t.Get(k)
			tr.Close()
		}
	})

	b.Run("Set", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			tr := fs.GetTransaction()
			j := rand.Int() % n
			k := NewBlockKey(uint64(j), BST_META, "").IB()
			tr.t.Set(k, fmt.Sprintf("V%d%d", j, i))
			tr.CommitUntilSucceeds()
		}
	})

	b.Run("Delete", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			tr := fs.GetTransaction()
			j := rand.Int() % n
			k := NewBlockKey(uint64(j), BST_META, "").IB()
			tr.t.Delete(k)
			tr.CommitUntilSucceeds()
		}
	})

}
