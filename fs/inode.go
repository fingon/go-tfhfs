/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2017 Markus Stenberg
 *
 * Created:       Fri Dec 29 08:21:32 2017 mstenber
 * Last modified: Wed Jan  3 11:18:37 2018 mstenber
 * Edit time:     214 min
 *
 */

package fs

import (
	"encoding/binary"
	"log"
	"math/rand"
	"time"

	"github.com/fingon/go-tfhfs/ibtree"
	"github.com/fingon/go-tfhfs/mlog"
	"github.com/fingon/go-tfhfs/util"
	"github.com/hanwen/go-fuse/fuse"
)

type inode struct {
	ino     uint64
	tracker *inodeTracker
	refcnt  uint64
	meta    *InodeMeta
}

func (self *inode) AddChild(name string, child *inode) {
	mlog.Printf2("fs/inode", "inode.AddChild %v = %v", name, child)
	tr := self.Fs().GetTransaction()
	k := NewblockKeyDirFilename(self.ino, name)
	rk := NewblockKeyReverseDirFilename(child.ino, self.ino, name)
	tr.Set(ibtree.IBKey(k), string(util.Uint64Bytes(child.ino)))
	tr.Set(ibtree.IBKey(rk), "")
	meta := child.Meta()
	meta.SetCTimeNow()
	meta.StNlink++
	child.SetMeta(meta)

	meta = self.Meta()
	meta.SetMTimeNow()
	meta.Nchildren++
	self.SetMeta(meta)

	self.Fs().CommitTransaction(tr)
}

func (self *inode) Fs() *Fs {
	return self.tracker.fs
}

func (self *inode) Ops() *fsOps {
	return &self.tracker.fs.ops
}

func unixNanoToFuse(t uint64, seconds *uint64, parts *uint32) {
	*seconds = t / uint64(time.Second)
	*parts = uint32(t % uint64(time.Second))
}

func (self *inode) FillAttr(out *fuse.Attr) fuse.Status {
	// EntryOut.Attr
	meta := self.Meta()
	if meta == nil {
		return fuse.ENOENT
	}
	out.Ino = self.ino
	out.Size = meta.StSize
	out.Blocks = meta.StSize / blockSize
	unixNanoToFuse(meta.StAtimeNs, &out.Atime, &out.Atimensec)
	unixNanoToFuse(meta.StCtimeNs, &out.Ctime, &out.Ctimensec)
	unixNanoToFuse(meta.StMtimeNs, &out.Mtime, &out.Mtimensec)
	out.Mode = meta.StMode
	out.Nlink = meta.StNlink
	out.Uid = meta.StUid
	out.Gid = meta.StGid
	out.Rdev = meta.StRdev
	return fuse.OK
}

func (self *inode) FillAttrOut(out *fuse.AttrOut) fuse.Status {
	out.AttrValid = attrValidity
	out.AttrValidNsec = 0
	if out.Nlink == 0 {
		// out.Nlink = 1
		// original hanwen's work does this, is this really
		// necessary? (allegedly new kernels have issues with
		// nlink=0 + link)
	}
	return self.FillAttr(&out.Attr)
}

func (self *inode) FillEntryOut(out *fuse.EntryOut) fuse.Status {
	// EntryOut
	out.NodeId = self.ino
	out.Generation = 0
	out.EntryValid = entryValidity
	out.AttrValid = attrValidity
	out.EntryValidNsec = 0
	out.AttrValidNsec = 0

	// Implicitly refer to us as well
	self.Refer()

	return self.FillAttr(&out.Attr)
}

func (self *inode) GetChildByName(name string) *inode {
	mlog.Printf2("fs/inode", "GetChildByName %s", name)
	k := NewblockKeyDirFilename(self.ino, name)
	tr := self.Fs().GetTransaction()
	v := tr.Get(ibtree.IBKey(k))
	if v == nil {
		mlog.Printf2("fs/inode", " child key %x not in tree", k)
		return nil
	}
	ino := binary.BigEndian.Uint64([]byte(*v))
	mlog.Printf2("fs/inode", " inode %v", ino)
	return self.tracker.GetInode(ino)
}

func (self *inode) GetFile(flags uint32) *inodeFH {
	file := &inodeFH{inode: self, flags: flags}
	self.tracker.AddFile(file)
	self.Refer()
	return file
}

func (self *inode) GetXAttr(attr string) (data []byte, code fuse.Status) {
	k := NewblockKey(self.ino, BST_XATTR, attr)
	tr := self.Fs().GetTransaction()
	v := tr.Get(ibtree.IBKey(k))
	if v == nil {
		code = fuse.ENOATTR
		return
	}
	data = []byte(*v)
	code = fuse.OK
	return
}

