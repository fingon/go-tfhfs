/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2018 Markus Stenberg
 *
 * Created:       Sun Jan  7 16:53:09 2018 mstenber
 * Last modified: Sun Jan  7 17:23:26 2018 mstenber
 * Edit time:     16 min
 *
 */

package xxx

// XXXList provides doubly linked list which does not have inefficient
// operations, is typesafe, and does minimum amount of extra
// allocations needed. This is accomplished by sticking the freed
// items to a freelist instead of freeing them directly. The list is
// obviously not threadsafe.
type XXXList struct {
	Back, Front *XXXListElement
	freeList    *XXXListElement
}

func (self *XXXList) getElement(v XXXType) (e *XXXListElement) {
	if self.freeList == nil {
		return &XXXListElement{Value: v}
	}
	e = self.freeList
	self.freeList = e.Next
	e.Next = nil
	e.Value = v
	return e
}

func (self *XXXList) PushBack(v XXXType) {
	e := self.getElement(v)
	e.Prev = self.Back
	if self.Back != nil {
		self.Back.Next = e
	}
	if self.Front == nil {
		self.Front = e
	}
	self.Back = e
}

func (self *XXXList) PushFront(v XXXType) {
	e := self.getElement(v)
	e.Next = self.Front
	if self.Front != nil {
		self.Front.Prev = e
	}
	if self.Back == nil {
		self.Back = e
	}
	self.Front = e
}

func (self *XXXList) Remove(e *XXXListElement) {
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
	e.Prev = nil
	e.Next = self.freeList
	self.freeList = e
}

type XXXListElement struct {
	Prev, Next *XXXListElement
	Value      XXXType
}
