/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2018 Markus Stenberg
 *
 * Created:       Thu Jan  4 12:24:51 2018 mstenber
 * Last modified: Thu Jan  4 12:26:48 2018 mstenber
 * Edit time:     2 min
 *
 */

package util

import (
	"sync"
	"testing"

	"github.com/stvp/assert"
)

func Test(t *testing.T) {
	t.Parallel()
	var l MutexLocked

	var wg sync.WaitGroup
	wg.Add(10)
	j := 0
	for i := 0; i < 10; i++ {
		go func() {
			defer wg.Done()
			defer l.Locked()()
			j++
		}()
	}
	wg.Wait()
	assert.Equal(t, j, 10)
}
