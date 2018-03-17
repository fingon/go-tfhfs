/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2018 Markus Stenberg
 *
 * Created:       Fri Mar 16 12:24:58 2018 mstenber
 * Last modified: Sat Mar 17 11:29:05 2018 mstenber
 * Edit time:     43 min
 *
 */

package xxx

import (
	"fmt"
	"testing"

	"github.com/fingon/go-tfhfs/mlog"
	"github.com/fingon/go-tfhfs/util"
	"github.com/stvp/assert"
)

func xxx(s string) XXXType {
	return XXXType(&s)
}

func TestCart(t *testing.T) {
	t.Parallel()
	size := 3
	n := 10
	c := XXXCart{}
	c.Init(size)
	for i := 0; i < n; i++ {
		s := fmt.Sprintf("%d", i)
		k := ZZZType(s)
		c.Set(k, xxx(s))
	}
	for i := 0; i < n; i++ {
		s := fmt.Sprintf("%d", i)
		k := ZZZType(s)
		v, ok := c.Get(k)
		assert.True(t, ok == (i >= n-size), "broken index ", i)
		if ok {
			assert.Equal(t, string(*v), s)
		}
	}
	// now 7-9 in cache
	c.Set("4", xxx("4"))
	c.Set("9", xxx("9"))
	c.Set("8", xxx("8"))
	c.Set("5", xxx("5"))
	c.Set("6", xxx("6"))
	v, created := c.GetOrCreate("10", func(key ZZZType) XXXType {
		return xxx("10")
	})
	assert.True(t, created)
	assert.Equal(t, string(*v), "10")
	v, created = c.GetOrCreate("10", func(key ZZZType) XXXType {
		return xxx("10")
	})
	assert.True(t, !created)
	assert.Equal(t, string(*v), "10")

	c.Set("10", xxx("11"))
	v, found := c.Get("10")
	assert.True(t, found)
	assert.Equal(t, string(*v), "11")
}

func sanityCheckCart(t *testing.T, cart XXXCart) {
	var cns, cnl, ct1, ct2, cb1, cb2 int
	for _, v := range cart.cache {
		if v.value != nil {
			if v.filterlong {
				cnl++
			} else {
				cns++
			}
			if v.frequentbit {
				ct2++
			} else {
				ct1++
			}
		} else {
			if v.frequentbit {
				cb2++
			} else {
				cb1++
			}
		}
	}
	assert.Equal(t, cart.ns, cns)
	assert.Equal(t, cart.nl, cnl)
	assert.Equal(t, cart.t1.Length, ct1)
	assert.Equal(t, cart.t2.Length, ct2)
	assert.True(t, ct1+ct2 <= cart.c)
	assert.Equal(t, cart.b1.Length, cb1)
	assert.Equal(t, cart.b2.Length, cb2)

	assert.True(t, cart.p >= 0)
	assert.True(t, cart.q >= 0)

	seen := make(map[ZZZType]bool)

	checkList := func(expvalue, expfreq bool, list XXXCartEntryList) {
		llen := 0
		list.Iterate(func(e *XXXCartEntry) {
			seen[e.key] = true
			assert.Equal(t, cart.cache[e.key], e)
			assert.Equal(t, expvalue, e.value != nil)
			assert.Equal(t, expfreq, e.frequentbit)
			llen++
		})
		assert.Equal(t, llen, list.Length)
	}
	checkList(true, false, cart.t1)
	checkList(true, true, cart.t2)
	checkList(false, false, cart.b1)
	checkList(false, true, cart.b2)
	assert.Equal(t, len(seen), len(cart.cache))
}

func TestCartTorture(t *testing.T) {
	t.Parallel()

	c := XXXCart{}
	size := 123
	c.Init(size)
	assert.Equal(t, c.c, size)
	rng := util.GetSeededRng()
	var hits, misses int

	for i := 0; i < size*100; i++ {
		var v int
		if rng.Int()%100 < 30 {
			// non-random
			v = (rng.Int() % size) * (rng.Int() % size)
		} else {
			v = rng.Int() % (size * size)
		}
		s := fmt.Sprintf("%d", v)
		k := ZZZType(s)
		value, ok := c.Get(k)
		if ok {
			assert.Equal(t, string(*value), s)
			hits++
		} else {
			if rng.Int()%100 < 20 {
				c.Set(k, nil)
				_, ok := c.Get(k)
				assert.False(t, ok)
			} else {
				c.Set(k, xxx(s))
			}
			misses++
		}
		if i%100 == 0 {
			sanityCheckCart(t, c)
		}
	}
	assert.True(t, misses > 0)
	assert.True(t, hits > 0)
	mlog.Printf2("xxx/cart_test", "Torture had %d hits and %d misses", hits, misses)
}
