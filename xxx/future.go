/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2018 Markus Stenberg
 *
 * Created:       Wed Jan 10 09:25:44 2018 mstenber
 * Last modified: Wed Jan 10 09:49:04 2018 mstenber
 * Edit time:     17 min
 *
 */

package xxx

import (
	"sync"

	"github.com/fingon/go-tfhfs/util"
)

type YYYFutureValueCallback func() YYYType

// YYYFuture is a value-based Future object. Threadsafe, and multiple
// Gets do work as expected unlike some other implementations. There
// are two ways to set this up: provide SetCallback, or call Set by
// hand at some point.
type YYYFuture struct {
	ValueCallback YYYFutureValueCallback
	lock          util.MutexLocked
	cond          sync.Cond
	v             YYYType
	called, vset  bool
}

func (self *YYYFuture) Get() YYYType {
	defer self.lock.Locked()()
	if !self.vset && self.ValueCallback != nil && !self.called {
		self.called = true
		value := self.ValueCallback()
		self.set(value)
	}
	for !self.vset {
		if self.cond.L == nil {
			self.cond.L = &self.lock
		}
		self.cond.Wait()
	}
	return self.v
}

func (self *YYYFuture) Set(v YYYType) {
	defer self.lock.Locked()()
	self.set(v)
}

func (self *YYYFuture) set(v YYYType) {
	self.v = v
	self.vset = true
	if self.cond.L != nil {
		self.cond.Broadcast()
	}

}

func (self *YYYFuture) SetCallback(cb YYYFutureValueCallback) *YYYFuture {
	self.ValueCallback = cb
	return self
}
