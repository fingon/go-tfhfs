/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2017 Markus Stenberg
 *
 * Created:       Thu Dec 14 19:19:24 2017 mstenber
 * Last modified: Thu Jan 25 00:30:43 2018 mstenber
 * Edit time:     229 min
 *
 */

package storage_test

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/fingon/go-tfhfs/codec"
	"github.com/fingon/go-tfhfs/mlog"
	"github.com/fingon/go-tfhfs/storage"
	"github.com/fingon/go-tfhfs/storage/factory"
	"github.com/stvp/assert"
)

func ProdBackend(t *testing.T, factory func() storage.Backend) {
	be := factory()
	mlog.Printf2("storage/storage_test", "ProdBackend %v", be)

	b1 := &storage.Block{Id: "foo",
		BlockMetadata: storage.BlockMetadata{RefCount: 123,
			Status: storage.BS_NORMAL}}
	data := []byte("data")
	b1.Data.Set(&data)
	be.SetNameToBlockId("name", "foo")
	be.StoreBlock(b1)

	mlog.Printf2("storage/storage_test", " initial set")
	b2 := be.GetBlockById("foo")
	mlog.Printf2("storage/storage_test", " got")
	assert.Equal(t, string(b2.GetData()), "data")
	mlog.Printf2("storage/storage_test", " data ok")
	// ^ has to be called before the next one, as .Data isn't
	// populated by default.
	//assert.Equal(t, b1, b2)
	assert.Equal(t, int(b2.RefCount), 123)
	assert.Equal(t, b2.Status, storage.BS_NORMAL)

	//be.UpdateBlockStatus(b1, BS_MISSING)
	//assert.Equal(t, b2.Status, BS_MISSING)

	mlog.Printf2("storage/storage_test", " get nok?")
	bn := be.GetBlockIdByName("name")
	assert.Equal(t, bn, "foo")

	be.SetNameToBlockId("name", "")
	mlog.Printf2("storage/storage_test", " second set")

	bn = be.GetBlockIdByName("name")
	assert.Equal(t, bn, "")
	be.Close()

	// Ensure second backend nop key fetch will return nothing
	be = factory()
	b3 := be.GetBlockById("nokey")
	assert.Nil(t, b3)
	be.Close()

	// Ensure third backend nop name fetch will return nothing
	be = factory()
	bn = be.GetBlockIdByName("noname")
	assert.Equal(t, bn, "")
	be.Close()

	ProdStorage(t, factory)
}

func ProdStorageOne(t *testing.T, s *storage.Storage) {
	mlog.Printf2("storage/storage_test", "ProdStorageOne")
	assert.Equal(t, s.TransientCount(), 0)
	v := []byte("v")
	st := storage.BS_NORMAL
	b := s.ReferOrStoreBlock("key", st, v) // +1 key ref, sref
	// assert.Equal(t, int(b.storageRefCount), 2)
	assert.True(t, b != nil)
	// assert.Equal(t, int(b.RefCount), 1)
	b2 := s.ReferOrStoreBlock("key", st, v) // +1 key ref, sref
	// assert.Equal(t, b, b2)
	// assert.Equal(t, int(b.storageRefCount), 3)
	// assert.Equal(t, int(b.RefCount), 2)
	// assert.Equal(t, s.dirtyBlocks.Get().Len(), 1)
	s.Flush()
	// assert.Equal(t, s.blocks.Get().Len(), 1)
	// assert.Equal(t, s.dirtyBlocks.Get().Len(), 0)
	// assert.Equal(t, int(b.storageRefCount), 2) // two references kept below

	b3 := s.ReferOrStoreBlock("key2", st, v) // +1 key2 ref, sref
	// assert.Equal(t, s.blocks.Get().Len(), 2)
	// assert.Equal(t, s.dirtyBlocks.Get().Len(), 1)
	// assert.Equal(t, int(b3.storageRefCount), 2)
	// assert.Equal(t, int(b3.RefCount), 1)
	// ^ b.size must be <= 3/4 max

	// assert.Equal(t, s.dirtyBlocks.Get().Len(), 1)
	s.Flush()
	// assert.Equal(t, s.blocks.Get().Len(), 2)
	// assert.Equal(t, s.dirtyBlocks.Get().Len(), 0)
	// assert.Equal(t, int(b3.storageRefCount), 1)
	// assert.Equal(t, int(b3.RefCount), 1)
	s.ReleaseBlockId("key2") // -1 key2 ref

	s.ReleaseBlockId("key") // -1 key ref
	s.ReleaseBlockId("key") // -1 key ref
	s.Flush()
	// assert.Equal(t, s.blocks.Get().Len(), 2)

	// assert.Equal(t, int(b.storageRefCount), 2)
	// assert.Equal(t, int(b3.storageRefCount), 1)
	// assert.Equal(t, s.dirtyBlocks.Get().Len(), 0)
	// assert.Equal(t, s.blocks.Get().Len(), 2)
	b.Close()
	b2.Close()
	b3.Close()
	// assert.Equal(t, int(b.storageRefCount), 0)
	// assert.Equal(t, int(b3.storageRefCount), 0)
	// assert.Equal(t, s.dirtyBlocks.Get().Len(), 0)
	// assert.Equal(t, s.blocks.Get().Len(), 0)
	s.Flush()
	assert.Equal(t, s.TransientCount(), 0)
}

