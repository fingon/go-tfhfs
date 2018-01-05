/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2017 Markus Stenberg
 *
 * Created:       Sat Dec 23 15:10:01 2017 mstenber
 * Last modified: Fri Jan  5 14:51:56 2018 mstenber
 * Edit time:     142 min
 *
 */

package badger

import (
	"log"

	"github.com/dgraph-io/badger"
	"github.com/fingon/go-tfhfs/mlog"
	"github.com/fingon/go-tfhfs/storage"
)

// badgerBackend provides on-disk storage.
//
// - key prefix 1 + block id -> metadata
// - key prefix 2 + block id -> data (essentially immutable)
// - key prefix 3 + name -> block id
type badgerBackend struct {
	storage.DirectoryBackendBase
	db *badger.DB
}

var _ storage.Backend = &badgerBackend{}

// Init makes the instance actually useful

func NewBadgerBackend(dir string) storage.Backend {
	self := &badgerBackend{}
	(&self.DirectoryBackendBase).Init(dir)
	opts := badger.DefaultOptions
	opts.Dir = dir
	opts.ValueDir = dir
	db, err := badger.Open(opts)
	if err != nil {
		log.Panic("badger.Open", err)
	}
	self.db = db
	return self

}

func (self *badgerBackend) Close() {
	self.db.Close()
}

func (self *badgerBackend) DeleteBlock(b *storage.Block) {
	mlog.Printf2("storage/badger/badger", "bad.DeleteBlock %x", b.Id)
	self.db.Update(func(txn *badger.Txn) error {
		k := append([]byte("1"), []byte(b.Id)...)
		if err := self.delete(k); err != nil {
			log.Panic("txn.Delete", err)
		}
		k = append([]byte("2"), []byte(b.Id)...)
		if err := self.delete(k); err != nil {
			log.Panic("txn.Delete 2", err)
		}
		return nil
	})
}

func (self *badgerBackend) getKKValue(prefix, suffix []byte) (v []byte, err error) {
	err = self.db.View(func(txn *badger.Txn) error {
		k := append(prefix, suffix...)
		i, err := txn.Get(k)
		if err == nil {
			v, err = i.ValueCopy(nil)
		}
		return err
	})
	return
}

func (self *badgerBackend) GetBlockData(b *storage.Block) []byte {
	bv, _ := self.getKKValue([]byte("2"), []byte(b.Id))
	return bv
}

func (self *badgerBackend) GetBlockById(id string) *storage.Block {
	bv, err := self.getKKValue([]byte("1"), []byte(id))
	if err == badger.ErrKeyNotFound {
		return nil
	}
	if err != nil {
		log.Panic("get error:", err)
	}
	b := &storage.Block{Id: id, Backend: self}
	_, err = b.BlockMetadata.UnmarshalMsg(bv)
	if err != nil {
		log.Panic(err)
	}
	mlog.Printf2("storage/badger/badger", "b.GetBlockById %x", id)
	return b
}

func (self *badgerBackend) GetBlockIdByName(name string) string {
	bv, err := self.getKKValue([]byte("3"), []byte(name))
	if err == badger.ErrKeyNotFound {
		return ""
	}
	if err != nil {
		log.Panic("get error:", err)
	}
	return string(bv)
}

func (self *badgerBackend) setKKValue(prefix, suffix, value []byte) {
	k := append(prefix, suffix...)
	if err := self.set(k, value); err != nil {
		log.Panic("set", err)
	}
}

func (self *badgerBackend) delete(k []byte) error {
	return self.db.Update(func(txn *badger.Txn) error {
		return txn.Delete(k)
	})
}

func (self *badgerBackend) set(k, v []byte) error {
	return self.db.Update(func(txn *badger.Txn) error {
		return txn.Set(k, v)
	})
}

func (self *badgerBackend) SetNameToBlockId(name, block_id string) {
	mlog.Printf2("storage/badger/badger", "bad.SetNameToBlockId %s = %x", name, block_id)
	self.setKKValue([]byte("3"), []byte(name), []byte(block_id))
}

func (self *badgerBackend) StoreBlock(b *storage.Block) {
	mlog.Printf2("storage/badger/badger", "bad.StoreBlock %x (%d b)", b.Id, len(b.Data))
	self.updateBlock(b)
	self.setKKValue([]byte("2"), []byte(b.Id), b.Data)
}

func (self *badgerBackend) updateBlock(b *storage.Block) {
	buf, err := b.BlockMetadata.MarshalMsg(nil)
	if err != nil {
		log.Panic(err)
	}
	self.setKKValue([]byte("1"), []byte(b.Id), buf)
}

func (self *badgerBackend) UpdateBlock(b *storage.Block) int {
	mlog.Printf2("storage/badger/badger", "bad.UpdateBlock %x", b.Id)
	self.updateBlock(b)
	return 1
}
