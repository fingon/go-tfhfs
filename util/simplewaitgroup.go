/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2018 Markus Stenberg
 *
 * Created:       Mon Jan  8 10:19:05 2018 mstenber
 * Last modified: Mon Jan  8 10:28:22 2018 mstenber
 * Edit time:     2 min
 *
 */

package util

import "sync"

type SimpleWaitGroup struct {
	sync.WaitGroup
}

func (self *SimpleWaitGroup) Go(cb MapRunnerCallback) {
	self.Add(1)
	go func() {
		defer self.Done()
		cb()
	}()
}
