/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2018 Markus Stenberg
 *
 * Created:       Fri Feb 16 10:17:18 2018 mstenber
 * Last modified: Thu Feb  7 09:53:58 2019 mstenber
 * Edit time:     11 min
 *
 */

package tree

import "github.com/fingon/go-tfhfs/storage"

type LocationEntry struct {
	Offset uint64 `zid:"0"`
	Size   uint64 `zid:"1"`
}

type LocationSlice []LocationEntry

type BlockData struct {
	storage.BlockMetadata `zid:"0"`
	Location              LocationSlice `zid:"1"`
}

type OpEntry struct {
	Location LocationEntry `zid:"0"`
	Free     bool          `zid:"1"`
}

type OpSlice []OpEntry

type Superblock struct {
	Generation      uint64        `zid:"0"`
	BytesUsed       uint64        `zid:"1"`
	BytesTotal      uint64        `zid:"2"`
	RootLocation    LocationSlice `zid:"3"`
	Pending         OpSlice       `zid:"4"`
	PendingLocation LocationSlice `zid:"5"`
}
