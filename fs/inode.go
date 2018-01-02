/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2017 Markus Stenberg
 *
 * Created:       Fri Dec 29 08:21:32 2017 mstenber
 * Last modified: Tue Jan  2 14:18:01 2018 mstenber
 * Edit time:     167 min
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

type Inode struct {
	ino     uint64
	tracker *InodeTracker
	refcnt  uint64
	meta    *InodeMeta
}

func (self *Inode) AddChild(name string, child *Inode) {
	mlog.Printf2("fs/inode", "Inode.AddChild %v = %v", name, child)
	tr := self.Fs().GetTransaction()
	k := NewBlockKeyDirFilename(self.ino, name)
	rk := NewBlockKeyReverseDirFilename(child.ino, self.ino, name)
	tr.Set(ibtree.IBKey(k), string(util.Uint64Bytes(child.ino)))
	tr.Set(ibtree.IBKey(rk), "")
	meta := child.Meta()
	meta.StNlink++
	child.SetMeta(meta)

	meta = self.Meta()
	meta.Nchildren++
	self.SetMeta(meta)

	self.Fs().CommitTransaction(tr)
}

func (self *Inode) Fs() *Fs {
	return self.tracker.fs
}

func (self *Inode) FillAttr(out *fuse.Attr) fuse.Status {
	// EntryOut.Attr
	meta := self.Meta()
	if meta == nil {
		return fuse.ENOENT
	}
	out.Size = meta.StSize
	out.Blocks = meta.StSize / blockSize
	out.Atime = meta.StAtimeNs
	out.Ctime = meta.StCtimeNs
	out.Mtime = meta.StMtimeNs
	out.Mode = meta.StMode
	out.Rdev = meta.StRdev
	out.Nlink = meta.StNlink
	// TBD rdev?
	// EntryOut.Attr.Owner
	out.Uid = meta.StUid
	out.Gid = meta.StGid
	return fuse.OK
}

func (self *Inode) FillAttrOut(out *fuse.AttrOut) fuse.Status {
	out.AttrValid = attrValidity
	out.AttrValidNsec = 0
	if out.Nlink == 0 {
		out.Nlink = 1
		// original hanwen's work does this, is this really
		// necessary? (allegedly new kernels have issues with
		// nlink=0 + link)
	}
	return self.FillAttr(&out.Attr)
}

