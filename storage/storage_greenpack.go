/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2017 Markus Stenberg
 *
 * Created:       Sun Dec 24 08:37:14 2017 mstenber
 * Last modified: Thu Feb  7 09:49:20 2019 mstenber
 * Edit time:     22 min
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

type BlockStatus byte

const (
	BS_UNSET BlockStatus = iota

	// Has references on based on data, data present
	BS_NORMAL

	// Has references on based on data, data gone
	BS_MISSING

	// No references on based on data, no data
	BS_WANT_NORMAL

	// No references on based on data, data present
	BS_WEAK

	// No references on based on data, no data
	BS_WANT_WEAK
)

type BlockMetadata struct {
	// RefCount is the non-negative number of references to a
	// block _on disk_ (or what should be on disk).
	RefCount int32 `zid:"0"`

	// Status describes the desired behavior of sub-references and
	// availability of data of a block.
	Status BlockStatus `zid:"1"`
}

type NameMapBlock struct {
	NameToBlockId map[string]string `zid:"0"`
}
