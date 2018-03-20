/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2017 Markus Stenberg
 *
 * Created:       Fri Dec 29 15:39:36 2017 mstenber
 * Last modified: Tue Mar 20 11:16:29 2018 mstenber
 * Edit time:     242 min
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
//
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
	"github.com/fingon/go-tfhfs/util"
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
	fs   *Fs
	ops  fuse.RawFileSystem
	lock util.MutexLocked
}

func (self *FSUser) String() string {
	return fmt.Sprintf("u{uid:%v,gid:%v}", self.Uid, self.Gid)
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
	return &FSUser{fs: fs, ops: &fs.Ops}
}

func (self *FSUser) lookup(path string, eo *fuse.EntryOut) (err error) {
	self.lock.AssertLocked()
	mlog.Printf2("fs/fsuser", "lookup %v", path)
	inode := uint64(fuse.FUSE_ROOT_ID)
	oinode := inode
	for _, name := range strings.Split(path, "/") {
		if name == "" {
			continue
		}
		self.NodeId = inode
		mlog.Printf2("fs/fsuser", " %v", name)
		err = s2e(self.ops.Lookup(&self.InHeader, name, eo))
		if err != nil {
			return
		}
		inode = eo.Ino
		self.ops.Forget(inode, 1)
	}
	self.NodeId = inode
	if inode == oinode {
		err = s2e(self.ops.Lookup(&self.InHeader, ".", eo))
		self.ops.Forget(inode, 1)
	}
	return
}

func (self *FSUser) ListDir(name string) (ret []string, err error) {
	defer self.lock.Locked()()
	var eo fuse.EntryOut
	err = self.lookup(name, &eo)
	if err != nil {
		return
	}
	// Cheat using backdoor API.
	ret = self.fs.ListDir(eo.Ino)

	var oo fuse.OpenOut
	err = s2e(self.ops.OpenDir(&fuse.OpenIn{InHeader: self.InHeader}, &oo))
	if err != nil {
		return
	}
	ifile := self.fs.GetFileByFh(oo.Fh)

	// Make sure readdir does not blow up and pretends to iterate
	// (greybox due to painful binary semantics involved)
	lofs := uint64(0)
	for {
		del := fuse.NewDirEntryList(make([]byte, 1000), lofs)
		err = s2e(self.ops.ReadDir(&fuse.ReadIn{Fh: oo.Fh,
			InHeader: self.InHeader, Offset: lofs}, del))
		if err != nil {
			return
		}
		if lofs < ifile.pos {
			lofs = ifile.pos
		} else {
			if int(lofs) != len(ret) {
				s := fmt.Sprintf("rd wrong final pos: %v != %v",
					lofs, len(ret))
				err = errors.New(s)
				return
			}
			break
		}
	}

	if ifile.readNextInodeBruteForceCount != 1 {
		log.Panicf("wrong bruteforcecount (!=1): %d",
			ifile.readNextInodeBruteForceCount)
	}
	// Make sure readdirplus does not blow up and pretends to iterate
	// (greybox due to painful binary semantics involved)
	lofs = 0
	for {
		del := fuse.NewDirEntryList(make([]byte, 1000), lofs)
		err = s2e(self.ops.ReadDirPlus(&fuse.ReadIn{Fh: oo.Fh,
			InHeader: self.InHeader, Offset: lofs}, del))
		if err != nil {
			return
		}
		if lofs < ifile.pos {
			lofs = ifile.pos
		} else {
			if int(lofs) != len(ret) {
				s := fmt.Sprintf("rd+ wrong final pos: %v != %v",
					lofs, len(ret))
				err = errors.New(s)
				return
			}
			break
		}
	}
	if ifile.readNextInodeBruteForceCount != 2 {
		log.Panicf("wrong bruteforcecount (!=2): %d",
			ifile.readNextInodeBruteForceCount)
	}

	// We got _something_. No way to make sure it was fine. Oh well.
	self.ops.ReleaseDir(&fuse.ReleaseIn{Fh: oo.Fh, InHeader: self.InHeader})
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
	defer self.lock.Locked()()
	dirname, basename := filepath.Split(path)

	var eo fuse.EntryOut
	err = self.lookup(dirname, &eo)
	if err != nil {
		return
	}
	err = s2e(self.ops.Mkdir(&fuse.MkdirIn{InHeader: self.InHeader,
		Mode: uint32(perm)}, basename, &eo))
	return
}

// Stat is clone of os.Stat
func (self *FSUser) Stat(path string) (fi os.FileInfo, err error) {
	defer self.lock.Locked()()
	mlog.Printf2("fs/fsuser", "Stat %v", path)
	var eo fuse.EntryOut
	err = self.lookup(path, &eo)
	if err != nil {
		return
	}
	var gai fuse.GetAttrIn
	var ao fuse.AttrOut
	gai.InHeader = self.InHeader
	err = s2e(self.ops.GetAttr(&gai, &ao))
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
	defer self.lock.Locked()()
	dirname, basename := filepath.Split(path)
	var eo fuse.EntryOut
	err = self.lookup(dirname, &eo)
	if err != nil {
		return
	}
	if fi.IsDir() {
		err = s2e(self.ops.Rmdir(&self.InHeader, basename))
	} else {
		err = s2e(self.ops.Unlink(&self.InHeader, basename))
	}
	return
}

