/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2018 Markus Stenberg
 *
 * Created:       Thu Jan  4 11:49:21 2018 mstenber
 * Last modified: Thu Jan  4 12:03:24 2018 mstenber
 * Edit time:     10 min
 *
 */

// xxx package contains things that are sed-able to be typesafe,
// specific instantations of the datastructures.
//
// Naming scheme:
// - xxx is package name (must be replaced)
// - uppercase XXXType, YYYType are type names where applicable.
// - XXX is used as part of class name etc.
package xxx

import (
	"testing"

	"github.com/stvp/assert"
)

func TestAtomicPointer(t *testing.T) {
	t.Parallel()

	s := "foo"
	v := XXXType(&s)

	s2 := "bar"
	v2 := XXXType(&s2)

	s3 := "bar"
	v3 := XXXType(&s3)

	// ensure nil default value
	val := XXXAtomicPointer{}
	assert.Nil(t, val.Get())

	// set to v
	val.Set(v)
	assert.Equal(t, val.Get(), v)

	// ensure setting to same value works
	r := val.SetIfEqualTo(v, v)
	assert.True(t, r)
	assert.Equal(t, val.Get(), v)

	// v3 -> v2 (won't work, stays v)
	r = val.SetIfEqualTo(v2, v3)
	assert.True(t, !r)
	assert.Equal(t, val.Get(), v)

	// v -> v2
	r = val.SetIfEqualTo(v2, v)
	assert.True(t, r)
	assert.Equal(t, val.Get(), v2)
}
