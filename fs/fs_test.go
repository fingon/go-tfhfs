/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2017 Markus Stenberg
 *
 * Created:       Thu Dec 28 14:31:48 2017 mstenber
 * Last modified: Thu Dec 28 14:35:31 2017 mstenber
 * Edit time:     3 min
 *
 */

package fs

import (
	"testing"

	"github.com/stvp/assert"
)

func TestFsTreeKey(t *testing.T) {
	t.Parallel()
	ino := uint64(42)
	ost := OST_META
	ostd := "foo"
	k := NewFsTreeKey(ino, ost, ostd)
	assert.Equal(t, k.Ino(), ino)
	assert.Equal(t, k.ObjectSubType(), ost)
	assert.Equal(t, k.SubTypeData(), ostd)
}
