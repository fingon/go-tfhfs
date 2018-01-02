/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2017 Markus Stenberg
 *
 * Created:       Fri Dec 29 00:34:54 2017 mstenber
 * Last modified: Tue Jan  2 10:22:19 2018 mstenber
 * Edit time:     15 min
 *
 */

package fs

import (
	"encoding/binary"
	"hash/fnv"

	"github.com/fingon/go-tfhfs/util"
)

type blockKey string

func (self blockKey) SubType() BlockSubType {
	return BlockSubType(self[blockSubTypeOffset])
}

func (self blockKey) Ino() uint64 {
	b := []byte(self[:inodeDataLength])
	return binary.BigEndian.Uint64(b)
}

func (self blockKey) SubTypeData() string {
	return string(self[inodeDataLength+1:])
}

func NewblockKey(ino uint64, st BlockSubType, data string) blockKey {
	b := util.ConcatBytes(util.Uint64Bytes(ino), []byte{byte(st)},
		[]byte(data))
	return blockKey(b)
}

func NewblockKeyDirFilename(ino uint64, filename string) blockKey {
	h := fnv.New32()
	h.Write([]byte(filename))
	n := h.Sum32()
	b0 := util.Uint32Bytes(n)
	b := util.ConcatBytes(b0, []byte(filename))
	return NewblockKey(ino, BST_DIR_NAME2INODE, string(b))
}

func NewblockKeyReverseDirFilename(ino, dirIno uint64, filename string) blockKey {
	b := util.ConcatBytes(util.Uint64Bytes(dirIno), []byte(filename))
	return NewblockKey(ino, BST_FILE_INODEFILENAME, string(b))
}

func NewblockKeyOffset(ino uint64, offset uint64) blockKey {
	offset = offset / dataExtentSize
	b := util.Uint64Bytes(offset)
	return NewblockKey(ino, BST_FILE_OFFSET2EXTENT, string(b))
}
