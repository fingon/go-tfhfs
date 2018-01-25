/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2018 Markus Stenberg
 *
 * Created:       Thu Jan  4 12:21:40 2018 mstenber
 * Last modified: Thu Jan 25 13:36:06 2018 mstenber
 * Edit time:     30 min
 *
 */

package util

import (
	"log"
	"sync"
	"sync/atomic"

	"github.com/fingon/go-tfhfs/mlog"
	"github.com/fingon/go-tfhfs/util/gid"
)

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
	gid := gid.GetGoroutineID()
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

type MutexLocked struct {
	mu sync.Mutex

	// just when debugging is enabled, ensure we actually own the
	// mutex as well
	owner uint64
}

func (self *MutexLocked) AssertLocked() {
	if mlog.IsEnabled() {
		gid := gid.GetGoroutineID()
		ogid := atomic.LoadUint64(&self.owner)
		if ogid != gid {
			log.Panicf("Not locked by us - %v != our %v", ogid, gid)
		}
	}
}

func (self *MutexLocked) Lock() {
	debugging := mlog.IsEnabled()
	if debugging {
		gid := gid.GetGoroutineID()
		if atomic.LoadUint64(&self.owner) == gid {
			log.Panic("Double lock by us")
		}
	}
	self.mu.Lock()
	if debugging {
		atomic.StoreUint64(&self.owner, gid.GetGoroutineID())
	}
}

func (self *MutexLocked) Unlock() {
	if mlog.IsEnabled() {
		atomic.StoreUint64(&self.owner, 0)
	}
	self.mu.Unlock()
}

func (self *MutexLocked) Locked() (unlock func()) {
	self.Lock()
	return func() {
		self.Unlock()
	}
}

func (self *MutexLocked) ClearOwner() {
	if mlog.IsEnabled() {
		atomic.StoreUint64(&self.owner, 0)
	}
}

func (self *MutexLocked) UpdateOwner() {
	if mlog.IsEnabled() {
		atomic.StoreUint64(&self.owner, gid.GetGoroutineID())
	}
}

func (self *MutexLocked) Do(cb func()) {
	self.Lock()
	cb()
	self.Unlock()
}
