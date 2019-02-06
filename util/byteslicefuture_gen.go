package util
import (
	"sync"

)

type ByteSliceFutureValueCallback func() []byte

// ByteSliceFuture is a value-based Future object. Threadsafe, and multiple
// Gets do work as expected unlike some other implementations. There
// are two ways to set this up: provide SetCallback, or call Set by
// hand at some point.
type ByteSliceFuture struct {
	ValueCallback ByteSliceFutureValueCallback
	lock          MutexLocked
	cond          sync.Cond
	v             []byte
	called, vset  bool
}

func (self *ByteSliceFuture) Get() []byte {
	defer self.lock.Locked()()
	if !self.vset && self.ValueCallback != nil && !self.called {
		self.called = true
		value := self.ValueCallback()
		self.set(value)
	}
	for !self.vset {
		if self.cond.L == nil {
			self.cond.L = &self.lock
		}
		self.cond.Wait()
	}
	return self.v
}

func (self *ByteSliceFuture) Set(v []byte) {
	defer self.lock.Locked()()
	self.set(v)
}

func (self *ByteSliceFuture) set(v []byte) {
	self.v = v
	self.vset = true
	if self.cond.L != nil {
		self.cond.Broadcast()
	}

}

func (self *ByteSliceFuture) SetCallback(cb ByteSliceFutureValueCallback) *ByteSliceFuture {
	self.ValueCallback = cb
	return self
}
