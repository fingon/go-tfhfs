package fs
import (
	"sync/atomic"
	"unsafe"
)

// InodeMetaAtomicPointer provides typesafe access to type
type InodeMetaAtomicPointer struct {
	pointer unsafe.Pointer
}

func (self *InodeMetaAtomicPointer) Get() (*InodeMeta) {
	v := atomic.LoadPointer(&self.pointer)
	return (*InodeMeta)(v)
}

func (self *InodeMetaAtomicPointer) Set(value (*InodeMeta)) {
	atomic.StorePointer(&self.pointer, unsafe.Pointer(value))
}

func (self *InodeMetaAtomicPointer) SetIfEqualTo(newAtomicPointer, oldAtomicPointer (*InodeMeta)) bool {
	return atomic.CompareAndSwapPointer(&self.pointer,
		unsafe.Pointer(oldAtomicPointer),
		unsafe.Pointer(newAtomicPointer))
}
