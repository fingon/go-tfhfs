/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2019 Markus Stenberg
 *
 * Created:       Thu Feb  7 10:48:54 2019 mstenber
 * Last modified: Thu Feb  7 11:12:11 2019 mstenber
 * Edit time:     3 min
 *
 */

package xxx

import (
	"testing"

	"github.com/stvp/assert"
)

func TestIChannel(t *testing.T) {
	t.Parallel()
	c := YYYIChannel{}
	v1 := YYYType(42)
	v2 := YYYType(43)
	c.Send(v1)
	c.Send(v2)
	assert.Equal(t, c.Receive(), v1)
	assert.Equal(t, c.Receive(), v2)

	c = YYYIChannel{}
	ch := c.Channel()
	c.Send(v2)
	c.Send(v1)
	assert.Equal(t, <-ch, v2)
	assert.Equal(t, <-ch, v1)
	c.Send(v1)
	c.Send(v2)
	assert.Equal(t, <-ch, v1)
	assert.Equal(t, <-ch, v2)
}
