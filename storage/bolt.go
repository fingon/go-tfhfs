/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2018 Markus Stenberg
 *
 * Created:       Wed Jan  3 22:49:15 2018 mstenber
 * Last modified: Thu Jan  4 20:09:16 2018 mstenber
 * Edit time:     18 min
 *
 */

package storage

import (
	"fmt"
	"log"

	bolt "github.com/coreos/bbolt"

	"github.com/fingon/go-tfhfs/mlog"
)

var metadataKey = []byte("key")
var dataKey = []byte("data")
var nameKey = []byte("name")

// BoltBlockBackend provides on-disk storage.
//
// - key prefix 1 + block id -> metadata
// - key prefix 2 + block id -> data (essentially immutable)
// - key prefix 3 + name -> block id
type BoltBlockBackend struct {
	DirectoryBlockBackendBase

	db *bolt.DB
	tx *bolt.Tx
}

var _ BlockBackend = &BoltBlockBackend{}

// Init makes the instance actually useful
func (self BoltBlockBackend) Init(dir string) *BoltBlockBackend {
	(&self.DirectoryBlockBackendBase).Init(dir)
	db, err := bolt.Open(fmt.Sprintf("%s/bolt.db", dir), 0600, nil)
	if err != nil {
		log.Fatal("bolt.Open", err)
	}
	self.db = db
	err = db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists(metadataKey)
		if err != nil {
			log.Panic(err)
		}
		_, err = tx.CreateBucketIfNotExists(dataKey)
		if err != nil {
			log.Panic(err)
		}
		_, err = tx.CreateBucketIfNotExists(nameKey)
		if err != nil {
			log.Panic(err)
		}
		return nil
	})
	if err != nil {
		log.Panic(err)
	}
	self.tx, err = self.db.Begin(false)
	if err != nil {
		log.Panic(err)
	}
	return &self
}

func (self *BoltBlockBackend) DeleteBlock(b *Block) {
	mlog.Printf2("storage/bolt", "bolt.DeleteBlock %x", b.Id)
	bid := []byte(b.Id)
	self.tx.Bucket(metadataKey).Delete(bid)
	self.tx.Bucket(dataKey).Delete(bid)
}

func (self *BoltBlockBackend) GetBlockData(b *Block) []byte {
	bid := []byte(b.Id)
	return self.tx.Bucket(dataKey).Get(bid)
}

func (self *BoltBlockBackend) GetBlockById(id string) *Block {
	bid := []byte(id)
	bv := self.tx.Bucket(metadataKey).Get(bid)
	if bv == nil {
		return nil
	}
	b := &Block{Id: id, backend: self}
	_, err := b.BlockMetadata.UnmarshalMsg(bv)
	if err != nil {
		log.Fatal(err)
	}
	mlog.Printf2("storage/bolt", "bolt.GetBlockById %x", id)
	return b
}

func (self *BoltBlockBackend) GetBlockIdByName(name string) string {
	return string(self.tx.Bucket(nameKey).Get([]byte(name)))
}

func (self *BoltBlockBackend) SetInFlush(value bool) {
	mlog.Printf2("storage/bolt", "bolt.SetInFlush %v", value)
	if !value {
		self.tx.Commit()
	}
	self.tx.Rollback()
	tx, err := self.db.Begin(value)
	if err != nil {
		log.Panic(err)
	}
	self.tx = tx
}

func (self *BoltBlockBackend) SetNameToBlockId(name, block_id string) {
	self.tx.Bucket(nameKey).Put([]byte(name), []byte(block_id))
}

func (self *BoltBlockBackend) StoreBlock(b *Block) {
	data := b.GetCodecData()
	bid := []byte(b.Id)
	mlog.Printf2("storage/bolt", "bolt.StoreBlock %x (%d b)", bid, len(data))
	self.updateBlock(b)
	self.tx.Bucket(dataKey).Put(bid, data)
}

func (self *BoltBlockBackend) updateBlock(b *Block) {
	buf, err := b.BlockMetadata.MarshalMsg(nil)
	if err != nil {
		log.Fatal(err)
	}
	bid := []byte(b.Id)
	self.tx.Bucket(metadataKey).Put(bid, buf)
}

func (self *BoltBlockBackend) UpdateBlock(b *Block) int {
	mlog.Printf2("storage/bolt", "bolt.UpdateBlock %x", b.Id)
	self.updateBlock(b)
	return 1
}

func (self *BoltBlockBackend) Close() {
	self.tx.Rollback()
	self.db.Close()
}
