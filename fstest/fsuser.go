/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2017 Markus Stenberg
 *
 * Created:       Fri Dec 29 15:39:36 2017 mstenber
 * Last modified: Fri Dec 29 16:48:42 2017 mstenber
 * Edit time:     47 min
 *
 */

// fstest provides (raw) fuse filesystem code
//
// Tests are mostly written with DummyUser module which provides ~os
// module functionality across the fuse APIs. This does NOT
// intentionally really mount the filesystem for obvious reasons.
//
// (parallel testing, arbitrary permission simulation with nonroot
// user)
package fstest

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/fingon/go-tfhfs/fs"
	"github.com/hanwen/go-fuse/fuse"
)

var ErrNok = errors.New("Non-zero fuse value")

func s2e(status fuse.Status) error {
	if !status.Ok() {
		return ErrNok
	}
	return nil
}

type FSUser struct {
	fuse.InHeader
	fs *fs.Fs
}

type fileInfo struct {
	name  string
	size  int64
	mode  os.FileMode
	mtime time.Time
}

func (self *fileInfo) Name() string {
	return self.name
}

func (self *fileInfo) Size() int64 {
	return self.size
}

func (self *fileInfo) Mode() os.FileMode {
	return self.mode
}

func (self *fileInfo) ModTime() time.Time {
	return self.mtime
}

func (self *fileInfo) IsDir() bool {
	return self.Mode().IsDir()
}

func (self *fileInfo) Sys() interface{} {
	return nil
}

func NewFSUser(f *fs.Fs) *FSUser {
	return &FSUser{fs: f}
}

func (self *FSUser) lookup(path string, eo *fuse.EntryOut) (err error) {
	inode := uint64(fuse.FUSE_ROOT_ID)
	for _, name := range strings.Split(path, "/") {
		if name == "" {
			continue
		}
		self.NodeId = inode
		err = s2e(self.fs.Lookup(&self.InHeader, name, eo))
		if err != nil {
			return
		}
		inode = eo.Ino
	}
	self.NodeId = inode
	err = s2e(self.fs.Lookup(&self.InHeader, ".", eo))
	return
}

func (self *FSUser) ListDir(name string) (ret []string, err error) {
	var eo fuse.EntryOut
	err = self.lookup(name, &eo)
	if err != nil {
		return
	}
	self.NodeId = eo.Ino
	var oo fuse.OpenOut
	err = s2e(self.fs.OpenDir(&fuse.OpenIn{InHeader: self.InHeader}, &oo))
	if err != nil {
		return
	}
	del := fuse.NewDirEntryList(make([]byte, 1000), 0)

	err = s2e(self.fs.ReadDir(&fuse.ReadIn{Fh: oo.Fh,
		InHeader: self.InHeader}, del))
	if err != nil {
		return
	}
	// We got _something_. No way to make sure it was fine. Oh well.
	// Cheat using backdoor API.
	ret = self.fs.ListDir(eo.Ino)
	self.fs.ReleaseDir(&fuse.ReleaseIn{Fh: oo.Fh, InHeader: self.InHeader})
	return
}

func (self *FSUser) ReadDir(dirname string) (ret []os.FileInfo, err error) {
	l, err := self.ListDir(dirname)
	if err != nil {
		return
	}
	ret = make([]os.FileInfo, len(l))
	for i, n := range l {
		var eo fuse.EntryOut
		err = self.lookup(fmt.Sprintf("%s/%s", dirname, n), &eo)
		if err != nil {
			return
		}
		ret[i] = &fileInfo{name: n,
			size:  int64(eo.Size),
			mode:  os.FileMode(eo.Mode),
			mtime: time.Unix(int64(eo.Mtime), int64(eo.Mtimensec))}
	}
	return
}
