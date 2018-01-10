/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2018 Markus Stenberg
 *
 * Created:       Wed Jan 10 09:22:12 2018 mstenber
 * Last modified: Wed Jan 10 11:21:54 2018 mstenber
 * Edit time:     20 min
 *
 */

package storage

import (
	"log"

	"github.com/fingon/go-tfhfs/util"
)

type mapRunnerBackend struct {
	proxyBackend
	mr util.MapRunner
}

var _ Backend = &mapRunnerBackend{}

func (self *mapRunnerBackend) Close() {
	self.mr.Close()
	log.Printf("MapRunnerBackend: %d queued, %d ran", self.mr.Queued, self.mr.Ran)
	self.Backend.Close()
}

func (self *mapRunnerBackend) runWithBlock(b *Block, cb func()) {
	b.addStorageRefCount(1)
	self.mr.Run(b.Id, cb)
	b.addStorageRefCount(-1)
}

func (self *mapRunnerBackend) DeleteBlock(b *Block) {
	b = b.copy()
	self.runWithBlock(b, func() {
		self.Backend.DeleteBlock(b)
	})
}

func (self *mapRunnerBackend) GetBlockData(b *Block) []byte {
	var fut util.ByteSliceFuture
	self.runWithBlock(b, func() {
		fut.Set(self.Backend.GetBlockData(b))
	})
	return fut.Get()
}

func (self *mapRunnerBackend) GetBlockById(id string) *Block {
	var fut BlockPointerFuture
	self.mr.Run(id, func() {
		bl := self.Backend.GetBlockById(id)
		if bl != nil {
			bl.Backend = self
		}
		fut.Set(bl)
	})
	return fut.Get()
}

func (self *mapRunnerBackend) StoreBlock(b *Block) {
	b = b.copy()
	self.runWithBlock(b, func() {
		self.Backend.StoreBlock(b)
	})
}

func (self *mapRunnerBackend) UpdateBlock(b *Block) int {
	b = b.copy()
	self.runWithBlock(b, func() {
		self.Backend.UpdateBlock(b)
	})
	return 1
}
