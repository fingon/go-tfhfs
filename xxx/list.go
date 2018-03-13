/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2018 Markus Stenberg
 *
 * Created:       Sun Jan  7 16:53:09 2018 mstenber
 * Last modified: Tue Mar 13 15:33:59 2018 mstenber
 * Edit time:     23 min
 *
 */

package xxx

import "fmt"

// YYYList provides doubly linked list which does not have inefficient
// operations, is typesafe, and does minimum amount of extra
// allocations needed. This is accomplished by sticking the freed
// items to a freelist instead of freeing them directly. The list is
// obviously not threadsafe.
type YYYList struct {
	Back, Front *YYYListElement
	freeList    *YYYListElement
}

func (self *YYYList) getElement(v YYYType) (e *YYYListElement) {
	if self.freeList == nil {
		return &YYYListElement{Value: v}
	}
	e = self.freeList
	self.freeList = e.Next
	e.Next = nil
	e.Value = v
	return e
}

func (self *YYYList) Iterate(cb func(v YYYType)) {
	for e := self.Front; e != nil; e = e.Next {
		cb(e.Value)
	}
}

func (self *YYYList) PushBack(v YYYType) *YYYListElement {
	e := self.getElement(v)
	e.Prev = self.Back
	if self.Back != nil {
		self.Back.Next = e
	}
	if self.Front == nil {
		self.Front = e
	}
	self.Back = e
	return e
}

func (self *YYYList) PushFront(v YYYType) *YYYListElement {
	e := self.getElement(v)
	e.Next = self.Front
	if self.Front != nil {
		self.Front.Prev = e
	}
	if self.Back == nil {
		self.Back = e
	}
	self.Front = e
	return e
}

func (self *YYYList) Remove(e *YYYListElement) {
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

func (self *YYYList) String() string {
	llen := func(l *YYYListElement) int {
		len := 0
		for ; l != nil; l = l.Next {
			len++
		}
		return len
	}

	return fmt.Sprintf("YYYList<%d entries/%d free>", llen(self.Front), llen(self.freeList))

}

type YYYListElement struct {
	Prev, Next *YYYListElement
	Value      YYYType
}
