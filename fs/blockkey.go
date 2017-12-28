/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2017 Markus Stenberg
 *
 * Created:       Fri Dec 29 00:34:54 2017 mstenber
 * Last modified: Fri Dec 29 00:35:08 2017 mstenber
 * Edit time:     0 min
 *
 */

package fs

import "encoding/binary"

type BlockKey string

func (self BlockKey) SubType() BlockSubType {
	return BlockSubType(self[blockSubTypeOffset])
}

func (self BlockKey) Ino() uint64 {
	b := []byte(self[:inodeDataLength])
	return binary.BigEndian.Uint64(b)
}

func (self BlockKey) SubTypeData() string {
	return string(self[inodeDataLength+1:])
}

func NewBlockKey(ino uint64, st BlockSubType, data string) BlockKey {
	b := make([]byte, inodeDataLength+1, inodeDataLength+1+len(data))
	binary.BigEndian.PutUint64(b, ino)
	b[inodeDataLength] = byte(st)
	b = append(b, []byte(data)...)
	return BlockKey(b)
}
