package fs
import "fmt"

// deleteNotifyList provides doubly linked list which does not have inefficient
// operations, is typesafe, and does minimum amount of extra
// allocations needed. This is accomplished by sticking the freed
// items to a freelist instead of freeing them directly. The list is
// obviously not threadsafe.
type deleteNotifyList struct {
	Back, Front *deleteNotifyListElement
	freeList    *deleteNotifyListElement
	Length      int
}

type deleteNotifyListElement struct {
	Prev, Next *deleteNotifyListElement
	Value      deleteNotify
}

func (self *deleteNotifyList) getElement(v deleteNotify) (e *deleteNotifyListElement) {
	if self.freeList == nil {
		return &deleteNotifyListElement{Value: v}
	}
	e = self.freeList
	self.freeList = e.Next
	e.Value = v
	return e
}

func (self *deleteNotifyList) Iterate(cb func(v deleteNotify)) {
	for e := self.Front; e != nil; e = e.Next {
		cb(e.Value)
	}
}

func (self *deleteNotifyList) PushBackElement(e *deleteNotifyListElement) {
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

func (self *deleteNotifyList) PushBack(v deleteNotify) *deleteNotifyListElement {
	e := self.getElement(v)
	self.PushBackElement(e)
	return e
}

func (self *deleteNotifyList) PushFrontElement(e *deleteNotifyListElement) {
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
func (self *deleteNotifyList) PushFront(v deleteNotify) *deleteNotifyListElement {
	e := self.getElement(v)
	self.PushFrontElement(e)
	return e
}

func (self *deleteNotifyList) RemoveElement(e *deleteNotifyListElement) {
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

func (self *deleteNotifyList) Remove(e *deleteNotifyListElement) {
	self.RemoveElement(e)
	e.Prev = nil
	e.Next = self.freeList
	self.freeList = e
}

func (self *deleteNotifyList) String() string {
	llen := func(l *deleteNotifyListElement) int {
		len := 0
		for ; l != nil; l = l.Next {
			len++
		}
		return len
	}

	return fmt.Sprintf("deleteNotifyList<%d entries/%d free>", llen(self.Front), llen(self.freeList))

}
