package util
import (
	"sync/atomic"
	"unsafe"
)

type ByteSliceAtomicList struct {
	// I wish this could reuse pointer.go but it would get bit too
	// meta even for me.
	New func() []byte

	// Start of freelist
	pointer unsafe.Pointer
}

func (self *ByteSliceAtomicList) Get() []byte {
	for {
		v := atomic.LoadPointer(&self.pointer)
		if v == nil {
			return self.New()
		}
		e := (*byteSliceAtomicListElement)(v)
		if atomic.CompareAndSwapPointer(&self.pointer,
			v,
			e.next) {
			return e.value
		}
	}
}

func (self *ByteSliceAtomicList) Put(value []byte) {
	for {
		v := atomic.LoadPointer(&self.pointer)
		e := &byteSliceAtomicListElement{value, v}
		if atomic.CompareAndSwapPointer(&self.pointer,
			v,
			unsafe.Pointer(e)) {
			return
		}
	}
}

type byteSliceAtomicListElement struct {
	value []byte
	next  unsafe.Pointer
}
