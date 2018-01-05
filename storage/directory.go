/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2018 Markus Stenberg
 *
 * Created:       Wed Jan  3 15:55:15 2018 mstenber
 * Last modified: Fri Jan  5 12:01:43 2018 mstenber
 * Edit time:     36 min
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
	status     int
	callback   delayedUInt64ValueCallback
}

func (self *delayedUInt64Value) Value() uint64 {
	self.valueMutex.Lock()
	defer self.valueMutex.Unlock()
	if self.status == 0 {
		// Unset values are bit bogus, so let's not do those
		v := self.callback()
		self.update(v)
		return self.value
	}
	if self.status == 1 || self.valueTime.Add(self.interval).After(time.Now()) {
		return self.value
	}
	self.status = 1
	fun := func() {
		// Calculate value without mutex
		value := self.callback()

		self.valueMutex.Lock()
		defer self.valueMutex.Unlock()

		self.update(value)
	}
	go fun()
	// Return old value for now; next call will get updated one
	return self.value

}

func (self *delayedUInt64Value) update(value uint64) {
	self.status = 2
	self.value = value
	self.valueTime = time.Now()
}

type DirectoryBackendBase struct {
	Dir string

	// ValueUpdateInterval describes how often cached values (e.g.
	// statfs stuff) are updated _in background_.
	ValueUpdateInterval time.Duration

	available, used delayedUInt64Value
}

func (self *DirectoryBackendBase) Init(dir string) *DirectoryBackendBase {
	self.Dir = dir
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

func (self *DirectoryBackendBase) GetBytesAvailable() uint64 {
	return self.available.Value()
}

func (self *DirectoryBackendBase) GetBytesUsed() uint64 {
	return self.used.Value()
}
