/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2017 Markus Stenberg
 *
 * Created:       Thu Dec 28 14:31:48 2017 mstenber
 * Last modified: Fri Dec 29 00:34:00 2017 mstenber
 * Edit time:     3 min
 *
 */

package fs

import (
	"testing"

	"github.com/stvp/assert"
)

func TestBlockKey(t *testing.T) {
	t.Parallel()
	ino := uint64(42)
	bst := BST_META
	bstd := "foo"
	k := NewBlockKey(ino, bst, bstd)
	assert.Equal(t, k.Ino(), ino)
	assert.Equal(t, k.SubType(), bst)
	assert.Equal(t, k.SubTypeData(), bstd)
}
