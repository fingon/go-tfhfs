/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2018 Markus Stenberg
 *
 * Created:       Wed Feb 21 15:19:19 2018 mstenber
 * Last modified: Wed Feb 21 15:20:34 2018 mstenber
 * Edit time:     1 min
 *
 */

package tree

import (
	"encoding/binary"

	"github.com/fingon/go-tfhfs/ibtree"
	"github.com/fingon/go-tfhfs/util"
)

// ToKeySO converts LocationEntry to ibtree.Key with size, offset order
func (self LocationEntry) ToKeySO() ibtree.Key {
	return ibtree.Key(util.ConcatBytes(util.Uint64Bytes(self.Size),
		util.Uint64Bytes(self.Offset)))
}

// ToKeyOS converts LocationEntry to ibtree.Key with offset, size order
func (self LocationEntry) ToKeyOS() ibtree.Key {
	return ibtree.Key(util.ConcatBytes(util.Uint64Bytes(self.Offset),
		util.Uint64Bytes(self.Size)))
}

// NewLocationEntryFromKeySO decodes ibtree.Key with size, offset order
func NewLocationEntryFromKeySO(key ibtree.Key) LocationEntry {
	b := []byte(key)
	s := binary.BigEndian.Uint64(b)
	o := binary.BigEndian.Uint64(b[8:])
	return LocationEntry{Size: s, Offset: o}
}
