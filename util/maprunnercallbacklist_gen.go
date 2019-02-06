package util
import "fmt"

// MapRunnerCallbackList provides doubly linked list which does not have inefficient
// operations, is typesafe, and does minimum amount of extra
// allocations needed. This is accomplished by sticking the freed
// items to a freelist instead of freeing them directly. The list is
// obviously not threadsafe.
type MapRunnerCallbackList struct {
	Back, Front *MapRunnerCallbackListElement
	freeList    *MapRunnerCallbackListElement
	Length      int
}

type MapRunnerCallbackListElement struct {
	Prev, Next *MapRunnerCallbackListElement
	Value      MapRunnerCallback
}

func (self *MapRunnerCallbackList) getElement(v MapRunnerCallback) (e *MapRunnerCallbackListElement) {
	if self.freeList == nil {
		return &MapRunnerCallbackListElement{Value: v}
	}
	e = self.freeList
	self.freeList = e.Next
	e.Value = v
	return e
}

func (self *MapRunnerCallbackList) Iterate(cb func(v MapRunnerCallback)) {
	for e := self.Front; e != nil; e = e.Next {
		cb(e.Value)
	}
}

func (self *MapRunnerCallbackList) PushBackElement(e *MapRunnerCallbackListElement) {
	e.Next = nil
	e.Prev = self.Back
	if self.Back != nil {
		self.Back.Next = e
	}
	if self.Front == nil {
		self.Front = e
	}
	self.Back = e
	self.Length++
}

func (self *MapRunnerCallbackList) PushBack(v MapRunnerCallback) *MapRunnerCallbackListElement {
	e := self.getElement(v)
	self.PushBackElement(e)
	return e
}

func (self *MapRunnerCallbackList) PushFrontElement(e *MapRunnerCallbackListElement) {
	e.Prev = nil
	e.Next = self.Front
	if self.Front != nil {
		self.Front.Prev = e
	}
	if self.Back == nil {
		self.Back = e
	}
	self.Front = e
	self.Length++
}
func (self *MapRunnerCallbackList) PushFront(v MapRunnerCallback) *MapRunnerCallbackListElement {
	e := self.getElement(v)
	self.PushFrontElement(e)
	return e
}

func (self *MapRunnerCallbackList) RemoveElement(e *MapRunnerCallbackListElement) {
	if e.Prev != nil {
		e.Prev.Next = e.Next
	} else {
		self.Front = e.Next
	}
	if e.Next != nil {
		e.Next.Prev = e.Prev
	} else {
		self.Back = e.Prev
	}
	self.Length--
}

func (self *MapRunnerCallbackList) Remove(e *MapRunnerCallbackListElement) {
	self.RemoveElement(e)
	e.Prev = nil
	e.Next = self.freeList
	self.freeList = e
}

func (self *MapRunnerCallbackList) String() string {
	llen := func(l *MapRunnerCallbackListElement) int {
		len := 0
		for ; l != nil; l = l.Next {
			len++
		}
		return len
	}

	return fmt.Sprintf("MapRunnerCallbackList<%d entries/%d free>", llen(self.Front), llen(self.freeList))

}
