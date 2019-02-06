package storage
import (
	"sync"

	"github.com/fingon/go-tfhfs/util"
)

type BlockPointerFutureValueCallback func() *Block

// BlockPointerFuture is a value-based Future object. Threadsafe, and multiple
// Gets do work as expected unlike some other implementations. There
// are two ways to set this up: provide SetCallback, or call Set by
// hand at some point.
type BlockPointerFuture struct {
	ValueCallback BlockPointerFutureValueCallback
	lock          util.MutexLocked
	cond          sync.Cond
	v             *Block
	called, vset  bool
}

func (self *BlockPointerFuture) Get() *Block {
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

func (self *BlockPointerFuture) Set(v *Block) {
	defer self.lock.Locked()()
	self.set(v)
}

func (self *BlockPointerFuture) set(v *Block) {
	self.v = v
	self.vset = true
	if self.cond.L != nil {
		self.cond.Broadcast()
	}

}

func (self *BlockPointerFuture) SetCallback(cb BlockPointerFutureValueCallback) *BlockPointerFuture {
	self.ValueCallback = cb
	return self
}
