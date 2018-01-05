/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2018 Markus Stenberg
 *
 * Created:       Fri Jan  5 01:52:26 2018 mstenber
 * Last modified: Fri Jan  5 02:03:26 2018 mstenber
 * Edit time:     8 min
 *
 */

package util

type NamedMutexLockedMap struct {
	l MutexLocked
	m map[string]*MutexLocked
	q map[string]int
}

func (self *NamedMutexLockedMap) Locked(name string) func() {
	self.l.Lock()
	if self.m == nil {
		self.m = make(map[string]*MutexLocked)
		self.q = make(map[string]int)
	}
	ll := self.m[name]
	if ll == nil {
		ll = &MutexLocked{}
		self.m[name] = ll
	}
	self.q[name]++
	self.l.Unlock()
	ul := ll.Locked()
	return func() {
		defer self.l.Locked()()
		self.q[name]--
		if self.q[name] == 0 {
			delete(self.m, name)
			delete(self.q, name)
			return
		}
		// normally unlock only mutex that is not outright deleted
		ul()
	}
}
