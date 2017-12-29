/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2017 Markus Stenberg
 *
 * Created:       Fri Dec 29 09:04:44 2017 mstenber
 * Last modified: Fri Dec 29 09:05:17 2017 mstenber
 * Edit time:     0 min
 *
 */

package util

import (
	"testing"

	"github.com/stvp/assert"
)

func TestConcatBytes(t *testing.T) {
	t.Parallel()

	assert.Equal(t, ConcatBytes([]byte("foo"), []byte("bar")), []byte("foobar"))
}
