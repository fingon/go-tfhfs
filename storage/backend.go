/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2018 Markus Stenberg
 *
 * Created:       Fri Jan  5 11:14:11 2018 mstenber
 * Last modified: Fri Jan  5 11:22:23 2018 mstenber
 * Edit time:     4 min
 *
 */

package storage

// Backend is the shadow behind the throne; it actually handles the
// low-level operations of blocks. It provides an API that returns
// results that are consistent with the previous calls. How it does
// this in practise is left as an exercise to the implementor.
type Backend interface {
	// Close the backend
	Close()

	// Getters

	// GetBlockData retrieves lazily (if need be) block data
	GetBlockData(b *Block) []byte

	// GetBlockById returns block by id or nil.
	GetBlockById(id string) *Block

	// GetBlockIdByName returns block id mapped to particular name.
	GetBlockIdByName(name string) string

	// GetBytesAvailable returns number of bytes available.
	GetBytesAvailable() uint64

	// GetBytesUsed returns number of bytes used.
	GetBytesUsed() uint64

	// Setters

	// DeleteBlock removes block from storage, and it MUST exist.
	DeleteBlock(b *Block)

	// SetBlockIdName sets the logical name to map to particular block id.
	SetNameToBlockId(name, block_id string)

	// StoreBlock adds new block to  It MUST NOT exist.
	StoreBlock(b *Block)

	// UpdateBlock updates block metadata in  It MUST exist.
	UpdateBlock(b *Block) int
}
