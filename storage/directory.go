/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2018 Markus Stenberg
 *
 * Created:       Wed Jan  3 15:55:15 2018 mstenber
 * Last modified: Wed Jan  3 18:12:43 2018 mstenber
 * Edit time:     28 min
 *
 */

package storage

import (
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/fingon/go-tfhfs/mlog"
)

type delayedUInt64ValueCallback func() uint64

type delayedUInt64Value struct {
	interval   time.Duration
	value      uint64
	valueTime  time.Time
	valueMutex sync.Mutex
	going      bool
	callback   delayedUInt64ValueCallback
}

func (self *delayedUInt64Value) Value() uint64 {
	self.valueMutex.Lock()
	defer self.valueMutex.Unlock()
	fun := func() {
		// Calculate value without mutex
		value := self.callback()

		self.valueMutex.Lock()
		defer self.valueMutex.Unlock()

		self.value = value
		self.valueTime = time.Now()
		self.going = false
	}
	if self.going || self.valueTime.Add(self.interval).After(time.Now()) {
		return self.value
	}
	self.going = true
	go fun()
	return self.value

}

type DirectoryBlockBackendBase struct {
	dir string

	// ValueUpdateInterval describes how often cached values (e.g.
	// statfs stuff) are updated _in background_.
	ValueUpdateInterval time.Duration

	available, used delayedUInt64Value
}

func (self *DirectoryBlockBackendBase) Init(dir string) *DirectoryBlockBackendBase {
	self.dir = dir
	minimumInterval := 5 * time.Second
	if self.ValueUpdateInterval < minimumInterval {
		self.ValueUpdateInterval = minimumInterval
	}
	self.available = delayedUInt64Value{interval: self.ValueUpdateInterval,
		callback: func() uint64 { return calculateAvailable(dir) }}
	self.used = delayedUInt64Value{interval: self.ValueUpdateInterval,
		callback: func() uint64 { return calculateUsed(dir) }}
	return self
}

func calculateAvailable(dir string) uint64 {
	var st syscall.Statfs_t
	err := syscall.Statfs(dir, &st)
	if err != nil {
		return 0
	}
	r := uint64(st.Bsize) * st.Bfree
	mlog.Printf2("storage/directory", "ba.GetBytesAvailable %v (%v * %v)", r, st.Bsize, st.Bfree)
	return r
}

func calculateUsed(dir string) (sum uint64) {
	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err == nil {
			sum += uint64(info.Size())
		}
		return nil
	})
	return sum
}

func (self *DirectoryBlockBackendBase) GetBytesAvailable() uint64 {
	return self.available.Value()
}

func (self *DirectoryBlockBackendBase) GetBytesUsed() uint64 {
	return self.used.Value()
}
