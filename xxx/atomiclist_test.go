/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2018 Markus Stenberg
 *
 * Created:       Thu Jan 25 14:19:32 2018 mstenber
 * Last modified: Thu Jan 25 14:22:19 2018 mstenber
 * Edit time:     3 min
 *
 */

package xxx

import (
	"testing"

	"github.com/stvp/assert"
)

func TestAtomicList(t *testing.T) {
	t.Parallel()
	s := "foo"
	v := XXXType(&s)
	l := XXXAtomicList{New: func() XXXType {
		return v
	}}
	v2 := l.Get()
	v3 := l.Get()
	assert.Equal(t, v2, v)
	assert.Equal(t, v3, v)
	l.Put(v2)
	l.Put(v3)
}
