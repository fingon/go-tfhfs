/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2017 Markus Stenberg
 *
 * Created:       Sun Dec 17 22:20:08 2017 mstenber
 * Last modified: Sat Dec 23 14:21:11 2017 mstenber
 * Edit time:     54 min
 *
 */

package storage

// InMemoryBlockBackend provides In-memory storage; data is always
// assumed to be available and is just stored in maps.
type InMemoryBlockBackend struct {
	id2Block map[string]*Block
	name2Id  map[string]string
}

var _ BlockBackend = &InMemoryBlockBackend{}

// Init makes the instance actually useful
func (self InMemoryBlockBackend) Init() *InMemoryBlockBackend {
	self.id2Block = make(map[string]*Block)
	self.name2Id = make(map[string]string)
	return &self
}

func (self *InMemoryBlockBackend) DeleteBlock(b *Block) {
	delete(self.id2Block, b.Id)
}

func (self *InMemoryBlockBackend) GetBlockData(b *Block) string {
	return b.Data
}

func (self *InMemoryBlockBackend) GetBlockById(id string) *Block {
	return self.id2Block[id]
}

func (self *InMemoryBlockBackend) GetBlockIdByName(name string) string {
	return self.name2Id[name]
}

func (self *InMemoryBlockBackend) GetBytesAvailable() int {
	return -1
}

func (self *InMemoryBlockBackend) GetBytesUsed() int {
	return -1
}

func (self *InMemoryBlockBackend) SetNameToBlockId(name, block_id string) {
	self.name2Id[name] = block_id
}

func (self *InMemoryBlockBackend) StoreBlock(b *Block) {
	self.id2Block[b.Id] = b
}

func (self *InMemoryBlockBackend) UpdateBlock(b *Block) int {
	return 1
}
