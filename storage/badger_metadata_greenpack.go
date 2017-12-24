/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2017 Markus Stenberg
 *
 * Created:       Sun Dec 24 07:49:51 2017 mstenber
 * Last modified: Sun Dec 24 08:55:17 2017 mstenber
 * Edit time:     3 min
 *
 */

package storage

type BlockStatus byte

type BadgerBlockMetadata struct {
	RefCount int         `zid:"0"`
	Status   BlockStatus `zid:"1"`
}
