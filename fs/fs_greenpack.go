/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2017 Markus Stenberg
 *
 * Created:       Mon Dec 25 01:15:39 2017 mstenber
 * Last modified: Thu Feb  7 09:49:03 2019 mstenber
 * Edit time:     27 min
 *
 */

package fs

import "github.com/fingon/go-tfhfs/ibtree"

const (
	BDT_EXTENT ibtree.BlockDataType = 0x42
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

	// should not occur in real world
	// (can be used as end-of-range marker)
	BST_LAST = 0x2f

	// value: FsData
	// (this should be only in root inode)
	//BST_FS_DATA = 0x30

	BST_NAMEHASH_NAME_BLOCK BlockSubType = 0x30
)

type InodeMetaData struct {
	// int64 st_ino = 1;
	// ^ part of key, not data
	StMode    uint32 `zid:"0"`
	StRdev    uint32 `zid:"1"`
	StUid     uint32 `zid:"2"`
	StGid     uint32 `zid:"3"`
	StAtimeNs uint64 `zid:"4"`
	StCtimeNs uint64 `zid:"5"`
	StMtimeNs uint64 `zid:"6"`
	StSize    uint64 `zid:"7"`
	StNlink   uint32 `zid:"8"`

	// Non-visible things

	// What is ino of our parent (directory-only)
	ParentIno uint64 `zid:"9"`
}

type InodeMeta struct {
	InodeMetaData
	Data []byte `zid:"10"`

	// dynamic: st_rdev
	// static but from elsewhere: st_blksize

	// Data contains e.g. symlink target, mini-file
	// content; at most path-max-len (~ 1kb?)
}

//type FsData struct {
// Eventually can stick fs-data in here
// notably: stuff for statfs?
//}
