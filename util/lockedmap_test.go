/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2018 Markus Stenberg
 *
 * Created:       Fri Jan  5 02:26:21 2018 mstenber
 * Last modified: Mon Jan  8 11:49:47 2018 mstenber
 * Edit time:     6 min
 *
 */

package util

import (
	"sync"
	"testing"

	"github.com/fingon/go-tfhfs/mlog"
)

func TestLockedMap(t *testing.T) {
	t.Parallel()
	l := &MutexLockedMap{}
	var mut, mut2, mut3 sync.Mutex
	// Rather complex lock sequence to ensure that lock reuse also works
	// (Proof is in the MLOG)
	mut.Lock()
	mut2.Lock()
	mut3.Lock()
	defer mut.Lock()
	defer l.Locked("foo")()
	go func() {
		mut2.Lock()
		mut3.Unlock()
		mlog.Printf2("util/lockedmap_test", "goroutine")
		defer mut.Unlock()
		defer l.Locked("foo")()
	}()
	defer l.Locked("bar")()
	mut2.Unlock()
	mut3.Lock()
	mlog.Printf2("util/lockedmap_test", "exiting")
}
