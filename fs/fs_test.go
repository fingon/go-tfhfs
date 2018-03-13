/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2017 Markus Stenberg
 *
 * Created:       Thu Dec 28 14:31:48 2017 mstenber
 * Last modified: Tue Mar 13 12:40:05 2018 mstenber
 * Edit time:     44 min
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

func TestTransaction(t *testing.T) {
	t.Parallel()

	RootName := "toor"
	backend := factory.New("inmemory", "")
	st := storage.Storage{Backend: backend}.Init()
	fs := NewFs(st, RootName, 0)
	defer fs.closeWithoutTransactions()

	st.IterateReferencesCallback = nil

	// simulate 3 parallel operations

	tr1 := fs.GetTransaction()
	tr1.IB().Set("foo1", "v1")

	tr2 := fs.GetTransaction()
	tr2.IB().Set("foo2", "v2")

	tr3 := fs.GetTransaction()
	tr3.IB().Set("foo3", "v3")

	tr1.CommitUntilSucceeds()
	tr2.CommitUntilSucceeds()
	tr3.CommitUntilSucceeds()

	tr1 = fs.GetTransaction()
	tr2 = fs.GetTransaction()
	tr3 = fs.GetTransaction()
	defer tr3.Close()
	assert.Equal(t, *tr1.IB().Get("foo1"), "v1")
	assert.Equal(t, *tr1.IB().Get("foo2"), "v2")
	assert.Equal(t, *tr1.IB().Get("foo3"), "v3")

	// Now tr1 updates, tr2 deletes one key, and second key vice versa
	tr1.IB().Set("foo1", "v11")
	tr1.IB().Delete("foo2")
	tr2.IB().Delete("foo1")
	tr2.IB().Set("foo2", "v21")
	tr1.CommitUntilSucceeds()
	tr2.CommitUntilSucceeds()

	// Most recent write wins in this case -> should have what tr2 did
	tr1 = fs.GetTransaction()
	defer tr1.Close()
	assert.Nil(t, tr1.IB().Get("foo1"))
	assert.Equal(t, *tr1.IB().Get("foo2"), "v21")
}

func BenchmarkBadgerFs(b *testing.B) {
	bename := "badger"
	dir, _ := ioutil.TempDir("", bename)
	defer os.RemoveAll(dir)

	// Add some items we can access/delete/set
	beconf := storage.BackendConfiguration{Directory: dir}
	conf := factory.CryptoStorageConfiguration{BackendConfiguration: beconf,
		BackendName: bename, Password: "assword"}
	st := factory.NewCryptoStorage(conf)
	fs := NewFs(st, "toor", 0)
	defer fs.closeWithoutTransactions()

	tr := fs.GetTransaction()
	n := 100000
	for i := 0; i < n; i++ {
		k := NewBlockKey(uint64(i), BST_META, "").IB()
		tr.IB().Set(k, fmt.Sprintf("v%d", i))
	}
	tr.CommitUntilSucceeds()

	b.Run("Get1", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			tr := fs.GetTransaction()
			j := rand.Int() % n
			k := NewBlockKey(uint64(j), BST_META, "").IB()
			tr.IB().Get(k)
			tr.Close()
		}
	})

	b.Run("GetN", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			tr := fs.GetTransaction()
			j := rand.Int() % n
			k := NewBlockKey(uint64(j), BST_META, "").IB()
			tr.IB().Get(k)
			tr.Close()
		}
	})

	b.Run("Set", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			tr := fs.GetTransaction()
			j := rand.Int() % n
			k := NewBlockKey(uint64(j), BST_META, "").IB()
			tr.IB().Set(k, fmt.Sprintf("V%d%d", j, i))
			tr.CommitUntilSucceeds()
		}
	})

	b.Run("Delete", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			tr := fs.GetTransaction()
			j := rand.Int() % n
			k := NewBlockKey(uint64(j), BST_META, "").IB()
			tr.IB().Delete(k)
			tr.CommitUntilSucceeds()
		}
	})

}
