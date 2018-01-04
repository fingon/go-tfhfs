/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2017 Markus Stenberg
 *
 * Created:       Thu Dec 14 19:19:24 2017 mstenber
 * Last modified: Thu Jan  4 20:22:26 2018 mstenber
 * Edit time:     138 min
 *
 */

package storage

import (
	"io/ioutil"
	"log"
	"os"
	"testing"

	"github.com/fingon/go-tfhfs/codec"
	"github.com/fingon/go-tfhfs/mlog"
	"github.com/stvp/assert"
)

func ProdBlockBackend(t *testing.T, factory func() BlockBackend) {
	func() {
		bs := factory()
		mlog.Printf2("storage/storage_test", "ProdBlockBackend %v", bs)
		defer bs.Close()

		b1 := &Block{Id: "foo", Data: []byte("data"),
			BlockMetadata: BlockMetadata{RefCount: 123,
				Status: BlockStatus_NORMAL}}
		bs.SetInFlush(true) // enable r-w mode
		bs.SetNameToBlockId("name", "foo")
		bs.StoreBlock(b1)
		bs.SetInFlush(false)

		log.Print(" initial set")
		b2 := bs.GetBlockById("foo")
		log.Print(" got")
		assert.Equal(t, string(b2.GetData()), "data")
		log.Print(" data ok")
		// ^ has to be called before the next one, as .Data isn't
		// populated by default.
		//assert.Equal(t, b1, b2)
		assert.Equal(t, int(b2.RefCount), 123)
		assert.Equal(t, b2.Status, BlockStatus_NORMAL)

		//bs.UpdateBlockStatus(b1, BlockStatus_MISSING)
		//assert.Equal(t, b2.Status, BlockStatus_MISSING)

		log.Print(" get nok?")
		bn := bs.GetBlockIdByName("name")
		assert.Equal(t, bn, "foo")

		bs.SetInFlush(true) // enable r-w mode
		bs.SetNameToBlockId("name", "")
		bs.SetInFlush(false)
		log.Print(" second set")

		bn = bs.GetBlockIdByName("name")
		assert.Equal(t, bn, "")
	}()

	func() {
		bs := factory()
		defer bs.Close()

		b3 := bs.GetBlockById("nokey")
		assert.Nil(t, b3)

	}()
	func() {
		bs := factory()
		defer bs.Close()

		bn := bs.GetBlockIdByName("noname")
		assert.Equal(t, bn, "")

	}()
	ProdStorage(t, factory)
}

func ProdStorageOne(t *testing.T, s *Storage) {
	mlog.Printf2("storage/storage_test", "ProdStorageOne")
	v := []byte("v")
	b := s.ReferOrStoreBlock("key", v)
	assert.Equal(t, int(b.storageRefCount), 2)
	assert.True(t, b != nil)
	assert.Equal(t, int(b.RefCount), 1)
	b2 := s.ReferOrStoreBlock("key", v)
	assert.Equal(t, b, b2)
	assert.Equal(t, int(b.storageRefCount), 3)
	assert.Equal(t, int(b.RefCount), 2)
	assert.Equal(t, s.dirtyBlocks.Get().Len(), 1)
	s.Flush()
	assert.Equal(t, s.blocks.Get().Len(), 1)
	assert.Equal(t, s.dirtyBlocks.Get().Len(), 0)
	assert.Equal(t, int(b.storageRefCount), 2) // two references kept below

	b3 := s.ReferOrStoreBlock("key2", v)
	assert.Equal(t, s.blocks.Get().Len(), 2)
	assert.Equal(t, s.dirtyBlocks.Get().Len(), 1)
	assert.Equal(t, int(b3.storageRefCount), 2)
	assert.Equal(t, int(b3.RefCount), 1)
	// ^ b.size must be <= 3/4 max

	assert.Equal(t, s.dirtyBlocks.Get().Len(), 1)
	s.Flush()
	assert.Equal(t, s.blocks.Get().Len(), 2)
	assert.Equal(t, s.dirtyBlocks.Get().Len(), 0)
	assert.Equal(t, int(b3.storageRefCount), 1)
	assert.Equal(t, int(b3.RefCount), 1)
	s.ReleaseBlockId("key2")

	s.ReleaseBlockId("key")
	s.ReleaseBlockId("key")
	s.Flush()
	assert.Equal(t, s.blocks.Get().Len(), 2)

	assert.Equal(t, int(b.storageRefCount), 2)
	assert.Equal(t, int(b3.storageRefCount), 1)
	assert.Equal(t, s.dirtyBlocks.Get().Len(), 0)
	assert.Equal(t, s.blocks.Get().Len(), 2)
	b.Close()
	b.Close()
	b3.Close()
	assert.Equal(t, int(b.storageRefCount), 0)
	assert.Equal(t, int(b3.storageRefCount), 0)
	assert.Equal(t, s.dirtyBlocks.Get().Len(), 0)
	assert.Equal(t, s.blocks.Get().Len(), 0)
}

