/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2017 Markus Stenberg
 *
 * Created:       Fri Dec 29 15:39:36 2017 mstenber
 * Last modified: Tue Jan  2 00:04:02 2018 mstenber
 * Edit time:     94 min
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
package fs

import (
	"bytes"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fingon/go-tfhfs/mlog"
	"github.com/hanwen/go-fuse/fuse"
)

func s2e(status fuse.Status) error {
	if !status.Ok() {
		return errors.New(fmt.Sprintf("%s", status.String()))
	}
	return nil
}

type FSUser struct {
	fuse.InHeader
	fs *Fs
}

type fileInfo struct {
	name       string
	size       int64
	mode       os.FileMode
	mtime      time.Time
	PrevNodeId uint64
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

func fileModeFromFuse(mode uint32) os.FileMode {
	var r os.FileMode
	translate := func(mask uint32, bits os.FileMode) {
		if (mode & mask) != 0 {
			mode = mode & ^mask
			r = r | bits
		}
	}
	translate(uint32(os.ModePerm), os.FileMode(mode)&os.ModePerm) // UNIX permissions
	translate(fuse.S_IFDIR, os.ModeDir)
	translate(fuse.S_IFLNK, os.ModeSymlink)
	translate(fuse.S_IFIFO, os.ModeNamedPipe)
	return r
}

func NewFSUser(fs *Fs) *FSUser {
	return &FSUser{fs: fs}
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
	err = s2e(self.fs.ReadDirPlus(&fuse.ReadIn{Fh: oo.Fh,
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

// ReadDir is clone of ioutil.ReadDir
func (self *FSUser) ReadDir(dirname string) (ret []os.FileInfo, err error) {
	mlog.Printf2("fs/fsuser", "ReadDir %s", dirname)
	l, err := self.ListDir(dirname)
	if err != nil {
		return
	}
	mlog.Printf2("fs/fsuser", " ListDir:%v", l)
	ret = make([]os.FileInfo, len(l))
	for i, n := range l {
		ret[i], err = self.Stat(fmt.Sprintf("%s/%s", dirname, n))
		if err != nil {
			return
		}
	}
	return
}

// MkDir is clone of os.MkDir
func (self *FSUser) Mkdir(path string, perm os.FileMode) (err error) {
	dirname, basename := filepath.Split(path)

	var eo fuse.EntryOut
	err = self.lookup(dirname, &eo)
	if err != nil {
		return
	}
	err = s2e(self.fs.Mkdir(&fuse.MkdirIn{InHeader: self.InHeader,
		Mode: uint32(perm)}, basename, &eo))
	return
}

// Stat is clone of os.Stat
func (self *FSUser) Stat(path string) (fi os.FileInfo, err error) {
	var eo fuse.EntryOut
	err = self.lookup(path, &eo)
	if err != nil {
		return
	}
	_, basename := filepath.Split(path)
	fi = &fileInfo{name: basename,
		size:  int64(eo.Size),
		mode:  fileModeFromFuse(eo.Mode),
		mtime: time.Unix(int64(eo.Mtime), int64(eo.Mtimensec))}
	return
}

// Remove is clone of os.Remove
func (self *FSUser) Remove(path string) (err error) {
	fi, err := self.Stat(path)
	if err != nil {
		return
	}
	dirname, basename := filepath.Split(path)
	var eo fuse.EntryOut
	err = self.lookup(dirname, &eo)
	if err != nil {
		return
	}
	if fi.IsDir() {
		err = s2e(self.fs.Rmdir(&self.InHeader, basename))
	} else {
		err = s2e(self.fs.Unlink(&self.InHeader, basename))
	}
	return
}

func (self *FSUser) GetXAttr(path, attr string) (b []byte, err error) {
	var eo fuse.EntryOut
	err = self.lookup(path, &eo)
	if err != nil {
		return
	}
	b, code := self.fs.GetXAttrData(&self.InHeader, attr)
	err = s2e(code)
	if err != nil {
		return
	}
	l, code := self.fs.GetXAttrSize(&self.InHeader, attr)
	err = s2e(code)
	if err != nil {
		return
	}
	if l != len(b) {
		log.Panic("length mismatch in GetXAttrSize", l, len(b))
	}
	return
}

func (self *FSUser) ListXAttr(path string) (s []string, err error) {
	var eo fuse.EntryOut
	err = self.lookup(path, &eo)
	if err != nil {
		return
	}
	b, code := self.fs.ListXAttr(&self.InHeader)
	err = s2e(code)
	if err != nil {
		return
	}
	bl := bytes.Split(b, []byte{0})
	s = make([]string, len(bl)-1) // always at least one extra
	for i, v := range bl[:len(bl)-1] {
		s[i] = string(v)
	}
	return
}

func (self *FSUser) RemoveXAttr(path, attr string) (err error) {
	var eo fuse.EntryOut
	err = self.lookup(path, &eo)
	if err != nil {
		return
	}
	return s2e(self.fs.RemoveXAttr(&self.InHeader, attr))
}

func (self *FSUser) SetXAttr(path, attr string, data []byte) (err error) {
	var eo fuse.EntryOut
	err = self.lookup(path, &eo)
	if err != nil {
		return
	}
	return s2e(self.fs.SetXAttr(&fuse.SetXAttrIn{InHeader: self.InHeader,
		Size: uint32(len(data))}, attr, data))
}
