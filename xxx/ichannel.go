/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2019 Markus Stenberg
 *
 * Created:       Thu Feb  7 10:30:15 2019 mstenber
 * Last modified: Thu Feb  7 10:58:28 2019 mstenber
 * Edit time:     17 min
 *
 */

package xxx

import "github.com/fingon/go-tfhfs/util"

// YYYIChannel provides infinite buffer channel abstraction. If
// blocking is not really an option, IChannel should be used instead
// of normal Go channel.

// YYYList must be also generated.

// I dislike 'sizing' channels, as typically the channel full behavior
// is untested and that forces setting large size, and that leads to
// unneccessary inefficiency.

type YYYIChannel struct {
	list             YYYList
	lock, waitLock   util.MutexLocked
	started, waiting bool
	receiveChannel   chan YYYType
}

func (self *YYYIChannel) start() {
	self.lock.AssertLocked()
	if self.started {
		return
	}
	self.started = true
	self.receiveChannel = make(chan YYYType)
	self.waitLock.Lock()
	go func() {
		for {
			var value YYYType
			self.lock.Lock()
			item := self.list.Front
			if item != nil {
				self.list.RemoveElement(item)
				value = item.Value
				self.waiting = false
			} else {
				self.waiting = true
			}
			self.lock.Unlock()
			if item == nil {
				self.waitLock.Lock()
				continue
			}
			self.receiveChannel <- value
		}
	}()
}

func (self *YYYIChannel) Start() {
	// Opportunistic attempt w/o lock; can't hurt
	if self.started {
		return
	}
	defer self.lock.Locked()()
	self.start()
}

func (self *YYYIChannel) Send(value YYYType) {
	defer self.lock.Locked()()
	self.start()
	self.list.PushBack(value)
	if self.waiting {
		self.waitLock.Unlock()
	}
}

func (self *YYYIChannel) Channel() chan YYYType {
	self.Start()
	return self.receiveChannel
}

func (self *YYYIChannel) Receive() YYYType {
	self.Start()
	return <-self.receiveChannel
}
