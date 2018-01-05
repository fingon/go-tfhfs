/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2017 Markus Stenberg
 *
 * Created:       Sun Dec 17 22:20:08 2017 mstenber
 * Last modified: Fri Jan  5 12:07:08 2018 mstenber
 * Edit time:     63 min
 *
 */

package inmemory

import (
	"log"

	"github.com/fingon/go-tfhfs/mlog"
	"github.com/fingon/go-tfhfs/storage"
	"github.com/fingon/go-tfhfs/util"
)

// inMemoryBackend provides In-memory storage; data is always
// assumed to be available and is just stored in maps.
type inMemoryBackend struct {
	id2Block map[string]*storage.Block
	name2Id  map[string]string
	in_flush bool
	lock     util.MutexLocked
}

var _ storage.Backend = &inMemoryBackend{}

// Init makes the instance actually useful
func NewInMemoryBackend() storage.Backend {
	self := &inMemoryBackend{}
	self.id2Block = make(map[string]*storage.Block)
	self.name2Id = make(map[string]string)
	return self
}

func (self *inMemoryBackend) Close() {

}

func (self *inMemoryBackend) DeleteBlock(b *storage.Block) {
	mlog.Printf2("storage/inmemory", "im.DeleteBlock %x", b.Id)
	if !self.in_flush {
		log.Fatal("DeleteBlock outside flush")
	}
	delete(self.id2Block, b.Id)
}

func (self *inMemoryBackend) GetBlockData(b *storage.Block) []byte {
	return b.Data
}

func (self *inMemoryBackend) GetBlockById(id string) *storage.Block {
	defer self.lock.Locked()()
	return self.id2Block[id]
}

func (self *inMemoryBackend) GetBlockIdByName(name string) string {
	defer self.lock.Locked()()
	return self.name2Id[name]
}

func (self *inMemoryBackend) GetBytesAvailable() uint64 {
	return 0
}

func (self *inMemoryBackend) GetBytesUsed() uint64 {
	return 0
}

func (self *inMemoryBackend) SetInFlush(value bool) {
	if self.in_flush == value {
		log.Fatal("Same in flush value in SetInFlush")
	}
	self.in_flush = value
}

func (self *inMemoryBackend) SetNameToBlockId(name, block_id string) {
	defer self.lock.Locked()()
	if !self.in_flush {
		log.Fatal("SetNameToBlockId outside flush")
	}
	self.name2Id[name] = block_id
}

func (self *inMemoryBackend) StoreBlock(b *storage.Block) {
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

func (self *inMemoryBackend) UpdateBlock(b *storage.Block) int {
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
