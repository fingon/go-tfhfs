/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2018 Markus Stenberg
 *
 * Created:       Thu Jan 11 07:40:22 2018 mstenber
 * Last modified: Thu Jan 25 13:34:46 2018 mstenber
 * Edit time:     17 min
 *
 */

package util

import (
	"runtime"
	"sync"
)

const DefaultPerCPU = 1

// ParallelLimiter provides a way of ensuring that at most N
// particular things occur at same time. It is essentially semaphore
// with trivial API. (either defer Limited()(), or Go(func))
type ParallelLimiter struct {
	// How many things are allowed per CPU (defaults to DefaultPerCPU)
	LimitPerCPU int

	// How many things are allowed by total (by default using
	// LimitPerCPU to calculate this)
	LimitTotal int

	lock        MutexLocked
	cond        sync.Cond
	running     int
	initialized bool
}

func (self *ParallelLimiter) init() {
	if self.LimitTotal == 0 {
		// initialize
		if self.LimitPerCPU == 0 {
			self.LimitPerCPU = DefaultPerCPU
		}
		self.LimitTotal = runtime.NumCPU() * self.LimitPerCPU
	}
	self.cond.L = &self.lock
	self.initialized = true
}

// Limited2 reserves 'count' execution slots.
func (self *ParallelLimiter) Limited2(count int) func() {
	defer self.lock.Locked()()

	if !self.initialized {
		self.init()
	}

	for (self.running + count) > self.LimitTotal {
		self.cond.Wait()
	}
	self.running += count
	return func() {
		defer self.lock.Locked()()
		self.running -= count
		self.cond.Signal()
	}
}

func (self *ParallelLimiter) Limited() func() {
	return self.Limited2(1)
}

func (self *ParallelLimiter) Go(cb func()) {
	unlock := self.Limited()
	go func() { // ok, limited by #LimitTotal
		defer unlock()
		cb()
	}()
}

func (self *ParallelLimiter) Exclusive(cb func()) {
	unlock := self.Limited2(self.LimitTotal)
	defer unlock()
	cb()
}
