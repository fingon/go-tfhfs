package hugger
import (
	"sync/atomic"
	"unsafe"
)

// treeRootAtomicPointer provides typesafe access to type
type treeRootAtomicPointer struct {
	pointer unsafe.Pointer
}

func (self *treeRootAtomicPointer) Get() (*treeRoot) {
	v := atomic.LoadPointer(&self.pointer)
	return (*treeRoot)(v)
}

func (self *treeRootAtomicPointer) Set(value (*treeRoot)) {
	atomic.StorePointer(&self.pointer, unsafe.Pointer(value))
}

func (self *treeRootAtomicPointer) SetIfEqualTo(newAtomicPointer, oldAtomicPointer (*treeRoot)) bool {
	return atomic.CompareAndSwapPointer(&self.pointer,
		unsafe.Pointer(oldAtomicPointer),
		unsafe.Pointer(newAtomicPointer))
}
