/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2018 Markus Stenberg
 *
 * Created:       Fri Mar 16 11:09:12 2018 mstenber
 * Last modified: Sat Mar 17 11:30:44 2018 mstenber
 * Edit time:     106 min
 *
 */

package xxx

import (
	"fmt"

	"github.com/fingon/go-tfhfs/mlog"
	"github.com/fingon/go-tfhfs/util"
)

// XXXCart provides CART (Clock with Adaptive Replacement and Temporal
// filtering) cache map of string-ish ZZZType to XXXType.
//
// For details about CART, see:
// Bansal & Modha 2004, CAR: Clock with Adaptive Replacement paper.
//
// The type is not threadsafe, and requires also YYYList to be
// available for XXXCartEntryList.
//
// variables are as close as possible to ones in paper
type XXXCart struct {
	// cache is the lookup map for entries in T[n] / B[n]
	cache map[ZZZType]*XXXCartEntry

	t1, t2, b1, b2 XXXCartEntryList

	c, p, q, ns, nl int
	// c = maximum size
	// p = maximum length of t1
	// q = maximum length of b1
	// ns = number of short-lived entries (in t1+t2)
	// nl = number of long-lived entries (in t1+t2)
}

// XXXCartEntry represents a single cache entry; maps point at it under key
type XXXCartEntry struct {
	key                ZZZType
	e                  XXXCartEntryListElement
	refbit, filterlong bool
	frequentbit        bool // if it is in 2 series of lists
	value              XXXType
}

func (self *XXXCartEntry) String() string {
	return fmt.Sprintf("ce{%s,r:%v,l:%v,f:%v}", self.key, self.refbit, self.filterlong, self.frequentbit)
}

func (self *XXXCart) Init(maximumSize int) *XXXCart {
	self.cache = make(map[ZZZType]*XXXCartEntry)
	self.c = maximumSize
	return self
}

// Get retrieves the key, and returns the value if found, and
// indicates in found if it was found or not.
func (self *XXXCart) Get(key ZZZType) (value XXXType, found bool) {
	mlog.Printf2("xxx/cart", "cart.Get %s", key)
	e, found := self.cache[key]
	if !found {
		mlog.Printf2("xxx/cart", " not in t/b")
		return
	}
	if e.value == nil {
		mlog.Printf2("xxx/cart", " not in t")
		found = false
		return
	}
	mlog.Printf2("xxx/cart", " found")
	e.refbit = true
	value = e.value
	return
}

// GetOrCreate uses Get first, and then calls factory if Get
// fails. The value is returned, as well as whether or not it was
// created.
func (self *XXXCart) GetOrCreate(key ZZZType, factory func(key ZZZType) XXXType) (value XXXType, created bool) {
	value, found := self.Get(key)
	if found {
		return value, false
	}
	value = factory(key)
	self.Set(key, value)
	return value, true
}

