/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2017 Markus Stenberg
 *
 * Created:       Fri Dec 29 00:34:54 2017 mstenber
 * Last modified: Fri Dec 29 09:11:27 2017 mstenber
 * Edit time:     9 min
 *
 */

package fs

import (
	"encoding/binary"
	"hash/fnv"

	"github.com/fingon/go-tfhfs/util"
)

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
	b := util.ConcatBytes(util.Uint64Bytes(ino), []byte{byte(st)},
		[]byte(data))
	return BlockKey(b)
}

func NewBlockKeyDirFilename(ino uint64, filename string) BlockKey {
	h := fnv.New32()
	h.Write([]byte(filename))
	n := h.Sum32()
	b := util.ConcatBytes(util.Uint32Bytes(n), []byte(filename))
	return NewBlockKey(ino, BST_DIR_NAME2INODE, string(b))
}