package ibtree
import "fmt"

// NodeDataCartEntryList provides doubly linked list which does not have inefficient
// operations, is typesafe, and does minimum amount of extra
// allocations needed. This is accomplished by sticking the freed
// items to a freelist instead of freeing them directly. The list is
// obviously not threadsafe.
type NodeDataCartEntryList struct {
	Back, Front *NodeDataCartEntryListElement
	freeList    *NodeDataCartEntryListElement
	Length      int
}

type NodeDataCartEntryListElement struct {
	Prev, Next *NodeDataCartEntryListElement
	Value      *NodeDataCartEntry
}

func (self *NodeDataCartEntryList) getElement(v *NodeDataCartEntry) (e *NodeDataCartEntryListElement) {
	if self.freeList == nil {
		return &NodeDataCartEntryListElement{Value: v}
	}
	e = self.freeList
	self.freeList = e.Next
	e.Value = v
	return e
}

func (self *NodeDataCartEntryList) Iterate(cb func(v *NodeDataCartEntry)) {
	for e := self.Front; e != nil; e = e.Next {
		cb(e.Value)
	}
}

func (self *NodeDataCartEntryList) PushBackElement(e *NodeDataCartEntryListElement) {
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

func (self *NodeDataCartEntryList) PushBack(v *NodeDataCartEntry) *NodeDataCartEntryListElement {
	e := self.getElement(v)
	self.PushBackElement(e)
	return e
}

func (self *NodeDataCartEntryList) PushFrontElement(e *NodeDataCartEntryListElement) {
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
func (self *NodeDataCartEntryList) PushFront(v *NodeDataCartEntry) *NodeDataCartEntryListElement {
	e := self.getElement(v)
	self.PushFrontElement(e)
	return e
}

func (self *NodeDataCartEntryList) RemoveElement(e *NodeDataCartEntryListElement) {
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

func (self *NodeDataCartEntryList) Remove(e *NodeDataCartEntryListElement) {
	self.RemoveElement(e)
	e.Prev = nil
	e.Next = self.freeList
	self.freeList = e
}

func (self *NodeDataCartEntryList) String() string {
	llen := func(l *NodeDataCartEntryListElement) int {
		len := 0
		for ; l != nil; l = l.Next {
			len++
		}
		return len
	}

	return fmt.Sprintf("NodeDataCartEntryList<%d entries/%d free>", llen(self.Front), llen(self.freeList))

}
