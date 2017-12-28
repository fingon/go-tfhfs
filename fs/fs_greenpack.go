/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2017 Markus Stenberg
 *
 * Created:       Mon Dec 25 01:15:39 2017 mstenber
 * Last modified: Fri Dec 29 00:33:03 2017 mstenber
 * Edit time:     9 min
 *
 */

package fs

type BlockDataType byte

const (
	BDT_NODE   BlockDataType = 1
	BDT_EXTENT BlockDataType = 2
)

type BlockSubType byte

const (
	// should not occur in real world
	// (can be used as end-of-range marker given ino+1 + this OST)
	BST_NONE BlockSubType = 0

	// value: INodeMeta
	BST_META BlockSubType = 1

	// key: k (string->bytes), value: data (bytes)
	BST_XATTR BlockSubType = 2

	// key: teahash.name, value: 8 byte inode
	BST_DIR_NAME2INODE BlockSubType = 0x10

	// key: inode (dir it is in) . filename
	BST_FILE_INODEFILENAME BlockSubType = 0x20
	// key: 8 byte offset, value: data block id (for data @ offset)
	BST_FILE_OFFSET2EXTENT BlockSubType = 0x21
)

type INodeMeta struct {
	// int64 st_ino = 1;
	// ^ part of key, not data
	StMode    int32
	StUid     int32
	StGid     int32
	StAtimeNs int64
	StCtimeNs int64
	StMtimeNs int64
	StSize    int64
	StNlink   int32
	// dynamic: st_rdev
	// static but from elsewhere: st_blksize

	// InPlaceData contains e.g. symlink target, mini-file
	// content; at most path-max-len (~ 1kb?)
	InPlaceData []byte
}
