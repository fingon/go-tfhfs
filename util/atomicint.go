/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2018 Markus Stenberg
 *
 * Created:       Wed Mar 21 11:19:49 2018 mstenber
 * Last modified: Wed Mar 21 11:24:53 2018 mstenber
 * Edit time:     5 min
 *
 */

package util

import "sync/atomic"

type AtomicInt int64

func (self *AtomicInt) Get() int64 {
	i := (*int64)(self)
	return atomic.LoadInt64(i)
}

func (self *AtomicInt) GetInt() int {
	return int(self.Get())
}

func (self *AtomicInt) Add(value int64) {
	i := (*int64)(self)
	atomic.AddInt64(i, value)
}

func (self *AtomicInt) AddInt(value int) {
	self.Add(int64(value))
}

func (self *AtomicInt) Set(value int64) {
	i := (*int64)(self)
	atomic.StoreInt64(i, value)
}

func (self *AtomicInt) SetInt(value int64) {
	self.Set(value)
}
