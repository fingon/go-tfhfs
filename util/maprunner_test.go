/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2018 Markus Stenberg
 *
 * Created:       Sun Jan  7 17:46:09 2018 mstenber
 * Last modified: Thu Jan 11 08:15:49 2018 mstenber
 * Edit time:     13 min
 *
 */

package util

import (
	"sync"
	"testing"
	"time"

	"github.com/stvp/assert"
)

func TestMapRunner(t *testing.T) {
	t.Parallel()

	mr := MapRunner{}
	l1 := sync.Mutex{}
	l1.Lock()
	started1 := 0
	started2 := 0
	started3 := 0
	done := false
	mr.Go(1, func() {
		started1++
		l1.Lock() // #1
	})
	mr.Go(2, func() {
		started2++
	})
	mr.Go(3, func() {
		started3++
	})
	mr.Go(1, func() {
		started1 += 2
		l1.Lock() // #2

	})
	mr.Go(1, func() {
		started1 += 4
		l1.Lock() // #3
		time.Sleep(time.Millisecond)
		done = true
	})
	time.Sleep(time.Millisecond)
	assert.Equal(t, started1, 1)
	assert.Equal(t, started2, 1)
	assert.Equal(t, started3, 1)
	l1.Unlock() // #1 - let 1.2 start
	time.Sleep(time.Millisecond)
	assert.Equal(t, started1, 3)
	l1.Unlock() // #2 - let 1.3 start
	time.Sleep(time.Millisecond)
	assert.Equal(t, started1, 7)
	l1.Unlock() // #3 - let 1.3 finish (with delay)
	mr.Close()
	assert.True(t, done)
	assert.Equal(t, mr.Queued, 2) // 1.2, 1.3
	assert.Equal(t, mr.Ran, 3)    // 1.1, 2, 3
}
