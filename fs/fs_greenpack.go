/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2017 Markus Stenberg
 *
 * Created:       Mon Dec 25 01:15:39 2017 mstenber
 * Last modified: Fri Dec 29 13:01:33 2017 mstenber
 * Edit time:     18 min
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

	// value: InodeMeta
	BST_META BlockSubType = 1

	// key: k (string->bytes), value: data (bytes)
	BST_XATTR BlockSubType = 2

	// key: fnvhash.name, value: 8 byte inode
	BST_DIR_NAME2INODE BlockSubType = 0x10

	// key: inode (dir it is in) . filename
	BST_FILE_INODEFILENAME BlockSubType = 0x20
	// key: 8 byte offset, value: data block id (for data @ offset)
	BST_FILE_OFFSET2EXTENT BlockSubType = 0x21

	// value: FsData
	// (this should be only in root inode)
	//BST_FS_DATA = 0x30
)

type InodeMetaData struct {
	// int64 st_ino = 1;
	// ^ part of key, not data
	StMode    uint32
	StUid     uint32
	StGid     uint32
	StAtimeNs uint64
	StCtimeNs uint64
	StMtimeNs uint64
	StSize    uint64
	StNlink   uint32
	Nchildren uint32
}

type InodeMeta struct {
	InodeMetaData
	// dynamic: st_rdev
	// static but from elsewhere: st_blksize

	// Data contains e.g. symlink target, mini-file
	// content; at most path-max-len (~ 1kb?)
	Data []byte
}

//type FsData struct {
// Eventually can stick fs-data in here
// notably: stuff for statfs?
//}
