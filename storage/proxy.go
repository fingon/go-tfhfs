/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2018 Markus Stenberg
 *
 * Created:       Sat Jan  6 00:08:05 2018 mstenber
 * Last modified: Sat Jan  6 00:11:39 2018 mstenber
 * Edit time:     3 min
 *
 */

package storage

type proxyBackend struct {
	backend Backend
}

var _ Backend = &proxyBackend{}

// Init makes the instance actually useful
func (self proxyBackend) Init(backend Backend) *proxyBackend {
	self.backend = backend
	return &self
}

func (self *proxyBackend) Close() {
	self.backend.Close()
}

func (self *proxyBackend) DeleteBlock(b *Block) {
	self.backend.DeleteBlock(b)
}

func (self *proxyBackend) GetBlockData(b *Block) []byte {
	return self.backend.GetBlockData(b)
}

func (self *proxyBackend) GetBlockById(id string) *Block {
	return self.backend.GetBlockById(id)
}

func (self *proxyBackend) GetBlockIdByName(name string) string {
	return self.backend.GetBlockIdByName(name)
}

func (self *proxyBackend) GetBytesAvailable() uint64 {
	return self.backend.GetBytesAvailable()
}

func (self *proxyBackend) GetBytesUsed() uint64 {
	return self.backend.GetBytesUsed()
}

func (self *proxyBackend) SetNameToBlockId(name, block_id string) {
	self.backend.SetNameToBlockId(name, block_id)
}

func (self *proxyBackend) StoreBlock(b *Block) {
	self.backend.StoreBlock(b)
}

func (self *proxyBackend) UpdateBlock(b *Block) int {
	return self.backend.UpdateBlock(b)
}