// Chown is clone of os.Chown
func (self *FSUser) Chown(path string, uid, gid int) (err error) {
	defer self.lock.Locked()()
	mlog.Printf2("fs/fsuser", "%v.Chown %v : %v %v", self, path, uid, gid)
	var eo fuse.EntryOut
	err = self.lookup(path, &eo)
	if err != nil {
		return
	}
	var sai fuse.SetAttrIn
	sai.InHeader = self.InHeader
	sai.Valid = fuse.FATTR_UID | fuse.FATTR_GID
	sai.Owner.Uid = uint32(uid)
	sai.Owner.Gid = uint32(gid)

	var ao fuse.AttrOut
	err = s2e(self.ops.SetAttr(&sai, &ao))
	return
}

// Chmod is clone of os.Chmod
func (self *FSUser) Chmod(path string, mode os.FileMode) (err error) {
	defer self.lock.Locked()()
	mlog.Printf2("fs/fsuser", "%v.Chmod %v : %v", self, path, mode)
	var eo fuse.EntryOut
	err = self.lookup(path, &eo)
	if err != nil {
		return
	}
	var sai fuse.SetAttrIn
	sai.InHeader = self.InHeader
	sai.Valid = fuse.FATTR_MODE
	sai.Mode = uint32(mode)

	var ao fuse.AttrOut
	err = s2e(self.ops.SetAttr(&sai, &ao))
	return
}

// Chtimes is clone of os.Chtimes
func (self *FSUser) Chtimes(path string, atime time.Time, mtime time.Time) (err error) {
	defer self.lock.Locked()()
	mlog.Printf2("fs/fsuser", "%v.Chtimes %v : %v %v", self, path, atime, mtime)
	var eo fuse.EntryOut
	err = self.lookup(path, &eo)
	if err != nil {
		return
	}
	var sai fuse.SetAttrIn
	sai.InHeader = self.InHeader
	sai.Valid = fuse.FATTR_ATIME | fuse.FATTR_MTIME
	sai.Atime = uint64(atime.Unix())
	sai.Atimensec = uint32(atime.Nanosecond())
	sai.Mtime = uint64(mtime.Unix())
	sai.Mtimensec = uint32(mtime.Nanosecond())

	var ao fuse.AttrOut
	err = s2e(self.ops.SetAttr(&sai, &ao))
	return

}

// Link is clone of os.Link
func (self *FSUser) Link(oldpath, newpath string) (err error) {
	defer self.lock.Locked()()
	mlog.Printf2("fs/fsuser", "%v.Link %v => %v", self, oldpath, newpath)
	var eo fuse.EntryOut
	err = self.lookup(oldpath, &eo)
	if err != nil {
		mlog.Printf2("fs/fsuser", " oldpath %v error %v", oldpath, err)
		return
	}
	var li fuse.LinkIn
	li.Oldnodeid = self.NodeId
	mlog.Printf2("fs/fsuser", " Oldnodeid=%v", li.Oldnodeid)
	dirname, basename := filepath.Split(newpath)
	err = self.lookup(dirname, &eo)
	if err != nil {
		mlog.Printf2("fs/fsuser", " dirname %v error %v", dirname, err)
		return
	}
	li.InHeader = self.InHeader
	err = s2e(self.ops.Link(&li, basename, &eo))
	return
}

// Rename is clone of os.Rename
func (self *FSUser) Rename(oldpath, newpath string) (err error) {
	defer self.lock.Locked()()
	mlog.Printf2("fs/fsuser", "%v.Rename %v => %v", self, oldpath, newpath)
	var eo fuse.EntryOut
	olddirname, oldbasename := filepath.Split(oldpath)
	newdirname, newbasename := filepath.Split(newpath)
	err = self.lookup(newdirname, &eo)
	if err != nil {
		return
	}
	var ri fuse.RenameIn
	ri.Newdir = self.NodeId
	err = self.lookup(olddirname, &eo)
	if err != nil {
		return
	}
	ri.InHeader = self.InHeader
	err = s2e(self.ops.Rename(&ri, oldbasename, newbasename))
	return
}

// Symlink is clone of os.Symlink
func (self *FSUser) Symlink(oldpath, newpath string) (err error) {
	defer self.lock.Locked()()
	mlog.Printf2("fs/fsuser", "%v.Symlink %v => %v %v", self, oldpath, newpath)
	dirname, basename := filepath.Split(newpath)
	var eo fuse.EntryOut
	err = self.lookup(dirname, &eo)
	if err != nil {
		return
	}
	err = s2e(self.ops.Symlink(&self.InHeader, oldpath, basename, &eo))
	return
}

// Readlink is clone of os.Readlink
func (self *FSUser) Readlink(path string) (s string, err error) {
	defer self.lock.Locked()()
	mlog.Printf2("fs/fsuser", "%v.Readlink %v", self, path)
	var eo fuse.EntryOut
	err = self.lookup(path, &eo)
	if err != nil {
		return
	}
	out, code := self.ops.Readlink(&self.InHeader)
	err = s2e(code)
	if err != nil {
		return
	}
	s = string(out)
	return

}

