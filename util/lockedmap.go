/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2018 Markus Stenberg
 *
 * Created:       Fri Jan  5 01:52:26 2018 mstenber
 * Last modified: Mon Jan  8 11:35:22 2018 mstenber
 * Edit time:     15 min
 *
 */

package util

import "github.com/fingon/go-tfhfs/mlog"

type MutexLockedMap struct {
	l MutexLocked
	m map[interface{}]*MutexLocked
	q map[interface{}]int
}

func (self *MutexLockedMap) GetLockedByName(name interface{}) *MutexLocked {
	defer self.l.Locked()()
	return self.m[name]
}

func (self *MutexLockedMap) Locked(name interface{}) func() {
	self.l.Lock()
	if self.m == nil {
		self.m = make(map[interface{}]*MutexLocked)
		self.q = make(map[interface{}]int)
	}
	ll := self.m[name]
	if ll == nil {
		mlog.Printf2("util/lockedmap", "Locked created lock %v", name)
		ll = &MutexLocked{}
		self.m[name] = ll
	}
	self.q[name]++
	self.l.Unlock()
	ul := ll.Locked()
	mlog.Printf2("util/lockedmap", "Locked %v", name)
	return func() {
		defer self.l.Locked()()
		mlog.Printf2("util/lockedmap", "Releasing %v", name)
		self.q[name]--
		if self.q[name] == 0 {
			mlog.Printf2("util/lockedmap", " was last -> gone")
			delete(self.m, name)
			delete(self.q, name)
			return
		}
		mlog.Printf2("util/lockedmap", " plain unlock")
		// normally unlock only mutex that is not outright deleted
		ul()
	}
}
