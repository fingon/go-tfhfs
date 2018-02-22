/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2018 Markus Stenberg
 *
 * Created:       Wed Feb 21 17:20:58 2018 mstenber
 * Last modified: Thu Feb 22 10:54:32 2018 mstenber
 * Edit time:     3 min
 *
 */

package tree

import (
	"testing"

	"github.com/stvp/assert"
)

func TestLocationEntryEndecode(t *testing.T) {
	t.Parallel()
	le := LocationEntry{Offset: 42, Size: 7}
	le2 := NewLocationEntryFromKeySO(le.ToKeySO())
	assert.Equal(t, le.Offset, le2.Offset)
	assert.Equal(t, le.BlockSize(), le2.BlockSize())
}