func (self *FSUser) GetXAttr(path, attr string) (b []byte, err error) {
	defer self.lock.Locked()()
	var eo fuse.EntryOut
	err = self.lookup(path, &eo)
	if err != nil {
		return
	}
	b, code := self.ops.GetXAttrData(&self.InHeader, attr)
	err = s2e(code)
	if err != nil {
		return
	}
	l, code := self.ops.GetXAttrSize(&self.InHeader, attr)
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
	defer self.lock.Locked()()
	var eo fuse.EntryOut
	err = self.lookup(path, &eo)
	if err != nil {
		return
	}
	b, code := self.ops.ListXAttr(&self.InHeader)
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
	defer self.lock.Locked()()
	var eo fuse.EntryOut
	err = self.lookup(path, &eo)
	if err != nil {
		return
	}
	return s2e(self.ops.RemoveXAttr(&self.InHeader, attr))
}

func (self *FSUser) SetXAttr(path, attr string, data []byte) (err error) {
	defer self.lock.Locked()()
	var eo fuse.EntryOut
	err = self.lookup(path, &eo)
	if err != nil {
		return
	}
	return s2e(self.ops.SetXAttr(&fuse.SetXAttrIn{InHeader: self.InHeader,
		Size: uint32(len(data))}, attr, data))
}

type fsFile struct {
	path string
	fh   uint64
	u    *FSUser
	pos  int64
}

func (self *fsFile) String() string {
	return fmt.Sprintf("fsFile{%v,fh:%v,pos:%v,u:%v}", self.path, self.fh, self.pos, self.u)
}

func (self *FSUser) OpenFile(path string, flag uint32, perm uint32) (f *fsFile, err error) {
	defer self.lock.Locked()()
	mlog.Printf2("fs/fsuser", "OpenFile %s f:%x perm:%x", path, flag, perm)
	var eo fuse.EntryOut
	var oo fuse.OpenOut
	if flag&uint32(os.O_CREATE) != 0 {
		dirname, basename := filepath.Split(path)
		err = self.lookup(dirname, &eo)
		if err != nil {
			return
		}
		ci := fuse.CreateIn{InHeader: self.InHeader, Flags: flag, Mode: perm}
		var co fuse.CreateOut
		err = s2e(self.ops.Create(&ci, basename, &co))
		oo = co.OpenOut
	} else {
		err = self.lookup(path, &eo)
		if err != nil {
			return
		}
		oi := fuse.OpenIn{InHeader: self.InHeader, Flags: flag, Mode: perm}
		err = s2e(self.ops.Open(&oi, &oo))
	}
	if err != nil {
		return
	}
	f = &fsFile{path: path, fh: oo.Fh, u: self}
	return
}

func (self *fsFile) Close() {
	ri := fuse.ReleaseIn{Fh: self.fh}
	self.u.ops.Release(&ri)
}

func (self *fsFile) Seek(ofs int64, whence int) (ret int64, err error) {
	var fi os.FileInfo
	mlog.Printf2("fs/fsuser", "%v.Seek %v %v", self, ofs, whence)
	fi, err = self.u.Stat(self.path)
	if err != nil {
		mlog.Printf2("fs/fsuser", " Seek encountered stat failure: %s", err)
		return
	}
	ret = ofs
	switch whence {
	case 0:
		// relative to start

	case 1:
		// relative to current offset
		ret += self.pos
	case 2:
		// relative to the end of it
		ret += fi.Size()
	}
	if ret < 0 {
		err = errors.New("seek before start")
		return
	}
	//if ret >= fi.Size() {
	//	err = errors.New("seek after end")
	//	return
	//}
	self.pos = ret
	mlog.Printf2("fs/fsuser", " after Seek: pos now %v", self.pos)
	return
}

func (self *fsFile) Read(b []byte) (n int, err error) {
	mlog.Printf2("fs/fsuser", "Read %d bytes @%v", len(b), self.pos)
	size := uint32(len(b))
	ri := fuse.ReadIn{Fh: self.fh,
		Offset: uint64(self.pos),
		Size:   size}
	r, code := self.u.ops.Read(&ri, b)
	err = s2e(code)
	if err != nil {
		return
	}
	n = r.Size()
	self.pos += int64(n)
	mlog.Printf2("fs/fsuser", " pos now %v", self.pos)
	return
}

func (self *fsFile) Write(b []byte) (n int, err error) {
	mlog.Printf2("fs/fsuser", "%v.Write %d bytes @%v", self, len(b), self.pos)
	size := uint32(len(b))
	wi := fuse.WriteIn{Fh: self.fh,
		Offset: uint64(self.pos),
		Size:   size}
	n32, code := self.u.ops.Write(&wi, b[n:])
	err = s2e(code)
	if err != nil {
		return
	}
	if n32 != size {
		log.Panic("Partial write")
	}
	self.pos += int64(n32)
	mlog.Printf2("fs/fsuser", " pos now %v", self.pos)
	n = len(b)
	return

}
