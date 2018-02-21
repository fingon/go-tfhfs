/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2018 Markus Stenberg
 *
 * Created:       Fri Feb 16 10:17:18 2018 mstenber
 * Last modified: Fri Feb 16 11:00:44 2018 mstenber
 * Edit time:     7 min
 *
 */

package tree

type LocationEntry struct {
	Offset, Size uint64
}

type LocationSlice []LocationEntry

type BlockData struct {
	Location LocationSlice
	RefCount int32
	Status   uint8
}

type Superblock struct {
	Generation         uint64
	Used               uint64
	Size               uint64
	RootLocation       LocationSlice
	PendingAllocations LocationSlice
	PendingFrees       LocationSlice
}
