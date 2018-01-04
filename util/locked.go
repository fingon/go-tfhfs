/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2018 Markus Stenberg
 *
 * Created:       Thu Jan  4 12:21:40 2018 mstenber
 * Last modified: Thu Jan  4 12:24:34 2018 mstenber
 * Edit time:     3 min
 *
 */

package util

import "sync"

type MutexLocked sync.Mutex

func (self *MutexLocked) Locked() (unlock func()) {
	mut := (*sync.Mutex)(self)
	mut.Lock()
	return func() {
		mut.Unlock()
	}
}
