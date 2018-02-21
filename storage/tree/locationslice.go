/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2018 Markus Stenberg
 *
 * Created:       Wed Feb 21 15:21:20 2018 mstenber
 * Last modified: Wed Feb 21 15:45:09 2018 mstenber
 * Edit time:     6 min
 *
 */

package tree

import (
	"github.com/fingon/go-tfhfs/ibtree"
	"github.com/fingon/go-tfhfs/mlog"
)

const locationEntryEncodedLength = 16

func (self *LocationSlice) ToBlockId() ibtree.BlockId {
	b := make([]byte, len(*self)*locationEntryEncodedLength)
	for ofs, le := range *self {
		s := le.ToKeySO()
		bofs := ofs * locationEntryEncodedLength
		copy(b[bofs:bofs+locationEntryEncodedLength], []byte(s))
	}
	return ibtree.BlockId(b)
}

func NewLocationSliceFromBlockId(bid ibtree.BlockId) LocationSlice {
	b := []byte(bid)
	if len(b)%locationEntryEncodedLength != 0 {
		mlog.Panicf("Invalid BlockId: %x (len %d)", b, len(b))
	}
	ls := make(LocationSlice, 0, len(bid)/locationEntryEncodedLength)
	for ofs := 0; ofs < len(b); ofs += locationEntryEncodedLength {
		le := NewLocationEntryFromKeySO(ibtree.Key(b[ofs : ofs+locationEntryEncodedLength]))
		ls = append(ls, le)
	}
	return ls
}
