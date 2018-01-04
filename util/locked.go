/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2018 Markus Stenberg
 *
 * Created:       Thu Jan  4 12:21:40 2018 mstenber
 * Last modified: Thu Jan  4 13:05:52 2018 mstenber
 * Edit time:     17 min
 *
 */

package util

import "sync"
import "sync/atomic"

// RMutexLocked is recursive mutex with convenience features
// (just defer x.Locked()()). It is also insanely slow because golang
// does not provide a way of getting current goroutine id.
type RMutexLocked struct {
	// mut is used by non-owners to request access
	mut sync.Mutex

	// ownerMut is used by owner to play with owner/timesOwned.
	ownerMut sync.Mutex

	owner      uint64
	timesOwned int64
}

func (self *RMutexLocked) Lock() {
	gid := GetGoroutineID()
	owning_gid := atomic.LoadUint64(&self.owner)
	if gid == owning_gid {
		self.ownerMut.Lock()
		owning_gid = self.owner
		if gid == owning_gid {
			self.timesOwned++
			self.ownerMut.Unlock()
			return
		}
		self.ownerMut.Unlock()
	}
	self.mut.Lock()
	atomic.StoreUint64(&self.owner, gid)
	self.ownerMut.Lock()
	self.timesOwned = 1
	self.ownerMut.Unlock()
}

func (self *RMutexLocked) Unlock() {
	self.ownerMut.Lock()
	self.timesOwned--
	if self.timesOwned == 0 {
		atomic.StoreUint64(&self.owner, 0)
		self.mut.Unlock()
	}
	self.ownerMut.Unlock()
}

func (self *RMutexLocked) Locked() (unlock func()) {
	self.Lock()
	return func() {
		self.Unlock()
	}
}

type MutexLocked sync.Mutex

func (self *MutexLocked) Locked() (unlock func()) {
	mut := (*sync.Mutex)(self)
	mut.Lock()
	return func() {
		mut.Unlock()
	}
}
