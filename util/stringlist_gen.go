package util
import "fmt"

// StringList provides doubly linked list which does not have inefficient
// operations, is typesafe, and does minimum amount of extra
// allocations needed. This is accomplished by sticking the freed
// items to a freelist instead of freeing them directly. The list is
// obviously not threadsafe.
type StringList struct {
	Back, Front *StringListElement
	freeList    *StringListElement
	Length      int
}

type StringListElement struct {
	Prev, Next *StringListElement
	Value      string
}

func (self *StringList) getElement(v string) (e *StringListElement) {
	if self.freeList == nil {
		return &StringListElement{Value: v}
	}
	e = self.freeList
	self.freeList = e.Next
	e.Value = v
	return e
}

func (self *StringList) Iterate(cb func(v string)) {
	for e := self.Front; e != nil; e = e.Next {
		cb(e.Value)
	}
}

func (self *StringList) PushBackElement(e *StringListElement) {
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

func (self *StringList) PushBack(v string) *StringListElement {
	e := self.getElement(v)
	self.PushBackElement(e)
	return e
}

func (self *StringList) PushFrontElement(e *StringListElement) {
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
func (self *StringList) PushFront(v string) *StringListElement {
	e := self.getElement(v)
	self.PushFrontElement(e)
	return e
}

func (self *StringList) RemoveElement(e *StringListElement) {
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

func (self *StringList) Remove(e *StringListElement) {
	self.RemoveElement(e)
	e.Prev = nil
	e.Next = self.freeList
	self.freeList = e
}

func (self *StringList) String() string {
	llen := func(l *StringListElement) int {
		len := 0
		for ; l != nil; l = l.Next {
			len++
		}
		return len
	}

	return fmt.Sprintf("StringList<%d entries/%d free>", llen(self.Front), llen(self.freeList))

}
