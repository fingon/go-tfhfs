/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2018 Markus Stenberg
 *
 * Created:       Sat Jan  6 00:08:05 2018 mstenber
 * Last modified: Tue Feb 20 10:34:54 2018 mstenber
 * Edit time:     6 min
 *
 */

package storage

import "github.com/fingon/go-tfhfs/mlog"

type proxyBackend struct {
	BackendConfiguration
	Backend Backend
}

var _ Backend = &proxyBackend{}

// Semi-init with only backend setting
func (self proxyBackend) SetBackend(backend Backend) *proxyBackend {
	self.Backend = backend
	return &self
}

// Init makes the instance actually useful
func (self *proxyBackend) Init(config BackendConfiguration) {
	self.BackendConfiguration = config
	self.Backend.Init(config)
}

func (self *proxyBackend) Flush() {
	self.Backend.Flush()
}

func (self *proxyBackend) Close() {
	mlog.Printf2("storage/proxybackend", "proxying backend Close()")
	self.Backend.Close()
}

func (self *proxyBackend) DeleteBlock(b *Block) {
	self.Backend.DeleteBlock(b)
}

func (self *proxyBackend) GetBlockData(b *Block) []byte {
	return self.Backend.GetBlockData(b)
}

func (self *proxyBackend) GetBlockById(id string) *Block {
	bl := self.Backend.GetBlockById(id)
	if bl != nil {
		bl.Backend = self
	}
	return bl
}

func (self *proxyBackend) GetBlockIdByName(name string) string {
	return self.Backend.GetBlockIdByName(name)
}

func (self *proxyBackend) GetBytesAvailable() uint64 {
	return self.Backend.GetBytesAvailable()
}

func (self *proxyBackend) GetBytesUsed() uint64 {
	return self.Backend.GetBytesUsed()
}

func (self *proxyBackend) SetNameToBlockId(name, block_id string) {
	self.Backend.SetNameToBlockId(name, block_id)
}

func (self *proxyBackend) StoreBlock(b *Block) {
	self.Backend.StoreBlock(b)
}

func (self *proxyBackend) UpdateBlock(b *Block) int {
	return self.Backend.UpdateBlock(b)
}
