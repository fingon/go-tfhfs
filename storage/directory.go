/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2018 Markus Stenberg
 *
 * Created:       Wed Jan  3 15:55:15 2018 mstenber
 * Last modified: Wed Jan  3 16:03:30 2018 mstenber
 * Edit time:     1 min
 *
 */

package storage

import (
	"os"
	"path/filepath"
	"syscall"

	"github.com/fingon/go-tfhfs/mlog"
)

type DirectoryBlockBackendBase struct {
	dir string
}

func (self *DirectoryBlockBackendBase) Init(dir string) *DirectoryBlockBackendBase {
	self.dir = dir
	return self
}

func (self *DirectoryBlockBackendBase) GetBytesAvailable() uint64 {
	var st syscall.Statfs_t
	err := syscall.Statfs(self.dir, &st)
	if err != nil {
		return 0
	}
	r := uint64(st.Bsize) * st.Bfree
	mlog.Printf2("storage/directory", "ba.GetBytesAvailable %v (%v * %v)", r, st.Bsize, st.Bfree)
	return r
}

func (self *DirectoryBlockBackendBase) GetBytesUsed() uint64 {
	var sum uint64
	filepath.Walk(self.dir, func(path string, info os.FileInfo, err error) error {
		sum += uint64(info.Size())
		return nil
	})
	mlog.Printf2("storage/directory", "ba.GetBytesUsed %v", sum)
	return sum
}
