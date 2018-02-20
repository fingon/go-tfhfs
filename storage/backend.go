/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2018 Markus Stenberg
 *
 * Created:       Fri Jan  5 11:14:11 2018 mstenber
 * Last modified: Tue Feb 20 10:32:04 2018 mstenber
 * Edit time:     11 min
 *
 */

package storage

import "time"

type BackendConfiguration struct {
	// How much delay should there per asynchronous operation
	// (useful only for testing)
	DelayPerOp time.Duration

	// Directory to be used for storing backend data
	Directory string

	// ValueUpdateInterval describes how often cached values (e.g.
	// statfs stuff) are updated.
	ValueUpdateInterval time.Duration
}

// BlockBackend is subset of the storage Backend which deals with raw
// named + reference counted blocks.
type BlockBackend interface {
	// GetBlockData retrieves lazily (if need be) block data
	GetBlockData(b *Block) []byte

	// GetBlockById returns block by id or nil.
	GetBlockById(id string) *Block

	// DeleteBlock removes block from storage, and it MUST exist.
	DeleteBlock(b *Block)

	// StoreBlock adds new block to  It MUST NOT exist.
	StoreBlock(b *Block)

	// UpdateBlock updates block metadata in  It MUST exist.
	UpdateBlock(b *Block) int
}

// NameBackend is subset of storage Backend which deals with names.
type NameBackend interface {

	// GetBlockIdByName returns block id mapped to particular name.
	GetBlockIdByName(name string) string

	// SetBlockIdName sets the logical name to map to particular block id.
	SetNameToBlockId(name, block_id string)
}

// Backend is the shadow behind the throne; it actually handles the
// low-level operations of blocks. It provides an API that returns
// results that are consistent with the previous calls. How it does
// this in practise is left as an exercise to the implementor. There
// is no guarantee it will not be called from multiple goroutines at
// once, and it is again the problem of the implementor to ensure that
// the results are consistent.
type Backend interface {
	// Initialize the backend with the given configuration; this
	// is typically called only for real storage backends and not
	// interim ones (e.g. mapRunnerBackend, codecBackend)
	Init(config BackendConfiguration)

	// Flush is used to hint that currently is good time to
	// snapshot state, if any; storage is done with flushing its
	// current state so e.g. names and block hierarchies are most
	// consistent right now
	Flush()

	// Close the backend
	Close()

	// GetBytesAvailable returns number of bytes available.
	GetBytesAvailable() uint64

	// GetBytesUsed returns number of bytes used.
	GetBytesUsed() uint64

	BlockBackend
	NameBackend
}
