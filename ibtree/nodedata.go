/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2018 Markus Stenberg
 *
 * Created:       Wed Feb 21 15:29:23 2018 mstenber
 * Last modified: Wed Feb 21 15:34:47 2018 mstenber
 * Edit time:     1 min
 *
 */

package ibtree

import (
	"fmt"
	"log"

	"github.com/fingon/go-tfhfs/mlog"
)

type BlockDataType byte

const (
	BDT_NODE BlockDataType = 7
)

func (self *NodeData) String() string {
	return fmt.Sprintf("ibnd{%p}", self)
}

// CheckNodeStructure allows sanity checking a node's content (should not be really used except for debugging)
func (self *NodeData) CheckNodeStructure() {
	for i, c := range self.Children {
		if c == nil {
			mlog.Panicf("tree broke: nil child at [%d] of %v", i, self)
		}
		if c.Key == "" {
			mlog.Panicf("tree broke: empty key at [%d] of %v", i, self)
		}
		if i > 0 {
			k0 := self.Children[i-1].Key
			if k0 >= c.Key {
				mlog.Panicf("tree broke: '%x'[%d] >= '%x'[%d]", k0, i-1, c.Key, i)
			}
		}
	}
}

func (self *NodeData) ToBytes() []byte {
	bb := make([]byte, self.Msgsize()+1)
	bb[0] = byte(BDT_NODE)
	b, err := self.MarshalMsg(bb[1:1])
	if err != nil {
		log.Panic(err)
	}
	return bb[0 : 1+len(b)]
}

func NewNodeDataFromBytes(bd []byte) *NodeData {
	dt := BlockDataType(bd[0])
	if dt != BDT_NODE {
		mlog.Printf("BytesToNodeData - wrong dt:%v", dt)
		return nil
	}
	nd := &NodeData{}
	_, err := nd.UnmarshalMsg(bd[1:])
	if err != nil {
		log.Panic(err)
	}
	return nd
}
