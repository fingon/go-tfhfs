/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2018 Markus Stenberg
 *
 * Created:       Tue Jan  9 18:32:41 2018 mstenber
 * Last modified: Tue Jan  9 20:00:51 2018 mstenber
 * Edit time:     1 min
 *
 */

package ibtree

import (
	"crypto/sha256"
	"log"
)

// DummyBackend is minimal in-memory backend that can be used for
// testing purposes.
type DummyBackend struct {
	h2nd  map[BlockId][]byte
	loads int
	saves int
}

func (self DummyBackend) Init() *DummyBackend {
	self.h2nd = make(map[BlockId][]byte)
	return &self
}

func (self *DummyBackend) LoadNode(id BlockId) *IBNodeData {
	self.loads++
	// Create new copy of IBNodeData WITHOUT childNode's set
	nd := &IBNodeData{}
	_, err := nd.UnmarshalMsg(self.h2nd[id])
	if err != nil {
		log.Panic(err)
	}
	return nd
}

func (self *DummyBackend) SaveNode(nd *IBNodeData) BlockId {
	b, _ := nd.MarshalMsg(nil)
	h := sha256.Sum256(b)
	bid := BlockId(h[:])
	self.h2nd[bid] = b
	nd.CheckNodeStructure()
	return bid
}
