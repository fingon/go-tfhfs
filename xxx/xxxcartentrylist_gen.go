package xxx
import "fmt"

// XXXCartEntryList provides doubly linked list which does not have inefficient
// operations, is typesafe, and does minimum amount of extra
// allocations needed. This is accomplished by sticking the freed
// items to a freelist instead of freeing them directly. The list is
// obviously not threadsafe.
type XXXCartEntryList struct {
	Back, Front *XXXCartEntryListElement
	freeList    *XXXCartEntryListElement
	Length      int
}

type XXXCartEntryListElement struct {
	Prev, Next *XXXCartEntryListElement
	Value      *XXXCartEntry
}

func (self *XXXCartEntryList) getElement(v *XXXCartEntry) (e *XXXCartEntryListElement) {
	if self.freeList == nil {
		return &XXXCartEntryListElement{Value: v}
	}
	e = self.freeList
	self.freeList = e.Next
	e.Value = v
	return e
}

func (self *XXXCartEntryList) Iterate(cb func(v *XXXCartEntry)) {
	for e := self.Front; e != nil; e = e.Next {
		cb(e.Value)
	}
}

func (self *XXXCartEntryList) PushBackElement(e *XXXCartEntryListElement) {
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

func (self *XXXCartEntryList) PushBack(v *XXXCartEntry) *XXXCartEntryListElement {
	e := self.getElement(v)
	self.PushBackElement(e)
	return e
}

func (self *XXXCartEntryList) PushFrontElement(e *XXXCartEntryListElement) {
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
func (self *XXXCartEntryList) PushFront(v *XXXCartEntry) *XXXCartEntryListElement {
	e := self.getElement(v)
	self.PushFrontElement(e)
	return e
}

func (self *XXXCartEntryList) RemoveElement(e *XXXCartEntryListElement) {
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

func (self *XXXCartEntryList) Remove(e *XXXCartEntryListElement) {
	self.RemoveElement(e)
	e.Prev = nil
	e.Next = self.freeList
	self.freeList = e
}

func (self *XXXCartEntryList) String() string {
	llen := func(l *XXXCartEntryListElement) int {
		len := 0
		for ; l != nil; l = l.Next {
			len++
		}
		return len
	}

	return fmt.Sprintf("XXXCartEntryList<%d entries/%d free>", llen(self.Front), llen(self.freeList))

}
