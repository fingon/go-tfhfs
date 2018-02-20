/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2017 Markus Stenberg
 *
 * Created:       Fri Dec 29 00:34:54 2017 mstenber
 * Last modified: Wed Jan 24 11:49:15 2018 mstenber
 * Edit time:     36 min
 *
 */

package fs

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"hash/fnv"

	"github.com/fingon/go-tfhfs/ibtree"
	"github.com/fingon/go-tfhfs/util"
)

type BlockKey string

func (self BlockKey) IB() ibtree.Key {
	return ibtree.Key(self)
}

func (self BlockKey) String() string {
	return fmt.Sprintf("bkey{ino:%v,subtype:%v,subtypedata:%x}", self.Ino(), self.SubType(), self.SubTypeData())

}

func (self BlockKey) SubType() BlockSubType {
	if len(self) <= blockSubTypeOffset {
		return 0
	}
	return BlockSubType(self[blockSubTypeOffset])
}

func (self BlockKey) Ino() uint64 {
	if len(self) < inodeDataLength {
		return 0
	}
	b := []byte(self[:inodeDataLength])
	return binary.BigEndian.Uint64(b)
}

func (self BlockKey) SubTypeData() string {
	return string(self[inodeDataLength+1:])
}

func (self BlockKey) Filename() string {
	if self.SubType() == BST_DIR_NAME2INODE {
		return self.SubTypeData()[filenameHashSize:]
	}
	return ""
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
	b0 := util.Uint32Bytes(n)
	b := util.ConcatBytes(b0, []byte(filename))
	return NewBlockKey(ino, BST_DIR_NAME2INODE, string(b))
}

func NewBlockKeyReverseDirFilename(ino, dirIno uint64, filename string) BlockKey {
	b := util.ConcatBytes(util.Uint64Bytes(dirIno), []byte(filename))
	return NewBlockKey(ino, BST_FILE_INODEFILENAME, string(b))
}

func NewBlockKeyOffset(ino uint64, offset uint64) BlockKey {
	offset = offset / dataExtentSize
	b := util.Uint64Bytes(offset)
	return NewBlockKey(ino, BST_FILE_OFFSET2EXTENT, string(b))
}

func NewBlockKeyNameBlock(name, id string) BlockKey {
	h := fnv.New64()
	h.Write([]byte(name))
	n := h.Sum64()
	b := make([]byte, len(name)+len(id))
	copy(b, name)
	copy(b[len(name):], id)
	return NewBlockKey(n, BST_NAMEHASH_NAME_BLOCK, string(b))
}
func NewBlockKeyNameEnd(name string) BlockKey {
	h := fnv.New64()
	nb := []byte(name)
	h.Write(nb)
	n := h.Sum64()
	b := util.ConcatBytes(nb, bytes.Repeat([]byte{255}, 33))
	return NewBlockKey(n, BST_NAMEHASH_NAME_BLOCK, string(b))
}
