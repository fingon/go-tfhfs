/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2018 Markus Stenberg
 *
 * Created:       Fri Mar 16 12:24:58 2018 mstenber
 * Last modified: Fri Mar 16 12:54:12 2018 mstenber
 * Edit time:     9 min
 *
 */

package xxx

import (
	"fmt"
	"testing"

	"github.com/stvp/assert"
)

func TestCart(t *testing.T) {
	t.Parallel()
	size := 3
	n := 10
	c := YYYCart{}.Init(size)
	for i := 0; i < n; i++ {
		k := fmt.Sprintf("%d", i)
		c.Set(k, YYYType(i))
	}
	for i := 0; i < n; i++ {
		k := fmt.Sprintf("%d", i)
		v, ok := c.Get(k)
		assert.True(t, ok == (i >= n-size), "broken index ", i)
		if ok {
			assert.Equal(t, v, YYYType(i))
		}
	}
	// now 7-9 in cache
	c.Set("4", YYYType(4))
	c.Set("9", YYYType(9))
	c.Set("8", YYYType(8))
	c.Set("5", YYYType(5))
	c.Set("6", YYYType(6))

}
