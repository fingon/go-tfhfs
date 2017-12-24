/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2017 Markus Stenberg
 *
 * Created:       Sun Dec 24 08:37:14 2017 mstenber
 * Last modified: Sun Dec 24 08:52:40 2017 mstenber
 * Edit time:     9 min
 *
 */

// These structs are used by Storage
// see const.go for types used here.
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
	IV            []byte `zid:"0"` // used for AES GCM
	EncryptedData []byte `zid:"1"` // AES GCM encrypted PlainBlock
}

type BlockType byte
type CompressionType byte

type PlainBlock struct {
	BlockType       BlockType       `zid:"0"`
	CompressionType CompressionType `zid:"1"`
	RawData         []byte          `zid:"2"`
	// if type=FILE_EXTENT, contains raw file data
	// if type=NODE, contains TreeNode (see below)
}
