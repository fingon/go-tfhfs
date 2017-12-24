/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2017 Markus Stenberg
 *
 * Created:       Sun Dec 24 08:37:14 2017 mstenber
 * Last modified: Sun Dec 24 13:23:07 2017 mstenber
 * Edit time:     17 min
 *
 */

// These structs are used by Storage that are actually persisted to
// disk.

package storage

/////////////////////////////////////////////////////////////////////////////

// Block layer

// Stored outside the raw block itself (in e.g. name)
// - block id (= index; sha256 hash of raw_data)
// - status

// need index by both as well was the raw data, so need at least:
//
// - 0-refcnt block ids (in key)
//
// - status -> block id (both in kv db key, no value)
//
// - block id (key) -> status + refcnt (value)
//
// - block id (key) ->  actual content (value = EncryptedBlock message)
// (Simpler versions may simply be value = Block message).

type EncryptedBlock struct {
	// IV used for AES GCM
	IV []byte `zid:"0"`

	// EncryptedData is AES GCM encrypted PlainBlock
	EncryptedData []byte `zid:"1"`
}

type BlockType byte

const (
	BlockType_UNSET BlockType = iota

	// TreeNode struct encoded within
	BlockType_TREE_NODE

	// Raw file data encoded within
	BlockType_FILE_EXTENT
)

type CompressionType byte

const (
	CompressionType_UNSET CompressionType = iota

	// The data has not been compressed.
	CompressionType_PLAIN

	// The data is compressed with LZ4.
	CompressionType_LZ4
)

type PlainBlock struct {
	// BlockType describes the type of data within.
	BlockType BlockType `zid:"0"`

	// CompressionType describes how the data has been compressed.
	CompressionType CompressionType `zid:"1"`

	// RawData is the raw data of the particular BlockType.
	RawData []byte `zid:"2"`
}

type BlockStatus byte

const (
	BlockStatus_UNSET BlockStatus = iota

	// Has references on based on data, data present
	BlockStatus_NORMAL

	// Has references on based on data, data gone
	BlockStatus_MISSING

	// No references on based on data, no data
	BlockStatus_WANT_NORMAL

	// No references on based on data, data present
	BlockStatus_WEAK

	// No references on based on data, no data
	BlockStatus_WANT_WEAK
)

type BlockMetadata struct {
	// RefCount is the non-negative number of references to a
	// block _on disk_ (or what should be on disk).
	RefCount int `zid:"0"`

	// Status describes the desired behavior of sub-references and
	// availability of data of a block.
	Status BlockStatus `zid:"1"`
}
