/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2017 Markus Stenberg
 *
 * Created:       Sun Dec 17 22:20:08 2017 mstenber
 * Last modified: Fri Jan  5 02:49:51 2018 mstenber
 * Edit time:     62 min
 *
 */

package storage

import (
	"log"

	"github.com/fingon/go-tfhfs/mlog"
	"github.com/fingon/go-tfhfs/util"
)

// InMemoryBlockBackend provides In-memory storage; data is always
// assumed to be available and is just stored in maps.
type InMemoryBlockBackend struct {
	id2Block map[string]*Block
	name2Id  map[string]string
	in_flush bool
	lock     util.MutexLocked
}

var _ BlockBackend = &InMemoryBlockBackend{}

// Init makes the instance actually useful
func (self InMemoryBlockBackend) Init() *InMemoryBlockBackend {
	self.id2Block = make(map[string]*Block)
	self.name2Id = make(map[string]string)
	return &self
}

func (self *InMemoryBlockBackend) DeleteBlock(b *Block) {
	mlog.Printf2("storage/inmemory", "im.DeleteBlock %x", b.Id)
	if !self.in_flush {
		log.Fatal("DeleteBlock outside flush")
	}
	delete(self.id2Block, b.Id)
}

func (self *InMemoryBlockBackend) GetBlockData(b *Block) []byte {
	return b.Data
}

func (self *InMemoryBlockBackend) GetBlockById(id string) *Block {
	defer self.lock.Locked()()
	return self.id2Block[id]
}

func (self *InMemoryBlockBackend) GetBlockIdByName(name string) string {
	defer self.lock.Locked()()
	return self.name2Id[name]
}

func (self *InMemoryBlockBackend) GetBytesAvailable() uint64 {
	return 0
}

func (self *InMemoryBlockBackend) GetBytesUsed() uint64 {
	return 0
}

func (self *InMemoryBlockBackend) SetInFlush(value bool) {
	if self.in_flush == value {
		log.Fatal("Same in flush value in SetInFlush")
	}
	self.in_flush = value
}

func (self *InMemoryBlockBackend) SetNameToBlockId(name, block_id string) {
	defer self.lock.Locked()()
	if !self.in_flush {
		log.Fatal("SetNameToBlockId outside flush")
	}
	self.name2Id[name] = block_id
}

func (self *InMemoryBlockBackend) StoreBlock(b *Block) {
	defer self.lock.Locked()()
	if !self.in_flush {
		log.Fatal("StoreBlock outside flush")
	}
	if self.id2Block[b.Id] != nil {
		log.Fatal("Existing block id in StoreBlock")
	}
	mlog.Printf2("storage/inmemory", "im.StoreBlock %x", b.Id)
	self.id2Block[b.Id] = b
}

func (self *InMemoryBlockBackend) UpdateBlock(b *Block) int {
	defer self.lock.Locked()()
	if !self.in_flush {
		log.Fatal("UpdateBlock outside flush")
	}
	if self.id2Block[b.Id] == nil {
		log.Fatal("Non-existent block id in StoreBlock")
	}
	mlog.Printf2("storage/inmemory", "im.UpdateBlock %x", b.Id)
	return 1
}

func (self *InMemoryBlockBackend) Close() {

}
