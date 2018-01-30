/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2017 Markus Stenberg
 *
 * Created:       Fri Dec 29 08:21:32 2017 mstenber
 * Last modified: Tue Jan 30 10:07:22 2018 mstenber
 * Edit time:     343 min
 *
 */

package fs

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"log"
	"math/rand"
	"sync/atomic"
	"time"

	"github.com/fingon/go-tfhfs/ibtree"
	"github.com/fingon/go-tfhfs/ibtree/hugger"
	"github.com/fingon/go-tfhfs/mlog"
	"github.com/fingon/go-tfhfs/util"
	"github.com/hanwen/go-fuse/fuse"
)

type inode struct {
	ino       uint64
	tracker   *inodeTracker
	refcnt    int64
	offsetMap util.MutexLockedMap

	meta          InodeMetaAtomicPointer
	metaWriteLock util.MutexLocked
}

func (self *inode) AddChild(name string, child *inode) (code fuse.Status) {
	mlog.Printf2("fs/inode", "inode.AddChild %v = %v", name, child)
	self.Fs().Update2(func(tr *hugger.Transaction) bool {
		meta := child.Meta()
		if meta == nil {
			code = fuse.ENOENT
			return false
		}
		meta.SetCTimeNow()
		meta.StNlink++
		if child.IsDir() {
			meta.ParentIno = self.ino
		}
		child.SetMetaInTransaction(meta, tr)

		meta = self.Meta()
		if meta == nil {
			code = fuse.ENOENT
			return false
		}
		meta.SetMTimeNow()
		meta.Nchildren++
		self.SetMetaInTransaction(meta, tr)

		k := NewBlockKeyDirFilename(self.ino, name).IB()
		rk := NewBlockKeyReverseDirFilename(child.ino, self.ino, name).IB()
		tr.IB().Set(k, string(util.Uint64Bytes(child.ino)))
		tr.IB().Set(rk, "")
		return true
	})
	return
}

func (self *inode) Fs() *Fs {
	return self.tracker.fs
}

func (self *inode) Ops() *fsOps {
	return &self.tracker.fs.Ops
}

func (self *inode) String() string {
	return fmt.Sprintf("inode{%v rc:%v}", self.ino, self.refcnt)
}

func unixNanoToFuse(t uint64, seconds *uint64, parts *uint32) {
	*seconds = t / uint64(time.Second)
	*parts = uint32(t % uint64(time.Second))
}

