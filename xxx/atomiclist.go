/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2018 Markus Stenberg
 *
 * Created:       Thu Jan 25 14:11:32 2018 mstenber
 * Last modified: Thu Jan 25 14:20:23 2018 mstenber
 * Edit time:     7 min
 *
 */

package xxx

import (
	"sync/atomic"
	"unsafe"
)

type XXXAtomicList struct {
	// I wish this could reuse pointer.go but it would get bit too
	// meta even for me.
	New func() XXXType

	// Start of freelist
	pointer unsafe.Pointer
}

func (self *XXXAtomicList) Get() XXXType {
	for {
		v := atomic.LoadPointer(&self.pointer)
		if v == nil {
			return self.New()
		}
		e := (*xxxAtomicListElement)(v)
		if atomic.CompareAndSwapPointer(&self.pointer,
			v,
			e.next) {
			return e.value
		}
	}
}

func (self *XXXAtomicList) Put(value XXXType) {
	for {
		v := atomic.LoadPointer(&self.pointer)
		e := &xxxAtomicListElement{value, v}
		if atomic.CompareAndSwapPointer(&self.pointer,
			v,
			unsafe.Pointer(e)) {
			return
		}
	}
}

type xxxAtomicListElement struct {
	value XXXType
	next  unsafe.Pointer
}
