/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2018 Markus Stenberg
 *
 * Created:       Mon Jan 15 18:39:20 2018 mstenber
 * Last modified: Thu Feb 22 10:06:27 2018 mstenber
 * Edit time:     20 min
 *
 */

package storage

import (
	"log"

	"github.com/fingon/go-tfhfs/util"
)

// NameInBlockBackend saves names in a single block. Unfortunately
// that design is somewhat vulnerable (as there is a time during which
// there are 0 name-blocks in db), so probably better design would be
// to use name-prefix and generation count. TBD, watch this space.
type NameInBlockBackend struct {
	mapName  string
	bb       BlockBackend
	namedMap *NameMapBlock
	lock     util.MutexLocked
	block    *Block
}

var _ NameBackend = &NameInBlockBackend{}

func (self *NameInBlockBackend) Init(mapName string, bb BlockBackend) {
	self.mapName = mapName
	self.bb = bb
}

func (self *NameInBlockBackend) getBlock() *NameMapBlock {
	if self.namedMap != nil {
		return self.namedMap
	}
	v := self.bb.GetBlockById(self.mapName)
	if v == nil {
		self.namedMap = &NameMapBlock{make(map[string]string)}
		return self.namedMap
	}
	self.block = v
	b := []byte(v.GetData())
	var nmb NameMapBlock
	_, err := nmb.UnmarshalMsg(b)
	if err != nil {
		log.Panic(err)
	}
	self.namedMap = &nmb
	return self.namedMap
}

func (self *NameInBlockBackend) GetBlockIdByName(name string) string {
	defer self.lock.Locked()()
	return self.getBlock().NameToBlockId[name]
}

func (self *NameInBlockBackend) SetNameToBlockId(name, block_id string) {
	defer self.lock.Locked()()
	block := self.getBlock()
	if block_id != "" {
		block.NameToBlockId[name] = block_id
	} else {
		delete(block.NameToBlockId, name)
	}
	if self.block != nil {
		self.bb.DeleteBlock(self.block)
	}
	b, err := self.namedMap.MarshalMsg(nil)
	if err != nil {
		log.Panic(err)
	}
	bl := &Block{Id: self.mapName}
	bl.Data.Set(&b)
	self.bb.StoreBlock(bl)
	self.block = bl
}
