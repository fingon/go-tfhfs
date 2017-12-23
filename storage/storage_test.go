/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2017 Markus Stenberg
 *
 * Created:       Thu Dec 14 19:19:24 2017 mstenber
 * Last modified: Sat Dec 23 14:56:34 2017 mstenber
 * Edit time:     35 min
 *
 */

package storage

import (
	"log"
	"testing"

	"github.com/fingon/go-tfhfs/tfhfs_proto"
	"github.com/golang/protobuf/proto"
	"github.com/stvp/assert"
)

func TestProto(t *testing.T) {
	n := &tfhfs_proto.TreeNodeEntry{Key: []byte("foo")}
	data, err := proto.Marshal(n)
	if err != nil {
		log.Fatal("marshaling error: ", err)
	}
	n2 := &tfhfs_proto.TreeNodeEntry{}
	err = proto.Unmarshal(data, n2)
	if err != nil {
		log.Fatal("unmarshaling error: ", err)
	}
	assert.Equal(t, n.Key, n2.Key)
	assert.True(t, proto.Equal(n, n2))
}

func ProdBlockBackend(t *testing.T, factory func() BlockBackend) {
	bs := factory()
	b1 := &Block{Id: "foo", Data: "data"}
	bs.SetNameToBlockId("name", "foo")
	bs.StoreBlock(b1)

	b2 := bs.GetBlockById("foo")
	assert.Equal(t, b1, b2)
	assert.Equal(t, b2.GetData(), "data")
	assert.Equal(t, b2.Status, tfhfs_proto.BlockStatus_NORMAL)

	//bs.UpdateBlockStatus(b1, tfhfs_proto.BlockStatus_MISSING)
	//assert.Equal(t, b2.Status, tfhfs_proto.BlockStatus_MISSING)

	bn := bs.GetBlockIdByName("name")
	assert.Equal(t, bn, "foo")

	bs.SetNameToBlockId("name", "")
	bn = bs.GetBlockIdByName("name")
	assert.Equal(t, bn, "")

	bs = factory()
	b3 := bs.GetBlockById("nokey")
	assert.Nil(t, b3)

	bs = factory()
	bn = bs.GetBlockIdByName("noname")
	assert.Equal(t, bn, "")

}

func ProdStorage(t *testing.T, factory func() BlockBackend) {
	bs := factory()
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
		return InMemoryBlockBackend{}.Init()
	})
	ProdStorage(t, func() BlockBackend {
		return InMemoryBlockBackend{}.Init()
	})
}