func ProdStorage(t *testing.T, factory func() BlockBackend) {
	bs := factory()
	mlog.Printf2("storage/storage_test", "ProdStorage %v", bs)
	defer bs.Close()

	s := Storage{Backend: bs}.Init()
	ProdStorageOne(t, s)

	c := codec.CodecChain{}.Init(&codec.CompressingCodec{})
	s2 := Storage{Backend: bs, Codec: c}.Init()
	ProdStorageOne(t, s2)

}

func TestInMemory(t *testing.T) {
	t.Parallel()
	ProdBlockBackend(t, func() BlockBackend {
		be := InMemoryBlockBackend{}.Init()
		return be
	})
}

func TestBadger(t *testing.T) {
	t.Parallel()
	dir, _ := ioutil.TempDir("", "badger")
	defer os.RemoveAll(dir)
	ProdBlockBackend(t, func() BlockBackend {
		be := BadgerBlockBackend{}.Init(dir)
		return be
	})
}

func TestBolt(t *testing.T) {
	t.Parallel()
	dir, _ := ioutil.TempDir("", "badger")
	defer os.RemoveAll(dir)
	ProdBlockBackend(t, func() BlockBackend {
		be := BoltBlockBackend{}.Init(dir)
		return be
	})
}

func TestFile(t *testing.T) {
	t.Parallel()
	dir, _ := ioutil.TempDir("", "file")
	defer os.RemoveAll(dir)
	ProdBlockBackend(t, func() BlockBackend {
		be := &FileBlockBackend{}
		be.Init(dir)
		return be
	})
}

func BenchmarkBadgerSet(b *testing.B) {
	dir, _ := ioutil.TempDir("", "badger")
	defer os.RemoveAll(dir)
	be := BadgerBlockBackend{}.Init(dir)
	defer be.Close()

	bl := &Block{Id: "foo", Data: []byte("data")}

	b.ResetTimer()

	be.SetInFlush(true)
	for i := 0; i < b.N; i++ {
		be.StoreBlock(bl)
	}
	be.SetInFlush(false)
}

func BenchmarkBadgerGet(b *testing.B) {
	dir, _ := ioutil.TempDir("", "badger")
	defer os.RemoveAll(dir)
	be := BadgerBlockBackend{}.Init(dir)
	defer be.Close()

	bl := &Block{Id: "foo", Data: []byte("data")}
	be.SetInFlush(true)
	be.StoreBlock(bl)
	be.SetInFlush(false)
	be.GetBlockById("foo")

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		be.GetBlockById("foo")
	}
}

func BenchmarkBadgerGetData(b *testing.B) {
	dir, _ := ioutil.TempDir("", "badger")
	defer os.RemoveAll(dir)
	be := BadgerBlockBackend{}.Init(dir)
	defer be.Close()

	bl := &Block{Id: "foo", Data: []byte("data")}
	be.SetInFlush(true)
	be.StoreBlock(bl)
	be.SetInFlush(false)

	bl2 := be.GetBlockById("foo")
	bl2.GetData()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		bl2.Data = nil
		bl2.GetData()
	}
}
