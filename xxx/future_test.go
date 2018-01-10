/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2018 Markus Stenberg
 *
 * Created:       Wed Jan 10 09:28:59 2018 mstenber
 * Last modified: Wed Jan 10 09:47:18 2018 mstenber
 * Edit time:     3 min
 *
 */

package xxx

import (
	"testing"

	"github.com/stvp/assert"
)

func TestFutureSet(t *testing.T) {
	t.Parallel()

	v := YYYType(7)
	v2 := YYYType(3)

	var f YYYFuture

	go func() {
		f.Set(v)
	}()
	v2 = f.Get()
	assert.Equal(t, v, v2)
}

func TestFutureSetCallback(t *testing.T) {
	t.Parallel()

	v := YYYType(7)
	v2 := YYYType(3)

	var f YYYFuture
	f.SetCallback(func() YYYType {
		return v
	})

	v2 = f.Get()
	assert.Equal(t, v, v2)
}
