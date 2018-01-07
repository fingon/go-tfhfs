/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2018 Markus Stenberg
 *
 * Created:       Thu Jan  4 11:44:09 2018 mstenber
 * Last modified: Sun Jan  7 16:52:20 2018 mstenber
 * Edit time:     11 min
 *
 */

package xxx

import (
	"sync/atomic"
	"unsafe"
)

// XXXAtomicPointer provides typesafe access to type
type XXXAtomicPointer struct {
	pointer unsafe.Pointer
}

func (self *XXXAtomicPointer) Get() XXXType {
	v := atomic.LoadPointer(&self.pointer)
	return XXXType(v)
}

func (self *XXXAtomicPointer) Set(value XXXType) {
	atomic.StorePointer(&self.pointer, unsafe.Pointer(value))
}

func (self *XXXAtomicPointer) SetIfEqualTo(newAtomicPointer, oldAtomicPointer XXXType) bool {
	return atomic.CompareAndSwapPointer(&self.pointer,
		unsafe.Pointer(oldAtomicPointer),
		unsafe.Pointer(newAtomicPointer))
}