func (self *inode) IterateSubTypeKeys(bst BlockSubType,
	keycb func(key blockKey) bool) {
	tr := self.Fs().GetTransaction()
	k := NewblockKey(self.ino, bst, "")
	for {
		nkeyp := tr.NextKey(ibtree.IBKey(k))
		if nkeyp == nil {
			return
		}
		nkey := blockKey(*nkeyp)
		if nkey.Ino() != self.ino || nkey.SubType() != bst {
			return
		}
		if !keycb(nkey) {
			return
		}
		k = nkey
	}

}

func (self *inode) RemoveXAttr(attr string) (code fuse.Status) {
	k := ibtree.IBKey(NewblockKey(self.ino, BST_XATTR, attr))
	mlog.Printf2("fs/inode", "RemoveXAttr %s - deleting %x", attr, k)
	tr := self.Fs().GetTransaction()
	v := tr.Get(k)
	if v == nil {
		code = fuse.ENOATTR
		return
	}
	tr.Delete(k)
	self.Fs().CommitTransaction(tr)
	return fuse.OK
}

func (self *inode) SetXAttr(attr string, data []byte) (code fuse.Status) {
	k := NewblockKey(self.ino, BST_XATTR, attr)
	mlog.Printf2("fs/inode", "SetXAttr %s - setting %x", attr, k)
	tr := self.Fs().GetTransaction()
	tr.Set(ibtree.IBKey(k), string(data))
	self.Fs().CommitTransaction(tr)
	return fuse.OK
}

func (self *inode) IsDir() bool {
	meta := self.Meta()
	return meta != nil && (meta.StMode&fuse.S_IFDIR) != 0
}

func (self *inode) IsFile() bool {
	meta := self.Meta()
	return meta != nil && (meta.StMode&fuse.S_IFREG) != 0
}

func (self *inode) IsLink() bool {
	meta := self.Meta()
	return meta != nil && (meta.StMode&fuse.S_IFLNK) != 0
}

func (self *inode) Refer() {
	self.refcnt++
}

func (self *inode) Forget(refcnt uint64) {
	self.refcnt -= refcnt
	if self.refcnt == 0 {
		// TBD if there's something else that should be done?
		delete(self.tracker.ino2inode, self.ino)
	}
}

func (self *inode) Release() {
	if self == nil {
		return
	}
	self.Forget(1)
}

func (self *inode) RemoveChildByName(name string) {
	mlog.Printf2("fs/inode", "inode.RemoveChildByName %v", name)
	child := self.GetChildByName(name)
	defer child.Release()
	if child == nil {
		mlog.Printf2("fs/inode", " not found")
		return
	}
	tr := self.Fs().GetTransaction()
	k := NewblockKeyDirFilename(self.ino, name)
	rk := NewblockKeyReverseDirFilename(child.ino, self.ino, name)
	tr.Delete(ibtree.IBKey(k))
	tr.Delete(ibtree.IBKey(rk))
	meta := child.Meta()
	meta.StNlink--
	meta.SetCTimeNow()
	child.SetMeta(meta)

	meta = self.Meta()
	meta.Nchildren--
	meta.SetMTimeNow()
	self.SetMeta(meta)

	mlog.Printf2("fs/inode", " Removed %v", child)
	self.Fs().CommitTransaction(tr)

	if self.Fs().server != nil {
		self.Fs().server.DeleteNotify(self.ino, child.ino, name)
	}
}

// Meta caches the current metadata for particular inode.
// It is valid for the duration of the inode, within validity period anyway.
func (self *inode) Meta() *InodeMeta {
	if self.meta == nil {
		mlog.Printf2("fs/inode", "inode.Meta #%d", self.ino)
		k := NewblockKey(self.ino, BST_META, "")
		tr := self.Fs().GetTransaction()
		v := tr.Get(ibtree.IBKey(k))
		if v == nil {
			mlog.Printf2("fs/inode", " not found")
			return nil
		}
		var m InodeMeta
		_, err := m.UnmarshalMsg([]byte(*v))
		if err != nil {
			log.Panic(err)
		}
		mlog.Printf2("fs/inode", " = %v", &m)
		self.meta = &m
	}
	return self.meta
}

func (self *inode) SetMeta(meta *InodeMeta) {
	times := 0

	if meta.StAtimeNs == 0 {
		times |= 1
	}
	if meta.StCtimeNs == 0 {
		times |= 2
	}
	if meta.StMtimeNs == 0 {
		times |= 4
	}
	if times != 0 {
		meta.setTimesNow(times&1 != 0, times&2 != 0, times&4 != 0)
	}
	k := NewblockKey(self.ino, BST_META, "")
	tr := self.Fs().GetTransaction()
	b, err := meta.MarshalMsg(nil)
	if err != nil {
		log.Panic(err)
	}
	tr.Set(ibtree.IBKey(k), string(b))
	self.Fs().CommitTransaction(tr)
	mlog.Printf2("fs/inode", "inode.SetMeta #%d = %v", self.ino, meta)
	self.meta = meta
}

