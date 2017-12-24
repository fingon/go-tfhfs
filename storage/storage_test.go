/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2017 Markus Stenberg
 *
 * Created:       Thu Dec 14 19:19:24 2017 mstenber
 * Last modified: Sun Dec 24 08:34:38 2017 mstenber
 * Edit time:     78 min
 *
 */

package storage

import (
	"io/ioutil"
	"log"
	"os"
	"testing"

	"github.com/stvp/assert"
)

func ProdBlockBackend(t *testing.T, factory func() BlockBackend) {
	func() {
		bs := factory()
		log.Printf("ProdBlockBackend %v", bs)
		defer bs.Close()

		b1 := &Block{Id: "foo", Data: "data", Status: BlockStatus_NORMAL}
		bs.SetInFlush(true) // enable r-w mode
		bs.SetNameToBlockId("name", "foo")
		bs.StoreBlock(b1)
		bs.SetInFlush(false)

		log.Print(" initial set")
		b2 := bs.GetBlockById("foo")
		log.Print(" got")
		assert.Equal(t, b2.GetData(), "data")
		log.Print(" data ok")
		// ^ has to be called before the next one, as .Data isn't
		// populated by default.
		//assert.Equal(t, b1, b2)
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

func ProdStorage(t *testing.T, factory func() BlockBackend) {
	bs := factory()
	log.Printf("ProdStorage %v", bs)
	defer bs.Close()

	s := Storage{Backend: bs}.Init()
	b := s.ReferOrStoreBlock("foo", "bar")
	assert.True(t, b != nil)
	assert.Equal(t, b.refCount, 1)
	b2 := s.ReferOrStoreBlock("foo", "bar")
	assert.Equal(t, b, b2)
	assert.Equal(t, b.refCount, 2)
	assert.Equal(t, len(s.dirty_bid2block), 1)
	s.Flush()
	assert.Equal(t, len(s.dirty_bid2block), 0)
}

func TestInMemory(t *testing.T) {
	ProdBlockBackend(t, func() BlockBackend {
		be := InMemoryBlockBackend{}.Init()
		return be
	})
}

func TestBadger(t *testing.T) {
	dir, _ := ioutil.TempDir("", "badger")
	defer os.RemoveAll(dir)
	ProdBlockBackend(t, func() BlockBackend {
		be := BadgerBlockBackend{}.Init(dir)
		return be
	})
}

func BenchmarkBadgerSet(b *testing.B) {
	dir, _ := ioutil.TempDir("", "badger")
	defer os.RemoveAll(dir)
	be := BadgerBlockBackend{}.Init(dir)
	defer be.Close()

	bl := &Block{Id: "foo", Data: "data"}

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

	bl := &Block{Id: "foo", Data: "data"}
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

	bl := &Block{Id: "foo", Data: "data"}
	be.SetInFlush(true)
	be.StoreBlock(bl)
	be.SetInFlush(false)

	bl2 := be.GetBlockById("foo")
	bl2.GetData()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		bl2.Data = ""
		bl2.GetData()
	}
}
