/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2018 Markus Stenberg
 *
 * Created:       Wed Mar 21 11:23:33 2018 mstenber
 * Last modified: Wed Mar 21 11:24:40 2018 mstenber
 * Edit time:     0 min
 *
 */

package util

import (
	"testing"

	"github.com/stvp/assert"
)

func TestAtomicInt(t *testing.T) {
	t.Parallel()
	var ai AtomicInt
	assert.Equal(t, ai.GetInt(), 0)
	ai.AddInt(1)
	assert.Equal(t, ai.Get(), int64(1))
	ai.SetInt(32)
	assert.Equal(t, ai.GetInt(), 32)
}