func (self *inode) SetSize(size uint64) {
	meta := self.Meta()
	shrink := false
	if size == meta.StSize {
		return
	} else if size < meta.StSize && meta.StSize > dataExtentSize {
		shrink = true
	}
	meta.StSize = size
	if size > embeddedSize {
		meta.Data = []byte{}
	}
	self.SetMeta(meta)
	if shrink {
		tr := self.Fs().GetTransaction()
		nextKey := NewblockKeyOffset(self.ino, size+dataExtentSize)
		mlog.Printf2("fs/inode", "SetSize shrinking inode %v - %x+ gone", self.ino, nextKey)
		lastKey := NewblockKeyOffset(self.ino, 1<<62)
		tr.DeleteRange(ibtree.IBKey(nextKey), ibtree.IBKey(lastKey))
		self.Fs().CommitTransaction(tr)
	}

}

type inodeNumberGenerator interface {
	CreateInodeNumber() uint64
}

type randomInodeNumberGenerator struct {
}

func (self *randomInodeNumberGenerator) CreateInodeNumber() uint64 {
	return rand.Uint64()
}

type inodeTracker struct {
	generator inodeNumberGenerator
	ino2inode map[uint64]*inode
	fh2ifile  map[uint64]*inodeFH
	fs        *Fs
	nextFh    uint64
}

func (self *inodeTracker) Init(fs *Fs) {
	self.ino2inode = make(map[uint64]*inode)
	self.fh2ifile = make(map[uint64]*inodeFH)
	self.fs = fs
	self.nextFh = 1
	if self.generator == nil {
		self.generator = &randomInodeNumberGenerator{}
	}
}

func (self *inodeTracker) AddFile(file *inodeFH) {
	self.nextFh++
	fh := self.nextFh
	file.fh = fh
	self.fh2ifile[fh] = file
}

func (self *inodeTracker) getinode(ino uint64) *inode {
	n := self.ino2inode[ino]
	if n == nil {
		n = &inode{ino: ino, tracker: self}
		self.ino2inode[ino] = n
	}
	n.refcnt++
	return n
}

func (self *inodeTracker) GetInode(ino uint64) *inode {
	mlog.Printf2("fs/inode", "GetInode %v", ino)
	inode := self.getinode(ino)
	if inode.Meta() == nil {
		mlog.Printf2("fs/inode", " no meta")
		inode.Release()
		return nil
	}
	mlog.Printf2("fs/inode", " valid")
	return inode
}

func (self *inodeTracker) GetFileByFh(fh uint64) *inodeFH {
	return self.fh2ifile[fh]
}

func (self *inodeTracker) CreateInode() *inode {
	mlog.Printf2("fs/inode", "CreateInode")
	for {
		ino := self.generator.CreateInodeNumber()
		mlog.Printf2("fs/inode", " %v", ino)
		if ino == 0 || self.ino2inode[ino] != nil {
			continue
		}

		// Potentially interesting. See if it is on disk.
		inode := self.getinode(ino)
		if inode.Meta() != nil {
			inode.Release()
			continue
		}

		// We have fresh inode for ourselves!
		return inode
	}
}

// Misc utility stuff

func (self *InodeMetaData) SetMkdirIn(input *fuse.MkdirIn) {
	self.StUid = input.Uid
	self.StGid = input.Gid
	self.StMode = input.Mode | fuse.S_IFDIR & ^input.Umask
}

func (self *InodeMetaData) SetCreateIn(input *fuse.CreateIn) {
	self.StUid = input.Uid
	self.StGid = input.Gid
	self.StMode = input.Mode | fuse.S_IFREG
	// & ^input.Umask
	// (Linux-only -> CBA for now)
}

func (self *InodeMetaData) SetMknodIn(input *fuse.MknodIn) {
	self.StUid = input.Uid
	self.StGid = input.Gid
	self.StMode = input.Mode
	self.StRdev = input.Rdev
}

func (self *InodeMetaData) setTimeValues(atime, ctime, mtime *time.Time) {
	if atime != nil {
		self.StAtimeNs = uint64(atime.UnixNano())
	}
	if ctime != nil {
		self.StCtimeNs = uint64(ctime.UnixNano())
	}
	if mtime != nil {
		self.StMtimeNs = uint64(mtime.UnixNano())
	}

}

func (self *InodeMetaData) setTimesNow(uatime, uctime, umtime bool) {
	now := time.Now()
	var atime, ctime, mtime *time.Time
	if uatime {
		atime = &now
	}
	if umtime {
		mtime = &now
	}
	if uctime {
		ctime = &now
	}
	self.setTimeValues(atime, ctime, mtime)

}

func (self *InodeMetaData) SetATimeNow() {
	self.setTimesNow(true, false, false)
}

func (self *InodeMetaData) SetCTimeNow() {
	self.setTimesNow(true, true, false)
}

func (self *InodeMetaData) SetMTimeNow() {
	self.setTimesNow(true, false, true)
}
