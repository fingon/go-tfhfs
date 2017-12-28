/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2017 Markus Stenberg
 *
 * Created:       Mon Dec 25 01:15:39 2017 mstenber
 * Last modified: Thu Dec 28 14:36:52 2017 mstenber
 * Edit time:     7 min
 *
 */

package fs

type BlockDataType byte

const (
	BDT_NODE   BlockDataType = 1
	BDT_EXTENT BlockDataType = 2
)

type ObjectSubType byte

const (
	// should not occur in real world
	// (can be used as end-of-range marker given ino+1 + this OST)
	OST_NONE ObjectSubType = 0

	// value: INodeMeta
	OST_META ObjectSubType = 1

	// key: k (string->bytes), value: data (bytes)
	OST_XATTR ObjectSubType = 2

	// key: teahash.name, value: 8 byte inode
	OST_DIR_NAME2INODE ObjectSubType = 0x10

	// key: inode (dir it is in) . filename
	OST_FILE_INODEFILENAME ObjectSubType = 0x20
	// key: 8 byte offset, value: data block id (for data @ offset)
	OST_FILE_OFFSET2EXTENT ObjectSubType = 0x21
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