func (self *Inode) FillEntryOut(out *fuse.EntryOut) fuse.Status {
	// EntryOut
	out.Ino = self.ino
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

func (self *Inode) GetChildByName(name string) *Inode {
	mlog.Printf2("fs/inode", "GetChildByName %s", name)
	k := NewBlockKeyDirFilename(self.ino, name)
	tr := self.Fs().GetTransaction()
	v := tr.Get(ibtree.IBKey(k))
	if v == nil {
		mlog.Printf2("fs/inode", " not in tree")
		return nil
	}
	ino := binary.BigEndian.Uint64([]byte(*v))
	mlog.Printf2("fs/inode", " inode %v", ino)
	return self.tracker.GetInode(ino)
}

func (self *Inode) GetFile(flags uint32) *InodeFH {
	file := &InodeFH{inode: self, flags: flags}
	self.tracker.AddFile(file)
	self.Refer()
	return file
}

func (self *Inode) GetXAttr(attr string) (data []byte, code fuse.Status) {
	k := NewBlockKey(self.ino, BST_XATTR, attr)
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

func (self *Inode) IterateSubTypeKeys(bst BlockSubType,
	keycb func(key BlockKey) bool) {
	tr := self.Fs().GetTransaction()
	k := NewBlockKey(self.ino, bst, "")
	for {
		nkeyp := tr.NextKey(ibtree.IBKey(k))
		if nkeyp == nil {
			return
		}
		nkey := BlockKey(*nkeyp)
		if nkey.Ino() != self.ino || nkey.SubType() != bst {
			return
		}
		if !keycb(nkey) {
			return
		}
		k = nkey
	}

}

func (self *Inode) RemoveXAttr(attr string) (code fuse.Status) {
	k := ibtree.IBKey(NewBlockKey(self.ino, BST_XATTR, attr))
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

func (self *Inode) SetXAttr(attr string, data []byte) (code fuse.Status) {
	k := NewBlockKey(self.ino, BST_XATTR, attr)
	tr := self.Fs().GetTransaction()
	tr.Set(ibtree.IBKey(k), string(data))
	self.Fs().CommitTransaction(tr)
	return fuse.OK
}

func (self *Inode) SetTimes(atime *time.Time, mtime *time.Time) fuse.Status {
	meta := self.Meta()
	if meta == nil {
		return fuse.ENOENT
	}
	if atime != nil {
		meta.StAtimeNs = uint64(atime.UnixNano())
	}
	if mtime != nil {
		meta.StMtimeNs = uint64(mtime.UnixNano())
	}
	return fuse.OK
}

func (self *Inode) UpdateAtime() {
	now := time.Now()
	self.SetTimes(&now, nil)
}

func (self *Inode) UpdateMtime() {
	now := time.Now()
	self.SetTimes(&now, &now)
}

func (self *Inode) IsDir() bool {
	meta := self.Meta()
	return meta != nil && (meta.StMode&fuse.S_IFDIR) != 0
}

func (self *Inode) IsFile() bool {
	meta := self.Meta()
	return meta != nil && (meta.StMode&fuse.S_IFREG) != 0
}

func (self *Inode) IsLink() bool {
	meta := self.Meta()
	return meta != nil && (meta.StMode&fuse.S_IFLNK) != 0
}

func (self *Inode) Refer() {
	self.refcnt++
}

func (self *Inode) Forget(refcnt uint64) {
	self.refcnt -= refcnt
	if self.refcnt == 0 {
		// TBD if there's something else that should be done?
		delete(self.tracker.ino2inode, self.ino)
	}
}

func (self *Inode) Release() {
	if self == nil {
		return
	}
	self.Forget(1)
}

func (self *Inode) RemoveChildByName(name string) {
	mlog.Printf2("fs/inode", "Inode.RemoveChildByName %v", name)
	child := self.GetChildByName(name)
	defer child.Release()
	if child == nil {
		mlog.Printf2("fs/inode", " not found")
		return
	}
	tr := self.Fs().GetTransaction()
	k := NewBlockKeyDirFilename(self.ino, name)
	rk := NewBlockKeyReverseDirFilename(child.ino, self.ino, name)
	tr.Delete(ibtree.IBKey(k))
	tr.Delete(ibtree.IBKey(rk))
	meta := child.Meta()
	meta.StNlink--
	child.SetMeta(meta)

	meta = self.Meta()
	meta.Nchildren--
	self.SetMeta(meta)

	mlog.Printf2("fs/inode", " Removed %v", child)
	self.Fs().CommitTransaction(tr)
}

// Meta caches the current metadata for particular inode.
// It is valid for the duration of the inode, within validity period anyway.
func (self *Inode) Meta() *InodeMeta {
	if self.meta == nil {
		mlog.Printf2("fs/inode", "Inode.Meta #%d", self.ino)
		k := NewBlockKey(self.ino, BST_META, "")
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

func (self *Inode) SetMeta(meta *InodeMeta) {
	k := NewBlockKey(self.ino, BST_META, "")
	tr := self.Fs().GetTransaction()
	b, err := meta.MarshalMsg(nil)
	if err != nil {
		log.Panic(err)
	}
	tr.Set(ibtree.IBKey(k), string(b))
	self.Fs().CommitTransaction(tr)
	mlog.Printf2("fs/inode", "Inode.SetMeta #%d = %v", self.ino, meta)
	self.meta = meta
}

func (self *Inode) SetSize(size uint64) {
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
		nextKey := NewBlockKeyOffset(self.ino, size+dataExtentSize)
		mlog.Printf2("fs/inode", "SetSize shrinking inode %v - %x+ gone", self.ino, nextKey)
		lastKey := NewBlockKeyOffset(self.ino, 1<<62)
		tr.DeleteRange(ibtree.IBKey(nextKey), ibtree.IBKey(lastKey))
		self.Fs().CommitTransaction(tr)
	}

}

type InodeNumberGenerator interface {
	CreateInodeNumber() uint64
}

type RandomInodeNumberGenerator struct {
}

func (self *RandomInodeNumberGenerator) CreateInodeNumber() uint64 {
	return rand.Uint64()
}

type InodeTracker struct {
	generator InodeNumberGenerator
	ino2inode map[uint64]*Inode
	fh2ifile  map[uint64]*InodeFH
	fs        *Fs
	nextFh    uint64
}

func (self *InodeTracker) Init(fs *Fs) {
	self.ino2inode = make(map[uint64]*Inode)
	self.fh2ifile = make(map[uint64]*InodeFH)
	self.fs = fs
	self.nextFh = 1
	if self.generator == nil {
		self.generator = &RandomInodeNumberGenerator{}
	}
}

func (self *InodeTracker) AddFile(file *InodeFH) {
	self.nextFh++
	fh := self.nextFh
	file.fh = fh
	self.fh2ifile[fh] = file
}

func (self *InodeTracker) getInode(ino uint64) *Inode {
	n := self.ino2inode[ino]
	if n == nil {
		n = &Inode{ino: ino, tracker: self}
		self.ino2inode[ino] = n
	}
	n.refcnt++
	return n
}

func (self *InodeTracker) GetInode(ino uint64) *Inode {
	mlog.Printf2("fs/inode", "GetInode %v", ino)
	inode := self.getInode(ino)
	if inode.Meta() == nil {
		mlog.Printf2("fs/inode", " no meta")
		inode.Release()
		return nil
	}
	mlog.Printf2("fs/inode", " valid")
	return inode
}

func (self *InodeTracker) GetFile(fh uint64) *InodeFH {
	return self.fh2ifile[fh]
}

func (self *InodeTracker) CreateInode() *Inode {
	mlog.Printf2("fs/inode", "CreateInode")
	for {
		ino := self.generator.CreateInodeNumber()
		mlog.Printf2("fs/inode", " %v", ino)
		if self.ino2inode[ino] != nil {
			continue
		}

		// Potentially interesting. See if it is on disk.
		inode := self.getInode(ino)
		if inode.Meta() != nil {
			inode.Release()
			continue
		}

		// We have fresh inode for ourselves!
		return inode
	}
}

// Misc utility stuff

func (self *InodeMeta) SetMkdirIn(input *fuse.MkdirIn) {
	self.StUid = input.Uid
	self.StGid = input.Gid
	self.StMode = input.Mode | fuse.S_IFDIR
	// TBD: Umask?

}

func (self *InodeMeta) SetCreateIn(input *fuse.CreateIn) {
	self.StUid = input.Uid
	self.StGid = input.Gid
	self.StMode = input.Mode | fuse.S_IFREG
	// TBD: Umask?
}

func (self *InodeMeta) SetMknodIn(input *fuse.MknodIn) {
	self.StUid = input.Uid
	self.StGid = input.Gid
	self.StMode = input.Mode
	self.StRdev = input.Rdev
}
