/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2018 Markus Stenberg
 *
 * Created:       Sun Jan  7 17:05:40 2018 mstenber
 * Last modified: Sun Jan  7 17:14:20 2018 mstenber
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

	l := XXXList{}
	s1 := "s1"
	s2 := "s2"
	s3 := "s3"
	p1 := XXXType(&s1)
	p2 := XXXType(&s2)
	p3 := XXXType(&s3)

	fun := func(fr bool) {
		if fr {
			l.PushFront(p1)
			l.PushFront(p2)
			l.PushFront(p3)
		} else {
			l.PushBack(p3)
			l.PushBack(p2)
			l.PushBack(p1)
		}

		assert.Equal(t, l.Front.Value, p3)
		assert.Equal(t, l.Front.Next.Value, p2)
		assert.Equal(t, l.Front.Next.Next.Value, p1)
		assert.Nil(t, l.Front.Next.Next.Next)

		assert.Equal(t, l.Back.Value, p1)
		assert.Equal(t, l.Back.Prev.Value, p2)
		assert.Equal(t, l.Back.Prev.Prev.Value, p3)
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
