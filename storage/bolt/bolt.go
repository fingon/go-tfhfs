/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2018 Markus Stenberg
 *
 * Created:       Wed Jan  3 22:49:15 2018 mstenber
 * Last modified: Wed Jan 10 11:32:34 2018 mstenber
 * Edit time:     29 min
 *
 */

package bolt

import (
	"fmt"
	"log"

	bbolt "github.com/coreos/bbolt"

	"github.com/fingon/go-tfhfs/mlog"
	"github.com/fingon/go-tfhfs/storage"
)

var metadataKey = []byte("key")
var dataKey = []byte("data")
var nameKey = []byte("name")

// boltBackend provides on-disk storage.
//
// - key prefix 1 + block id -> metadata
// - key prefix 2 + block id -> data (essentially immutable)
// - key prefix 3 + name -> block id
type boltBackend struct {
	storage.DirectoryBackendBase

	db *bbolt.DB
}

var _ storage.Backend = &boltBackend{}

// Init makes the instance actually useful
func NewBoltBackend() storage.Backend {
	self := &boltBackend{}
	return self
}

func (self *boltBackend) Init(config storage.BackendConfiguration) {
	dir := config.Directory
	(&self.DirectoryBackendBase).Init(config)
	db, err := bbolt.Open(fmt.Sprintf("%s/bbolt.db", dir), 0600, nil)
	if err != nil {
		log.Fatal("bbolt.Open", err)
	}
	self.db = db
	err = db.Update(func(tx *bbolt.Tx) error {
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
}

func (self *boltBackend) Close() {
	self.db.Close()
}

func (self *boltBackend) DeleteBlock(b *storage.Block) {
	mlog.Printf2("storage/bolt/bolt", "bbolt.DeleteBlock %x", b.Id)
	bid := []byte(b.Id)
	self.db.Update(func(tx *bbolt.Tx) error {
		tx.Bucket(metadataKey).Delete(bid)
		tx.Bucket(dataKey).Delete(bid)
		return nil
	})
}

func (self *boltBackend) GetBlockData(b *storage.Block) (v []byte) {
	bid := []byte(b.Id)
	self.db.View(func(tx *bbolt.Tx) error {
		v = tx.Bucket(dataKey).Get(bid)
		return nil
	})
	return
}

func (self *boltBackend) GetBlockById(id string) *storage.Block {
	bid := []byte(id)
	var bv []byte
	self.db.View(func(tx *bbolt.Tx) error {
		bv = tx.Bucket(metadataKey).Get(bid)
		return nil
	})
	if bv == nil {
		return nil
	}
	b := &storage.Block{Id: id, Backend: self}
	_, err := b.BlockMetadata.UnmarshalMsg(bv)
	if err != nil {
		log.Fatal(err)
	}
	mlog.Printf2("storage/bolt/bolt", "bbolt.GetBlockById %x", id)
	return b
}

func (self *boltBackend) GetBlockIdByName(name string) (s string) {
	self.db.View(func(tx *bbolt.Tx) error {
		s = string(tx.Bucket(nameKey).Get([]byte(name)))
		return nil
	})
	return
}

func (self *boltBackend) SetNameToBlockId(name, block_id string) {
	self.db.Update(func(tx *bbolt.Tx) error {
		tx.Bucket(nameKey).Put([]byte(name), []byte(block_id))
		return nil
	})
	return
}

func (self *boltBackend) StoreBlock(b *storage.Block) {
	data := b.Data.Get()
	if data == nil {
		log.Panicf("data not set in StoreBlock")
	}
	bid := []byte(b.Id)
	mlog.Printf2("storage/bolt/bolt", "bbolt.StoreBlock %x (%d b)", bid, len(*data))
	self.updateBlock(b)
	self.db.Update(func(tx *bbolt.Tx) error {
		tx.Bucket(dataKey).Put(bid, *data)
		return nil
	})
}

func (self *boltBackend) updateBlock(b *storage.Block) {
	buf, err := b.BlockMetadata.MarshalMsg(nil)
	if err != nil {
		log.Fatal(err)
	}
	bid := []byte(b.Id)
	self.db.Update(func(tx *bbolt.Tx) error {
		tx.Bucket(metadataKey).Put(bid, buf)
		return nil
	})
}

func (self *boltBackend) UpdateBlock(b *storage.Block) int {
	mlog.Printf2("storage/bolt/bolt", "bbolt.UpdateBlock %x", b.Id)
	self.updateBlock(b)
	return 1
}
