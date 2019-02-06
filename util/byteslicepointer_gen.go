package util
import (
	"sync/atomic"
	"unsafe"
)

// ByteSliceAtomicPointer provides typesafe access to type
type ByteSliceAtomicPointer struct {
	pointer unsafe.Pointer
}

func (self *ByteSliceAtomicPointer) Get() (*[]byte) {
	v := atomic.LoadPointer(&self.pointer)
	return (*[]byte)(v)
}

func (self *ByteSliceAtomicPointer) Set(value (*[]byte)) {
	atomic.StorePointer(&self.pointer, unsafe.Pointer(value))
}

func (self *ByteSliceAtomicPointer) SetIfEqualTo(newAtomicPointer, oldAtomicPointer (*[]byte)) bool {
	return atomic.CompareAndSwapPointer(&self.pointer,
		unsafe.Pointer(oldAtomicPointer),
		unsafe.Pointer(newAtomicPointer))
}
