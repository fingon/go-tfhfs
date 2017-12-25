/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2017 Markus Stenberg
 *
 * Created:       Mon Dec 25 17:07:23 2017 mstenber
 * Last modified: Mon Dec 25 17:19:10 2017 mstenber
 * Edit time:     5 min
 *
 */

package ibtree

import (
	"fmt"
	"testing"

	"github.com/stvp/assert"
)

func TestIBTree(t *testing.T) {
	r := IBTree{}.Init(nil).NewRoot()
	v := r.Get(IBKey("foo"))
	assert.Nil(t, v)
	for i := 0; i < 100; i++ {
		fmt.Printf("Creating #%d\n", i)
		r = r.Set(IBKey(fmt.Sprintf("%d", i)), "foo")
	}
}
