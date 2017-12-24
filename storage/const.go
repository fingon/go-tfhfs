/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2017 Markus Stenberg
 *
 * Created:       Sun Dec 24 08:26:02 2017 mstenber
 * Last modified: Sun Dec 24 08:55:18 2017 mstenber
 * Edit time:     6 min
 *
 */
package storage

const (
	BlockStatus_UNSET BlockStatus = iota
	BlockStatus_NORMAL
	BlockStatus_MISSING
	BlockStatus_WANT_NORMAL
	BlockStatus_WEAK
	BlockStatus_WANT_WEAK
)

const (
	BlockType_UNSET BlockType = iota
	BlockType_TREE_NODE
	BlockType_FILE_EXTENT
)

const (
	CompressionType_UNSET CompressionType = iota
	CompressionType_PLAIN
	CompressionType_LZ4
)
