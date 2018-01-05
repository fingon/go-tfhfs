/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2018 Markus Stenberg
 *
 * Created:       Fri Jan  5 01:52:26 2018 mstenber
 * Last modified: Fri Jan  5 02:56:57 2018 mstenber
 * Edit time:     14 min
 *
 */

package util

import "github.com/fingon/go-tfhfs/mlog"

type NamedMutexLockedMap struct {
	l MutexLocked
	m map[string]*MutexLocked
	q map[string]int
}

func (self *NamedMutexLockedMap) GetLockedByName(name string) *MutexLocked {
	defer self.l.Locked()()
	return self.m[name]
}

func (self *NamedMutexLockedMap) Locked(name string) func() {
	self.l.Lock()
	if self.m == nil {
		self.m = make(map[string]*MutexLocked)
		self.q = make(map[string]int)
	}
	ll := self.m[name]
	if ll == nil {
		mlog.Printf2("util/lockedmap", "Locked created lock %s", name)
		ll = &MutexLocked{}
		self.m[name] = ll
	}
	self.q[name]++
	self.l.Unlock()
	ul := ll.Locked()
	mlog.Printf2("util/lockedmap", "Locked %s", name)
	return func() {
		defer self.l.Locked()()
		mlog.Printf2("util/lockedmap", "Releasing %s", name)
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
