/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2017 Markus Stenberg
 *
 * Created:       Fri Dec 29 08:21:32 2017 mstenber
 * Last modified: Fri Dec 29 09:24:18 2017 mstenber
 * Edit time:     30 min
 *
 */

package fs

import (
	"encoding/binary"
	"log"

	"github.com/fingon/go-tfhfs/ibtree"
	"github.com/hanwen/go-fuse/fuse"
)

type Inode struct {
	ino     uint64
	tracker *InodeTracker
	refcnt  uint64
}

func (self *Inode) Fs() *Fs {
	return self.tracker.fs
}

func (self *Inode) FillEntry(out *fuse.EntryOut) {
	// EntryOut
	out.Ino = self.ino
	out.NodeId = self.ino
	out.Generation = 0
	out.EntryValid = 5
	out.AttrValid = 5
	out.EntryValidNsec = 0
	out.AttrValidNsec = 0
	// EntryOut.Attr
	meta := self.Meta()
	out.Size = meta.StSize
	out.Blocks = meta.StSize / blockSize
	out.Atime = meta.StAtimeNs
	out.Ctime = meta.StCtimeNs
	out.Mtime = meta.StMtimeNs
	out.Mode = meta.StMode
	out.Nlink = meta.StNlink
	// TBD rdev?
	// EntryOut.Attr.Owner
	out.Uid = meta.StUid
	out.Gid = meta.StGid
}

func (self *Inode) GetChildByName(name string) *Inode {
	k := NewBlockKeyDirFilename(self.ino, name)
	tr := self.Fs().GetTransaction()
	v := tr.Get(ibtree.IBKey(k))
	if v == nil {
		return nil
	}
	ino := binary.BigEndian.Uint64([]byte(*v))
	return self.tracker.GOCInode(ino)
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
	self.Forget(1)
}

func (self *Inode) Meta() *InodeMeta {
	k := NewBlockKey(self.ino, BST_META, "")
	tr := self.Fs().GetTransaction()
	v := tr.Get(ibtree.IBKey(k))
	if v == nil {
		return nil
	}
	var m InodeMeta
	_, err := m.UnmarshalMsg([]byte(*v))
	if err != nil {
		log.Panic(err)
	}
	return &m
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
}

type InodeTracker struct {
	ino2inode map[uint64]*Inode
	fs        *Fs
}

func (self *InodeTracker) getInode(ino uint64, create bool) *Inode {
	n := self.ino2inode[ino]
	if n == nil {
		if create {
			n = &Inode{ino: ino, tracker: self}
			self.ino2inode[ino] = n
		}
	}
	if n != nil {
		n.refcnt++
	}
	return n
}

func (self *InodeTracker) GetInode(ino uint64) *Inode {
	return self.getInode(ino, false)
}

func (self *InodeTracker) GOCInode(ino uint64) *Inode {
	return self.getInode(ino, true)
}
