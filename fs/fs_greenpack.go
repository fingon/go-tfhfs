/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2017 Markus Stenberg
 *
 * Created:       Mon Dec 25 01:15:39 2017 mstenber
 * Last modified: Mon Dec 25 01:22:37 2017 mstenber
 * Edit time:     3 min
 *
 */

package fs

type ObjectSubType byte

const (
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
