/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2018 Markus Stenberg
 *
 * Created:       Wed Feb 21 17:20:58 2018 mstenber
 * Last modified: Wed Feb 21 17:21:42 2018 mstenber
 * Edit time:     1 min
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
	assert.Equal(t, le, NewLocationEntryFromKeySO(le.ToKeySO()))
}
