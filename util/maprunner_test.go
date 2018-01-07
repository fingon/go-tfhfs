/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2018 Markus Stenberg
 *
 * Created:       Sun Jan  7 17:46:09 2018 mstenber
 * Last modified: Sun Jan  7 17:54:27 2018 mstenber
 * Edit time:     4 min
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
	mr.Run(1, func() {
		started1++
		l1.Lock()
	})
	mr.Run(2, func() {
		started2++
	})
	mr.Run(1, func() {
		started1++
		l1.Lock()

	})
	time.Sleep(time.Millisecond)
	assert.Equal(t, started1, 1)
	assert.Equal(t, started2, 1)
	l1.Unlock()
	time.Sleep(time.Millisecond)
	assert.Equal(t, started1, 2)
	l1.Unlock()
}
