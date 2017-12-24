/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2017 Markus Stenberg
 *
 * Created:       Sat Dec 23 15:10:01 2017 mstenber
 * Last modified: Sun Dec 24 08:56:02 2017 mstenber
 * Edit time:     84 min
 *
 */

package storage

import (
	"log"

	"github.com/dgraph-io/badger"
)

// BadgerBlockBackend provides on-disk storage.
// key prefix 1 + block id -> metadata
// key prefix 2 + block id -> data (essentially immutable)
// key prefix 3 + name -> block id
type BadgerBlockBackend struct {
	db  *badger.DB
	txn *badger.Txn
}

var _ BlockBackend = &BadgerBlockBackend{}

// Init makes the instance actually useful
func (self BadgerBlockBackend) Init(dir string) *BadgerBlockBackend {
	opts := badger.DefaultOptions
	opts.Dir = dir
	opts.ValueDir = dir
	db, err := badger.Open(opts)
	if err != nil {
		log.Fatal("badger.Open", err)
	}
	self.db = db
	self.txn = db.NewTransaction(false)
	return &self
}

func (self *BadgerBlockBackend) DeleteBlock(b *Block) {
	k := append([]byte("1"), []byte(b.Id)...)
	if err := self.delete(k); err != nil {
		log.Fatal("txn.Delete", err)
	}
	k = append([]byte("2"), []byte(b.Id)...)
	if err := self.delete(k); err != nil {
		log.Fatal("txn.Delete 2", err)
	}
}

func (self *BadgerBlockBackend) getKKValue(prefix, suffix []byte) ([]byte, error) {
	k := append(prefix, suffix...)
	i, err := self.txn.Get(k)

	if err != nil {
		return nil, err
	}
	v, err := i.Value()
	if err != nil {
		return nil, err
	}
	return v, nil

}

func (self *BadgerBlockBackend) GetBlockData(b *Block) string {
	bv, _ := self.getKKValue([]byte("2"), []byte(b.Id))
	return string(bv)
}

func (self *BadgerBlockBackend) GetBlockById(id string) *Block {
	bv, err := self.getKKValue([]byte("1"), []byte(id))
	if err == badger.ErrKeyNotFound {
		return nil
	}
	if err != nil {
		log.Fatal("get error:", err)
	}
	var md BadgerBlockMetadata
	_, err = md.UnmarshalMsg(bv)
	if err != nil {
		log.Fatal(err)
	}
	b := &Block{Id: id, backend: self,
		refCount: md.RefCount,
		Status:   BlockStatus(md.Status)}
	//log.Printf("b.GetBlockById %v", r)
	return b
}

func (self *BadgerBlockBackend) GetBlockIdByName(name string) string {
	bv, err := self.getKKValue([]byte("3"), []byte(name))
	if err == badger.ErrKeyNotFound {
		return ""
	}
	if err != nil {
		log.Fatal("get error:", err)
	}
	return string(bv)
}

func (self *BadgerBlockBackend) GetBytesAvailable() int {
	return -1
}

func (self *BadgerBlockBackend) GetBytesUsed() int {
	return -1
}

func (self *BadgerBlockBackend) SetInFlush(value bool) {
	if value {
		// Old transaction was read-only
		self.txn.Discard()

	} else {
		self.commit()
	}
	self.txn = self.db.NewTransaction(value)
}

func (self *BadgerBlockBackend) commit() {
	if err := self.txn.Commit(nil); err != nil {
		log.Fatal("commit:", err)
	}
}

func (self *BadgerBlockBackend) setKKValue(prefix, suffix, value []byte) {
	k := append(prefix, suffix...)
	if err := self.set(k, value); err != nil {
		log.Fatal("set", err)
	}
}

func (self *BadgerBlockBackend) delete(k []byte) error {
	err := self.txn.Delete(k)
	if err == badger.ErrTxnTooBig {
		self.commit()
		self.txn = self.db.NewTransaction(true)
		return self.delete(k)
	}
	return err
}

func (self *BadgerBlockBackend) set(k, v []byte) error {
	err := self.txn.Set(k, v)
	if err == badger.ErrTxnTooBig {
		self.commit()
		self.txn = self.db.NewTransaction(true)
		return self.set(k, v)
	}
	return err
}

func (self *BadgerBlockBackend) SetNameToBlockId(name, block_id string) {
	self.setKKValue([]byte("3"), []byte(name), []byte(block_id))
}

func (self *BadgerBlockBackend) StoreBlock(b *Block) {
	//log.Printf("StoreBlock %v", b)
	self.updateBlock(b)
	self.setKKValue([]byte("2"), []byte(b.Id), []byte(b.Data))
}

func (self *BadgerBlockBackend) updateBlock(b *Block) {
	md := BadgerBlockMetadata{Status: b.Status,
		RefCount: b.refCount}
	buf, err := md.MarshalMsg(nil)
	if err != nil {
		log.Fatal(err)
	}
	self.setKKValue([]byte("1"), []byte(b.Id), buf)
}

func (self *BadgerBlockBackend) UpdateBlock(b *Block) int {
	self.updateBlock(b)
	return 1
}

func (self *BadgerBlockBackend) Close() {
	self.txn.Discard()
	self.db.Close()
}
