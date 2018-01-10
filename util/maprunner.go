/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2018 Markus Stenberg
 *
 * Created:       Sun Jan  7 16:45:31 2018 mstenber
 * Last modified: Wed Jan 10 10:54:22 2018 mstenber
 * Edit time:     37 min
 *
 */

package util

import (
	"log"
	"sync"

	"github.com/fingon/go-tfhfs/mlog"
)

type MapRunnerCallback func()

// MapRunner provides facility of running arbitrary set of goroutines
// that do not conflict with each other. The confliction is defined by
// the key provided alongside the callback. Conflicting callbacks are
// serialized.
type MapRunner struct {
	busy             map[interface{}]bool
	blockedPerTarget map[interface{}]*MapRunnerCallbackList
	lock             MutexLocked
	closing, closed  bool
	died             sync.Cond
	Ran, Queued      int
}

// Close runs to completion current (and subsequently queued items)
// until we can close cleanly.
func (self *MapRunner) Close() {
	defer self.lock.Locked()()
	if self.busy == nil {
		return
	}
	check := func() bool {
		self.closing = true
		if len(self.busy) == 0 {
			return true
		}
		self.closed = true
		return false
	}
	for !check() {
		self.died.Wait()
	}
}

func (self *MapRunner) Run(key interface{}, cb MapRunnerCallback) {
	defer self.lock.Locked()()
	if self.busy == nil {
		self.died.L = &self.lock
		self.busy = make(map[interface{}]bool)
		self.blockedPerTarget = make(map[interface{}]*MapRunnerCallbackList)
	}
	if self.busy[key] {
		mlog.Printf("mr.Run queued")
		self.Queued++
		l := self.blockedPerTarget[key]
		if l == nil {
			l = &MapRunnerCallbackList{}
			self.blockedPerTarget[key] = l
		}
		l.PushBack(cb)
		return
	}
	self.Ran++
	if self.closed {
		log.Panicf("Attempt to .Run() on closed MapRunner")
	}
	// It's not busy, we can just start goroutine and mark it busy
	mlog.Printf("mr.Run immediate")
	self.busy[key] = true
	go func() {
		self.run(key, cb)
	}()
}

func (self *MapRunner) run(key interface{}, cb MapRunnerCallback) {
	for cb != nil {
		cb()
		cb = self.checkMore(key)
	}
}

func (self *MapRunner) checkMore(key interface{}) MapRunnerCallback {
	defer self.lock.Locked()()
	l, ok := self.blockedPerTarget[key]
	if !ok || l.Front == nil {
		mlog.Printf("checkMore: nothing in queue")
		if self.closing {
			self.died.Signal()
		}
		delete(self.busy, key)
		delete(self.blockedPerTarget, key)
		return nil
	}
	mlog.Printf("checkMore: popped from queue")
	cb := l.Front.Value
	l.Remove(l.Front)
	return cb
}
