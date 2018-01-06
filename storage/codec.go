/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2018 Markus Stenberg
 *
 * Created:       Sat Jan  6 00:13:13 2018 mstenber
 * Last modified: Sat Jan  6 02:22:13 2018 mstenber
 * Edit time:     8 min
 *
 */

package storage

import (
	"log"

	"github.com/fingon/go-tfhfs/codec"
)

type codecBackend struct {
	proxyBackend
	codec codec.Codec
}

func (self codecBackend) Init(backend Backend, codec codec.Codec) *codecBackend {
	self.backend = backend
	self.codec = codec
	return &self
}

func (self *codecBackend) GetBlockById(id string) *Block {
	b := self.backend.GetBlockById(id)
	if b != nil {
		b.Backend = self
	}
	if b == nil || b.Data.Get() == nil {
		return b
	}
	nb := *b
	nb.Data.Set(nil)
	return &nb
}

func (self *codecBackend) GetBlockData(bl *Block) []byte {
	data := self.backend.GetBlockData(bl)
	b, err := self.codec.DecodeBytes(data, []byte(bl.Id))
	if err != nil {
		log.Panic("Decoding failed", err)
	}
	return b
}

func (self *codecBackend) StoreBlock(bl *Block) {
	dp := bl.Data.Get()
	b, err := self.codec.EncodeBytes(*dp, []byte(bl.Id))
	if err != nil {
		log.Panic("Encoding failed", err)
	}
	bl2 := *bl
	bl2.Data.Set(&b)
	self.backend.StoreBlock(&bl2)
}