func ProdStorageDeps(t *testing.T, be storage.Backend) {
	k2v := make(map[string]string)
	broken := false
	s := storage.Storage{Backend: be,
		IterateReferencesCallback: func(id string, data []byte, cb storage.BlockReferenceCallback) {
			s := string(data)
			if string(data) != k2v[id] {
				mlog.Printf2("storage/storage_test", "data mismatch: %s <> %s", s, k2v[id])
				broken = true
			}
			mlog.Printf2("storage/storage_test", "id:%v data:%v", id, s)
			for _, subid := range strings.Split(s, " ") {
				if subid != "" {
					mlog.Printf2("storage/storage_test", " %v", subid)
					cb(subid)
				}
			}
		}}.Init()
	defer s.Close()
	mlog.Printf2("storage/storage_test", "ProdStorageDeps")
	assert.Equal(t, s.TransientCount(), 0)
	world := []struct {
		key, value string
	}{
		{"sub11", " "},
		{"sub12", " "},
		{"sub1", "sub11 sub12"},
		{"sub2", " "},
		{"sub", "sub1 sub2"},
	}

	for _, v := range world {
		k2v[v.key] = v.value
		nv := make([]byte, len(v.value))
		copy(nv, []byte(v.value))
		s.ReferOrStoreBlock(v.key, storage.BS_NORMAL, nv).Close()
	}
	name := "subname"
	s.SetNameToBlockId(name, "sub")
	for _, v := range world {
		s.ReleaseBlockId(v.key)
	}
	s.Flush()
	n := s.GetBlockIdByName(name)
	assert.Equal(t, n, "sub")

	b := s.GetBlockById("sub12")
	assert.True(t, b != nil)
	b.Close()

	assert.Equal(t, s.TransientCount(), 0)
	s.SetNameToBlockId(name, "sub1")
	s.Flush()

	// should be gone due to ref disappearing
	assert.Nil(t, s.GetBlockById("sub"))

	// children of sub should be still ok
	b = s.GetBlockById("sub12")
	assert.True(t, b != nil)
	b.Close()

	// transient count still should be 0
	assert.Equal(t, s.TransientCount(), 0)

	assert.True(t, !broken)
}

func ProdStorage(t *testing.T, factory func() storage.Backend) {
	be := factory()
	mlog.Printf2("storage/storage_test", "ProdStorage %v", be)

	s := storage.Storage{Backend: be}.Init()
	ProdStorageOne(t, s)
	s.Backend = nil
	s.Close()

	c := codec.CodecChain{}.Init(&codec.CompressingCodec{})
	s2 := storage.Storage{Backend: be, Codec: c}.Init()
	ProdStorageOne(t, s2)
	s2.Backend = nil
	s2.Close()

	ProdStorageDeps(t, be)
}

func TestBackend(t *testing.T) {
	for _, k := range factory.List() {
		k := k
		t.Run(k, func(t *testing.T) {
			t.Parallel()
			dir, _ := ioutil.TempDir("", k)
			defer os.RemoveAll(dir)
			ProdBackend(t, func() storage.Backend {
				config := storage.BackendConfiguration{Directory: dir,
					DelayPerOp: time.Millisecond}
				return factory.NewWithConfig(k, config)
			})
		})
	}
}

func BenchmarkBackend(b *testing.B) {
	for _, k := range factory.List() {
		k := k
		setup := func() (storage.Backend, func()) {
			dir, _ := ioutil.TempDir("", k)
			be := factory.New(k, dir)
			return be, func() {
				be.Close()
				os.RemoveAll(dir)

			}
		}
		bl := &storage.Block{Id: "foo"}
		data := []byte("data")
		bl.Data.Set(&data)

		b.Run(fmt.Sprintf("%s-set", k),
			func(b *testing.B) {
				be, undo := setup()
				defer undo()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					bl := &storage.Block{Id: fmt.Sprintf("foo%d", i)}
					bl.Data.Set(&data)
					be.StoreBlock(bl)
				}
			})
		b.Run(fmt.Sprintf("%s-get", k),
			func(b *testing.B) {
				be, undo := setup()
				defer undo()
				be.StoreBlock(bl)
				be.GetBlockById("foo")
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					be.GetBlockById("foo")
				}
			})
		b.Run(fmt.Sprintf("%s-getdata", k),
			func(b *testing.B) {
				be, undo := setup()
				defer undo()

				be.StoreBlock(bl)
				bl2 := be.GetBlockById("foo")
				bl2.GetData()
				b.ResetTimer()

				for i := 0; i < b.N; i++ {
					bl2.Data.Set(nil)
					bl2.GetData()
				}

			})
	}
}