func (self *inode) addRefCount(refcnt int64) {
	refcnt = atomic.AddInt64(&self.refcnt, refcnt)
	if refcnt > 0 {
		return
	}
	if refcnt == 0 {
		defer self.tracker.inodeLock.Locked()()
		// was taken by someone
		if self.refcnt > 0 {
			return
		}
		// TBD if there's something else that should be done?
		delete(self.tracker.ino2inode, self.ino)

		// TBD Delete from tree if StNlink == 0
		return
	}
	if refcnt < 0 {
		log.Panicf("inode refcount below zero")
	}
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
	// sometimes e.g. Link seems to provide nil out
	// ( TBD is it a bug? feature? )
	if out == nil {
		return fuse.OK
	}
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
	k := NewBlockKeyDirFilename(self.ino, name)
	tr := self.Fs().GetTransaction()
	defer tr.Close()
	v := tr.IB().Get(k.IB())
	if v == nil {
		mlog.Printf2("fs/inode", " child %v not in tree", k)
		// tr.root.node.PrintToMLogAll()
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
	defer self.offsetMap.Locked(-1)()
	k := NewBlockKey(self.ino, BST_XATTR, attr)
	tr := self.Fs().GetTransaction()
	defer tr.Close()
	v := tr.IB().Get(k.IB())
	if v == nil {
		code = fuse.ENOATTR
		return
	}
	data = []byte(*v)
	code = fuse.OK
	return
}

func IterateInoSubTypeKeys(t *ibtree.Transaction, ino uint64, bst BlockSubType, keycb func(key BlockKey) bool) {
	k := NewBlockKey(ino, bst, "")
	for {
		nkeyp := t.NextKey(k.IB())
		if nkeyp == nil {
			return
		}
		nkey := BlockKey(*nkeyp)
		if nkey.Ino() != ino || nkey.SubType() != bst {
			return
		}
		if !keycb(nkey) {
			return
		}
		k = nkey
	}

}

func (self *inode) IterateSubTypeKeys(bst BlockSubType, keycb func(key BlockKey) bool) {
	tr := self.Fs().GetTransaction()
	defer tr.Close()
	IterateInoSubTypeKeys(tr.IB(), self.ino, bst, keycb)
}

func (self *inode) RemoveXAttr(attr string) (code fuse.Status) {
	defer self.offsetMap.Locked(-1)()
	self.Fs().Update(func(tr *hugger.Transaction) {
		k := NewBlockKey(self.ino, BST_XATTR, attr).IB()
		mlog.Printf2("fs/inode", "RemoveXAttr %s - deleting %x", attr, k)
		v := tr.IB().Get(k)
		if v == nil {
			code = fuse.ENOATTR
			return
		}
		tr.IB().Delete(k)
	})
	return
}

func (self *inode) SetXAttr(attr string, data []byte) (code fuse.Status) {
	defer self.offsetMap.Locked(-1)()
	self.Fs().Update(func(tr *hugger.Transaction) {
		k := NewBlockKey(self.ino, BST_XATTR, attr)
		mlog.Printf2("fs/inode", "SetXAttr %s - setting %x", attr, k)
		tr.IB().Set(k.IB(), string(data))
	})
	return fuse.OK
}

func (self *inode) IsDir() bool {
	return self.Meta().IsDir()
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
	self.addRefCount(1)
}

func (self *inode) Release() {
	if self == nil {
		return
	}
	self.addRefCount(-1)
}

func (self *inode) Forget(nlookup uint64) {
	if self == nil {
		return
	}
	self.addRefCount(-int64(nlookup))
}

func (self *inode) RemoveChild(child *inode, name string) (code fuse.Status) {
	mlog.Printf2("fs/inode", "inode.RemoveChildByName %v", name)
	self.Fs().Update2(func(tr *hugger.Transaction) bool {
		meta := child.Meta()
		if meta == nil {
			code = fuse.ENOENT
			return false
		}
		meta.StNlink--
		meta.ParentIno = 0
		meta.SetCTimeNow()
		child.SetMetaInTransaction(meta, tr)

		meta = self.Meta()
		if meta == nil {
			code = fuse.ENOENT
			return false
		}
		meta.Nchildren--
		meta.SetMTimeNow()
		self.SetMetaInTransaction(meta, tr)

		k := NewBlockKeyDirFilename(self.ino, name)
		rk := NewBlockKeyReverseDirFilename(child.ino, self.ino, name)
		tr.IB().Delete(k.IB())
		tr.IB().Delete(rk.IB())
		return true
	})
	mlog.Printf2("fs/inode", " Removed %v", child)
	if self.Fs().server != nil {
		self.Fs().server.DeleteNotify(self.ino, child.ino, name)
	}
	return
}

func decodeInodeMeta(v string) *InodeMeta {
	var m InodeMeta
	_, err := m.UnmarshalMsg([]byte(v))
	if err != nil {
		log.Panic(err)
	}
	mlog.Printf2("fs/inode", " = %v", &m)
	return &m

}

func (self *inode) getMeta() *InodeMeta {
	mlog.Printf2("fs/inode", "inode.Meta #%d", self.ino)
	k := NewBlockKey(self.ino, BST_META, "")
	tr := self.Fs().GetTransaction()
	defer tr.Close()
	v := tr.IB().Get(k.IB())
	if v == nil {
		mlog.Printf2("fs/inode", " not found")
		return nil
	}
	return decodeInodeMeta(*v)
}

// Meta returns a copy of current inode metadata. If the inode in
// question has disappeared from disk, nil may be returned.
func (self *inode) Meta() *InodeMeta {
	m := self.meta.Get()
	if m == nil {
		m = self.getMeta()
		if m == nil {
			return nil
		}
		// We don't have a lock so this is best we can do
		self.meta.SetIfEqualTo(m, nil)
	}
	// We return a copy so mutation in place is safe
	nm := *m
	return &nm
}

func (self *inode) SetMetaInTransaction(meta *InodeMeta, tr *hugger.Transaction) bool {
	self.metaWriteLock.AssertLocked()
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

	k := NewBlockKey(self.ino, BST_META, "")
	b, err := meta.MarshalMsg(nil)
	if err != nil {
		log.Panic(err)
	}
	old := self.meta.Get()
	if old == nil || old.InodeMetaData != meta.InodeMetaData || !bytes.Equal(meta.Data, old.Data) {
		tr.IB().Set(k.IB(), string(b))
		self.meta.Set(meta)
		return true
	}
	return false
}

func (self *inode) SetMetaSizeInTransaction(meta *InodeMeta, size uint64, tr *hugger.Transaction) bool {
	shrink := false
	if size == meta.StSize {
		return false
	} else if size < meta.StSize && meta.StSize > dataExtentSize {
		shrink = true
	}
	meta.StSize = size
	if size > EmbeddedSize {
		mlog.Printf2("fs/inode", "SetSize cleared in-place metadata")
		meta.Data = nil
	}
	if shrink {
		nextKey := NewBlockKeyOffset(self.ino, size+dataExtentSize)
		mlog.Printf2("fs/inode", "SetSize shrinking inode %v - %x+ gone", self.ino, nextKey)
		lastKey := NewBlockKeyOffset(self.ino, 1<<62)
		tr.IB().DeleteRange(nextKey.IB(), lastKey.IB())
	}
	return true
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
	fs        *Fs
	inodeLock util.MutexLocked
	generator inodeNumberGenerator
	ino2inode map[uint64]*inode
	fh2ifile  map[uint64]*inodeFH
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
	defer self.inodeLock.Locked()()
	self.nextFh++
	fh := self.nextFh
	file.fh = fh
	self.fh2ifile[fh] = file
}

func (self *inodeTracker) getInode(ino uint64) *inode {
	self.inodeLock.AssertLocked()
	n := self.ino2inode[ino]
	if n == nil {
		n = &inode{ino: ino, tracker: self}
		self.ino2inode[ino] = n
	}
	atomic.AddInt64(&n.refcnt, 1)
	return n
}

func (self *inodeTracker) GetInode(ino uint64) *inode {
	self.inodeLock.Lock()
	mlog.Printf2("fs/inode", "GetInode %v", ino)
	inode := self.getInode(ino)
	if inode.Meta() == nil {
		mlog.Printf2("fs/inode", " no meta")
		self.inodeLock.Unlock()
		inode.Release()
		return nil
	}
	mlog.Printf2("fs/inode", " valid")
	self.inodeLock.Unlock()
	return inode
}

func (self *inodeTracker) GetFileByFh(fh uint64) *inodeFH {
	defer self.inodeLock.Locked()()
	return self.fh2ifile[fh]
}

func (self *inodeTracker) createInode() *inode {
	self.inodeLock.AssertLocked()
	mlog.Printf2("fs/inode", "createInode")
	for {
		ino := self.generator.CreateInodeNumber()
		mlog.Printf2("fs/inode", " %v", ino)
		if ino == 0 || self.ino2inode[ino] != nil {
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

func (self *inodeTracker) CreateInode() *inode {
	defer self.inodeLock.Locked()()
	return self.createInode()
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
	self.setTimesNow(true, true, true)
}

func (self *InodeMetaData) IsDir() bool {
	return self != nil && (self.StMode&fuse.S_IFDIR) != 0

}
