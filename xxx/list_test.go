/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2018 Markus Stenberg
 *
 * Created:       Sun Jan  7 17:05:40 2018 mstenber
 * Last modified: Wed Jan 10 09:33:02 2018 mstenber
 * Edit time:     7 min
 *
 */

package xxx

import (
	"testing"

	"github.com/stvp/assert"
)

func Test(t *testing.T) {
	t.Parallel()

	l := YYYList{}
	v1 := YYYType(7)
	v2 := YYYType(13)
	v3 := YYYType(42)

	fun := func(fr bool) {
		if fr {
			l.PushFront(v1)
			l.PushFront(v2)
			l.PushFront(v3)
		} else {
			l.PushBack(v3)
			l.PushBack(v2)
			l.PushBack(v1)
		}

		assert.Equal(t, l.Front.Value, v3)
		assert.Equal(t, l.Front.Next.Value, v2)
		assert.Equal(t, l.Front.Next.Next.Value, v1)
		assert.Nil(t, l.Front.Next.Next.Next)

		assert.Equal(t, l.Back.Value, v1)
		assert.Equal(t, l.Back.Prev.Value, v2)
		assert.Equal(t, l.Back.Prev.Prev.Value, v3)
		assert.Nil(t, l.Back.Prev.Prev.Prev)
	}
	empty := func() {
		assert.True(t, l.Front != nil)
		assert.True(t, l.Back != nil)
		l.Remove(l.Front)
		l.Remove(l.Front)
		l.Remove(l.Front)
		assert.Nil(t, l.Front)
		assert.Nil(t, l.Back)
	}
	// First mallocs
	fun(true)
	empty()
	// Ensure it also works via freelist
	fun(false)
	empty()
	fun(true)
}
