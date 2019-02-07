package fs
import "github.com/fingon/go-tfhfs/util"

// deleteNotifyIChannel provides infinite buffer channel abstraction. If
// blocking is not really an option, IChannel should be used instead
// of normal Go channel.

// deleteNotifyList must be also generated.

// I dislike 'sizing' channels, as typically the channel full behavior
// is untested and that forces setting large size, and that leads to
// unneccessary inefficiency.

type deleteNotifyIChannel struct {
	list             deleteNotifyList
	lock, waitLock   util.MutexLocked
	started, waiting bool
	receiveChannel   chan deleteNotify
}

func (self *deleteNotifyIChannel) start() {
	self.lock.AssertLocked()
	if self.started {
		return
	}
	self.started = true
	self.receiveChannel = make(chan deleteNotify)
	self.waitLock.Lock()
	go func() {
		for {
			var value deleteNotify
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

func (self *deleteNotifyIChannel) Start() {
	// Opportunistic attempt w/o lock; can't hurt
	if self.started {
		return
	}
	defer self.lock.Locked()()
	self.start()
}

func (self *deleteNotifyIChannel) Send(value deleteNotify) {
	defer self.lock.Locked()()
	self.start()
	self.list.PushBack(value)
	if self.waiting {
		self.waitLock.Unlock()
	}
}

func (self *deleteNotifyIChannel) Channel() chan deleteNotify {
	self.Start()
	return self.receiveChannel
}

func (self *deleteNotifyIChannel) Receive() deleteNotify {
	self.Start()
	return <-self.receiveChannel
}
