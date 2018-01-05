/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2017 Markus Stenberg
 *
 * Created:       Thu Dec 14 19:19:24 2017 mstenber
 * Last modified: Fri Jan  5 16:33:16 2018 mstenber
 * Edit time:     182 min
 *
 */

package storage_test

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"
	"testing"

	"github.com/fingon/go-tfhfs/codec"
	"github.com/fingon/go-tfhfs/mlog"
	"github.com/fingon/go-tfhfs/storage"
	"github.com/fingon/go-tfhfs/storage/factory"
	"github.com/stvp/assert"
)

func ProdBackend(t *testing.T, factory func() storage.Backend) {
	bs := factory()
	mlog.Printf2("storage/storage_test", "ProdBackend %v", bs)

	b1 := &storage.Block{Id: "foo", Data: []byte("data"),
		BlockMetadata: storage.BlockMetadata{RefCount: 123,
			Status: storage.BlockStatus_NORMAL}}
	bs.SetNameToBlockId("name", "foo")
	bs.StoreBlock(b1)

	log.Print(" initial set")
	b2 := bs.GetBlockById("foo")
	log.Print(" got")
	assert.Equal(t, string(b2.GetData()), "data")
	log.Print(" data ok")
	// ^ has to be called before the next one, as .Data isn't
	// populated by default.
	//assert.Equal(t, b1, b2)
	assert.Equal(t, int(b2.RefCount), 123)
	assert.Equal(t, b2.Status, storage.BlockStatus_NORMAL)

	//bs.UpdateBlockStatus(b1, BlockStatus_MISSING)
	//assert.Equal(t, b2.Status, BlockStatus_MISSING)

	log.Print(" get nok?")
	bn := bs.GetBlockIdByName("name")
	assert.Equal(t, bn, "foo")

	bs.SetNameToBlockId("name", "")
	log.Print(" second set")

	bn = bs.GetBlockIdByName("name")
	assert.Equal(t, bn, "")
	bs.Close()

	// Ensure second backend nop key fetch will return nothing
	bs = factory()
	b3 := bs.GetBlockById("nokey")
	assert.Nil(t, b3)
	bs.Close()

	// Ensure third backend nop name fetch will return nothing
	bs = factory()
	bn = bs.GetBlockIdByName("noname")
	assert.Equal(t, bn, "")
	bs.Close()

	ProdStorage(t, factory)
}

func ProdStorageOne(t *testing.T, s *storage.Storage) {
	mlog.Printf2("storage/storage_test", "ProdStorageOne")
	v := []byte("v")
	b := s.ReferOrStoreBlock("key", v)
	// assert.Equal(t, int(b.storageRefCount), 2)
	assert.True(t, b != nil)
	// assert.Equal(t, int(b.RefCount), 1)
	b2 := s.ReferOrStoreBlock("key", v)
	assert.Equal(t, b, b2)
	// assert.Equal(t, int(b.storageRefCount), 3)
	// assert.Equal(t, int(b.RefCount), 2)
	// assert.Equal(t, s.dirtyBlocks.Get().Len(), 1)
	s.Flush()
	// assert.Equal(t, s.blocks.Get().Len(), 1)
	// assert.Equal(t, s.dirtyBlocks.Get().Len(), 0)
	// assert.Equal(t, int(b.storageRefCount), 2) // two references kept below

	b3 := s.ReferOrStoreBlock("key2", v)
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
	s.ReleaseBlockId("key2")

	s.ReleaseBlockId("key")
	s.ReleaseBlockId("key")
	s.Flush()
	// assert.Equal(t, s.blocks.Get().Len(), 2)

	// assert.Equal(t, int(b.storageRefCount), 2)
	// assert.Equal(t, int(b3.storageRefCount), 1)
	// assert.Equal(t, s.dirtyBlocks.Get().Len(), 0)
	// assert.Equal(t, s.blocks.Get().Len(), 2)
	b.Close()
	b.Close()
	b3.Close()
	// assert.Equal(t, int(b.storageRefCount), 0)
	// assert.Equal(t, int(b3.storageRefCount), 0)
	// assert.Equal(t, s.dirtyBlocks.Get().Len(), 0)
	// assert.Equal(t, s.blocks.Get().Len(), 0)
}

func ProdStorageDeps(t *testing.T, s *storage.Storage) {
	mlog.Printf("ProdStorageDeps")
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
		s.ReferOrStoreBlock(v.key, []byte(v.value)).Close()
	}
	s.SetNameToBlockId("name", "sub")
	for _, v := range world {
		s.ReleaseBlockId(v.key)
	}
	s.Flush()
	n := s.GetBlockIdByName("name")
	assert.Equal(t, n, "sub")
	b := s.GetBlockById("sub12")
	assert.True(t, b != nil)
	b.Close()
}

func ProdStorage(t *testing.T, factory func() storage.Backend) {
	bs := factory()
	mlog.Printf2("storage/storage_test", "ProdStorage %v", bs)
	defer bs.Close()

	s := storage.Storage{Backend: bs}.Init()
	ProdStorageOne(t, s)
	s.Close()

	c := codec.CodecChain{}.Init(&codec.CompressingCodec{})
	s2 := storage.Storage{Backend: bs, Codec: c}.Init()
	ProdStorageOne(t, s2)
	s2.Close()

	s3 := storage.Storage{Backend: bs,
		Codec: c,
		IterateReferencesCallback: func(id string, data []byte, cb storage.BlockReferenceCallback) {
			for _, subid := range strings.Split(string(data), " ") {
				if subid != "" {
					cb(subid)
				}
			}
		}}.Init()
	ProdStorageDeps(t, s3)
	s3.Close()

}

func TestBackend(t *testing.T) {
	for _, k := range factory.List() {
		k := k
		t.Run(k, func(t *testing.T) {
			t.Parallel()
			dir, _ := ioutil.TempDir("", k)
			defer os.RemoveAll(dir)
			ProdBackend(t, func() storage.Backend {
				return factory.New(k, dir)
			})
		})
	}
}

func BenchmarkBackend(b *testing.B) {
	for _, k := range factory.List() {
		k := k
		dir, _ := ioutil.TempDir("", k)
		defer os.RemoveAll(dir)
		be := factory.New(k, dir)
		defer be.Close()
		bl := &storage.Block{Id: "foo", Data: []byte("data")}

		b.Run(fmt.Sprintf("%s-set", k),
			func(b *testing.B) {
				for i := 0; i < b.N; i++ {
					be.StoreBlock(bl)
				}
			})
		b.Run(fmt.Sprintf("%s-get", k),
			func(b *testing.B) {
				be.StoreBlock(bl)
				be.GetBlockById("foo")
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					be.GetBlockById("foo")
				}
			})
		b.Run(fmt.Sprintf("%s-get", k),
			func(b *testing.B) {
				bl2 := be.GetBlockById("foo")
				bl2.GetData()
				b.ResetTimer()

				for i := 0; i < b.N; i++ {
					bl2.Data = nil
					bl2.GetData()
				}

			})
	}
}
