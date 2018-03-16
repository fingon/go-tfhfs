/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2017 Markus Stenberg
 *
 * Created:       Fri Dec 29 09:04:44 2017 mstenber
 * Last modified: Fri Mar 16 11:41:03 2018 mstenber
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

func TestIMin(t *testing.T) {
	t.Parallel()
	assert.Equal(t, IMin(1, 2, 3), 1)
	assert.Equal(t, IMin(3, 2, 1), 1)

}

func TestIMax(t *testing.T) {
	t.Parallel()
	assert.Equal(t, IMax(1, 2, 3), 3)
	assert.Equal(t, IMax(3, 2, 1), 3)
}