// Set sets the key to value. If value is nil, the key is cleared
// instead.
func (self *XXXCart) Set(key ZZZType, value XXXType) {
	mlog.Printf2("xxx/cart", "cart.Set %v %v", key, value)
	if self.c == 0 {
		mlog.Printf2("xxx/cart", " not enabled")
		return
	}
	e, found := self.cache[key]
	if value == nil {
		// just like in gcache, setting nil = delete.
		if found && e.value != nil {
			if e.frequentbit {
				self.t1.RemoveElement(&e.e)
			} else {
				self.t2.RemoveElement(&e.e)
			}
			if e.filterlong {
				self.nl--
			} else {
				self.ns--
			}
			e.value = nil
		}
		return
	}
	if found && e.value != nil {
		// cache hit
		e.refbit = true
		e.value = value
		return
	}
	if self.t1.Length+self.t2.Length == self.c {
		mlog.Printf2("xxx/cart", " cache full")
		// cache full; replace page from cache
		self.replace()

		// also clear history space if it missed altogether
		// and history is full
		if !found && self.b1.Length+self.b2.Length > self.c {
			if self.b1.Length > self.q || self.b2.Length == 0 {
				mlog.Printf2("xxx/cart", " bumped from b1")
				delete(self.cache, self.b1.Front.Value.key)
				self.b1.RemoveElement(self.b1.Front)
			} else {
				mlog.Printf2("xxx/cart", " bumped from b2")
				delete(self.cache, self.b2.Front.Value.key)
				self.b2.RemoveElement(self.b2.Front)
			}

		}
	}

	if !found {
		mlog.Printf2("xxx/cart", " added fresh")
		e := XXXCartEntry{key: key, value: value}
		self.cache[key] = &e
		e.e.Value = &e
		self.t1.PushBackElement(&e.e)
		self.ns++
		return
	}

	if !e.frequentbit {
		mlog.Printf2("xxx/cart", " b1->t1")
		self.p = util.IMin(self.p+util.IMax(1, self.ns/self.b1.Length), self.c)
		mlog.Printf2("xxx/cart", "  p = %d", self.p)
		e.filterlong = true
		self.b1.RemoveElement(&e.e)
	} else {
		mlog.Printf2("xxx/cart", " b2->t1")
		e.frequentbit = false
		self.p = util.IMax(self.p-util.IMax(1, self.nl/self.b2.Length), 0)
		mlog.Printf2("xxx/cart", "  p = %d", self.p)
		self.b2.RemoveElement(&e.e)
		if self.t2.Length+self.b2.Length+self.t1.Length-self.ns >= self.c {
			self.q = util.IMin(self.q+1, 2*self.c-self.t1.Length)
			mlog.Printf2("xxx/cart", "  q = %d", self.q)
		}
		// as it comes from b2, it already has filterlong set
	}
	self.t1.PushBackElement(&e.e)
	e.value = value
	e.refbit = false
	self.nl++
}

func (self *XXXCart) replace() {
	// replace() in the paper p11
	mlog.Printf2("xxx/cart", "replace()")
	for self.t2.Front != nil && self.t2.Front.Value.refbit {
		e := self.t2.Front.Value
		mlog.Printf2("xxx/cart", " moving %s t2->t1", e)
		self.t2.RemoveElement(self.t2.Front)

		e.refbit = false
		e.frequentbit = false
		self.t1.PushBackElement(&e.e)

		if self.t2.Length+self.b2.Length+self.t1.Length-self.ns >= self.c {
			self.q = util.IMin(self.q+1, self.c*2-self.t1.Length)
			mlog.Printf2("xxx/cart", "  q = %d", self.q)
		}
	}
	for self.t1.Front != nil && (self.t1.Front.Value.filterlong || self.t1.Front.Value.refbit) {
		e := self.t1.Front.Value
		if e.refbit {
			mlog.Printf2("xxx/cart", " moving to head of t1 %v", e)
			self.t1.RemoveElement(&e.e)
			self.t1.PushBackElement(&e.e)
			e.refbit = false
			if self.t1.Length >= util.IMin(self.p+1, self.b1.Length) && !e.filterlong {
				e.filterlong = true
				self.ns--
				self.nl++
			}
		} else {
			mlog.Printf2("xxx/cart", " promoting t1->t2 %v", e)
			self.t1.RemoveElement(&e.e)

			self.t2.PushBackElement(&e.e)
			self.q = util.IMax(self.q-1, self.c-self.t1.Length)
			mlog.Printf2("xxx/cart", "  q = %d", self.q)
			e.frequentbit = true
		}

	}
	if self.t1.Length >= util.IMax(1, self.p) {
		e := self.t1.Front.Value
		mlog.Printf2("xxx/cart", " evicting %v from t1", e)
		e.value = nil
		self.t1.RemoveElement(&e.e)
		self.b1.PushBackElement(&e.e)
		self.ns--
	} else {
		e := self.t2.Front.Value
		mlog.Printf2("xxx/cart", " evicting %v from t2", e)
		e.value = nil
		self.t2.RemoveElement(&e.e)
		self.b2.PushBackElement(&e.e)
		self.nl--
	}

}
