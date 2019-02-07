package fs
import (
	"sync"

	"github.com/fingon/go-tfhfs/util"
)

// deleteNotifyIChannel provides infinite buffer channel abstraction. If
// blocking is not really an option, IChannel should be used instead
// of normal Go channel.

// deleteNotifyList must be also generated.

// I dislike 'sizing' channels, as typically the channel full behavior
// is untested and that forces setting large size, and that leads to
// unneccessary inefficiency.

type deleteNotifyIChannel struct {
	list           deleteNotifyList
	lock           util.MutexLocked
	started        bool
	cond           *sync.Cond
	receiveChannel chan deleteNotify
}

func (self *deleteNotifyIChannel) start() {
	self.lock.AssertLocked()
	if self.started {
		return
	}
	self.started = true
	self.cond = sync.NewCond(&self.lock)
	self.receiveChannel = make(chan deleteNotify)
	go func() {
		for {
			var value deleteNotify
			self.lock.Lock()
			item := self.list.Front
			if item != nil {
				self.list.RemoveElement(item)
				value = item.Value
			} else {
				self.cond.Wait()
			}
			self.lock.Unlock()
			if item != nil {
				self.receiveChannel <- value
			}
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
	if self.list.Front == nil {
		self.cond.Signal()
	}
	self.list.PushBack(value)
}

func (self *deleteNotifyIChannel) Channel() chan deleteNotify {
	self.Start()
	return self.receiveChannel
}

func (self *deleteNotifyIChannel) Receive() deleteNotify {
	self.Start()
	return <-self.receiveChannel
}
