/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2018 Markus Stenberg
 *
 * Created:       Fri Mar 16 11:09:12 2018 mstenber
 * Last modified: Fri Mar 16 12:57:28 2018 mstenber
 * Edit time:     80 min
 *
 */

package xxx

import (
	"fmt"

	"github.com/fingon/go-tfhfs/mlog"
	"github.com/fingon/go-tfhfs/util"
)

// YYYCart provides CART (Clock with Adaptive Replacement and Temporal
// filtering) cache map of string to YYYType.
//
// For details about CART, see:
// Bansal & Modha 2004, CAR: Clock with Adaptive Replacement paper.
//
// The type is not threadsafe, and requires also YYYList to be
// available for the appropriate type.
//
// variables are as close as possible to ones in paper
type YYYCart struct {
	// cache is the lookup map for entries in T[n] / B[n]
	cache map[string]*YYYCartEntry

	t1, t2, b1, b2 YYYCartEntryList

	c, p, q, ns, nl int
	// c = maximum size
	// p = maximum length of t1
	// q = maximum length of b1
	// ns = number of short-lived entries (in t1+t2)
	// nl = number of long-lived entries (in t1+t2)
}

// YYYCartEntry represents a single cache entry; maps point at it under key
type YYYCartEntry struct {
	key                string
	e                  YYYCartEntryListElement
	refbit, filterlong bool
	frequentbit        bool // if it is in 2 series of lists
	value              *YYYType
}

func (self *YYYCartEntry) String() string {
	return fmt.Sprintf("ce{%s,r:%v,l:%v,f:%v}", self.key, self.refbit, self.filterlong, self.frequentbit)
}

func (self YYYCart) Init(maximumSize int) *YYYCart {
	self.cache = make(map[string]*YYYCartEntry)
	self.c = maximumSize
	return &self
}

func (self *YYYCart) Get(key string) (value YYYType, found bool) {
	mlog.Printf("cart.Get %s", key)
	e, found := self.cache[key]
	if !found {
		mlog.Printf(" not in t/b")
		return
	}
	if e.value == nil {
		mlog.Printf(" not in t")
		found = false
		return
	}
	mlog.Printf(" found")
	e.refbit = true
	value = *e.value
	return
}

func (self *YYYCart) Set(key string, value YYYType) {
	mlog.Printf("cart.Set %v %v", key, value)
	if self.c == 0 {
		mlog.Printf(" not enabled")
		return
	}
	e, found := self.cache[key]
	if found && e.value != nil {
		// cache hit
		e.refbit = true
		return
	}
	if self.t1.Length+self.t2.Length == self.c {
		mlog.Printf(" cache full")
		// cache full; replace page from cache
		self.replace()

		// also clear history space if it missed altogether
		// and history is full
		if !found && self.b1.Length+self.b2.Length > self.c {
			if self.b1.Length > self.q || self.b2.Length == 0 {
				mlog.Printf(" bumped from b1")
				delete(self.cache, self.b1.Front.Value.key)
				self.b1.RemoveElement(self.b1.Front)
			} else {
				mlog.Printf(" bumped from b2")
				delete(self.cache, self.b2.Front.Value.key)
				self.b2.RemoveElement(self.b2.Front)
			}

		}
	}

	if !found {
		mlog.Printf(" added fresh")
		e := YYYCartEntry{key: key, value: &value}
		self.cache[key] = &e
		e.e.Value = &e
		self.t1.PushBackElement(&e.e)
		self.ns++
		return
	}

	if !e.frequentbit {
		mlog.Printf(" b1->t1")
		self.p = util.IMin(self.p+util.IMax(1, self.ns/self.b1.Length), self.c)
		mlog.Printf("  p = %d", self.p)
		e.filterlong = true
		self.b1.RemoveElement(&e.e)
	} else {
		mlog.Printf(" b2->t1")
		e.frequentbit = false
		self.p = util.IMax(self.p-util.IMax(1, self.nl/self.b2.Length), 0)
		mlog.Printf("  p = %d", self.p)
		self.b2.RemoveElement(&e.e)
		if self.t2.Length+self.b2.Length+self.t1.Length-self.ns >= self.c {
			self.q = util.IMin(self.q+1, 2*self.c-self.t1.Length)
			mlog.Printf("  q = %d", self.q)
		}
	}
	self.t1.PushBackElement(&e.e)
	e.value = &value
	e.refbit = false
	self.nl++
}

func (self *YYYCart) replace() {
	// replace() in the paper p11
	mlog.Printf("replace()")
	for self.t2.Front != nil && self.t2.Front.Value.refbit {
		e := self.t2.Front.Value
		mlog.Printf(" moving %s t2->t1", e)
		self.t2.RemoveElement(self.t2.Front)

		e.refbit = false
		e.frequentbit = false
		self.t1.PushBackElement(&e.e)

		if self.t2.Length+self.b2.Length+self.t1.Length-self.ns >= self.c {
			self.q = util.IMin(self.q+1, self.c*2-self.t1.Length)
			mlog.Printf("  q = %d", self.q)
		}
	}
	for self.t1.Front != nil && (self.t1.Front.Value.filterlong || self.t1.Front.Value.refbit) {
		e := self.t1.Front.Value
		if e.refbit {
			mlog.Printf(" moving to head of t1 %v", e)
			self.t1.RemoveElement(&e.e)
			self.t1.PushBackElement(&e.e)
			e.refbit = false
			if self.t1.Length >= util.IMin(self.p+1, self.b1.Length) && !e.filterlong {
				e.filterlong = true
				self.ns--
				self.nl++
			}
		} else {
			mlog.Printf(" promoting t1->t2 %v", e)
			self.t1.RemoveElement(&e.e)

			self.t2.PushBackElement(&e.e)
			self.q = util.IMax(self.q-1, self.c-self.t1.Length)
			mlog.Printf("  q = %d", self.q)
			e.frequentbit = true
		}

	}
	if self.t1.Length >= util.IMax(1, self.p) {
		e := self.t1.Front.Value
		mlog.Printf(" evicting %v from t1", e)
		e.value = nil
		self.t1.RemoveElement(&e.e)
		self.b1.PushBackElement(&e.e)
		self.ns--
	} else {
		e := self.t2.Front.Value
		mlog.Printf(" evicting %v from t2", e)
		e.value = nil
		self.t2.RemoveElement(&e.e)
		self.b2.PushBackElement(&e.e)
		self.nl--
	}

}
