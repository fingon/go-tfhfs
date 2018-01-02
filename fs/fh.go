/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2017 Markus Stenberg
 *
 * Created:       Tue Jan  2 10:07:37 2018 mstenber
 * Last modified: Tue Jan  2 13:59:10 2018 mstenber
 * Edit time:     52 min
 *
 */

package fs

import (
	"bytes"
	"encoding/binary"

	"github.com/fingon/go-tfhfs/ibtree"
	"github.com/fingon/go-tfhfs/mlog"
	"github.com/fingon/go-tfhfs/util"
	"github.com/hanwen/go-fuse/fuse"
)

// InodeFH represents a single open instance of a file/directory.
type InodeFH struct {
	inode *Inode
	fh    uint64
	flags uint32

	// last position in directory
	pos uint64

	// last key in directory at pos (if any)
	lastKey *BlockKey
}

func (self *InodeFH) ReadNextInode() (inode *Inode, name string) {
	// dentry at lastName (if set) or pos (if not set);
	// return true if reading was successful (and pos got advanced)
	tr := self.Fs().GetTransaction()
	kp := self.lastKey
	mlog.Printf2("fs/fh", "fh.ReadNextInode %v", kp == nil)
	if kp == nil {
		i := uint64(0)
		self.inode.IterateSubTypeKeys(BST_DIR_NAME2INODE,
			func(key BlockKey) bool {
				mlog.Printf2("fs/fh", " #%d %v", i, key.SubTypeData()[filenameHashSize:])
				if i == self.pos {
					kp = &key
					mlog.Printf2("fs/fh", " found what we looked for")
					return false
				}
				i++
				return true
			})
	} else {
		nkeyp := tr.NextKey(ibtree.IBKey(*kp))
		if nkeyp == nil {
			mlog.Printf2("fs/fh", " next missing")
			return nil, ""
		}
		nkey := BlockKey(*nkeyp)
		kp = &nkey
	}
	if kp == nil {
		mlog.Printf2("fs/fh", " empty")
		return nil, ""
	}
	if kp.Ino() != self.inode.ino || kp.SubType() != BST_DIR_NAME2INODE {
		return nil, ""
	}
	inop := tr.Get(ibtree.IBKey(*kp))
	ino := binary.BigEndian.Uint64([]byte(*inop))
	name = string(kp.SubTypeData()[filenameHashSize:])
	inode = self.inode.tracker.GetInode(ino)
	return
}

func (self *InodeFH) ReadDirEntry(l *fuse.DirEntryList) bool {
	mlog.Printf2("fs/fh", "fh.ReadDirEntry")
	inode, name := self.ReadNextInode()
	defer inode.Release()
	if inode == nil {
		mlog.Printf2("fs/fh", " nothing found")
		return false
	}
	defer inode.Release()
	meta := inode.Meta()
	e := fuse.DirEntry{Mode: meta.StMode, Name: name, Ino: inode.ino}
	ok, _ := l.AddDirEntry(e)
	if ok {
		nkey := NewBlockKeyDirFilename(inode.ino, name)
		// mlog.Printf2("fs/fh", " #%d %s", self.pos, nkey)
		// ^ key is probably byte array!
		self.pos++
		self.lastKey = &nkey
	}
	return ok
}

func (self *InodeFH) ReadDirPlus(input *fuse.ReadIn, l *fuse.DirEntryList) bool {
	inode, name := self.ReadNextInode()
	defer inode.Release()
	if inode == nil {
		return false
	}
	defer inode.Release()
	meta := inode.Meta()
	e := fuse.DirEntry{Mode: meta.StMode, Name: name, Ino: inode.ino}
	entry, _ := l.AddDirLookupEntry(e)
	if entry == nil {
		return false
	}
	*entry = fuse.EntryOut{}
	self.Fs().Lookup(&input.InHeader, name, entry)

	// Move on with things
	self.pos++
	nkey := NewBlockKeyDirFilename(inode.ino, name)
	self.lastKey = &nkey
	return true
}

func (self *InodeFH) Fs() *Fs {
	return self.inode.Fs()
}

func (self *InodeFH) Release() {
	delete(self.inode.tracker.fh2ifile, self.fh)
	self.inode.Release()
}

func (self *InodeFH) SetPos(pos uint64) {
	mlog.Printf2("fs/fh", "InodeFH.SetPos %d", pos)
	if self.pos == pos {
		return
	}
	self.pos = pos
	// TBD - does this need something else too?
	self.lastKey = nil
}

func (self *InodeFH) Read(buf []byte, offset uint64) (rr fuse.ReadResult, code fuse.Status) {
	mlog.Printf2("fs/fh", "fh.Read %v @%v", len(buf), offset)
	end := offset + uint64(len(buf))
	meta := self.inode.Meta()
	size := meta.StSize
	if end > size {
		end = size
		mlog.Printf2("fs/fh", " hit EOF -> end at %v", end)
	}

	var b []byte
	if size <= embeddedSize {
		b = meta.Data
	} else {
		k := NewBlockKeyOffset(self.inode.ino, offset)
		e := offset / dataExtentSize
		offset -= e * dataExtentSize
		end -= e * dataExtentSize
		if end > dataExtentSize {
			end = dataExtentSize
		}
		tr := self.Fs().GetTransaction()
		vp := tr.Get(ibtree.IBKey(k))
		if vp != nil {
			b = []byte(*vp)
		}
	}

	// offset / end are now relative to current extent, which is
	// in b.

	zeros := int(end) - len(b)
	if zeros > 0 {
		// implicitly pad it with zeros
		mlog.Printf2("fs/fh", " padding result with %d zeros", zeros)
		b = util.ConcatBytes(b, bytes.Repeat([]byte{0}, zeros))
	}

	if end <= offset {
		mlog.Printf2("fs/fh", " nothing to read")
		rr = fuse.ReadResultData([]byte{})
	} else {
		read := end - offset
		mlog.Printf2("fs/fh", " read %v ([%v:%v])", read, offset, end)
		rr = fuse.ReadResultData(b[offset:end])
	}
	return
}

func (self *InodeFH) Write(buf []byte, offset uint64) (written uint32, code fuse.Status) {
	mlog.Printf2("fs/fh", "fh.Write %v @%v", len(buf), offset)
	var r fuse.ReadResult

	// Grab start of block, if any
	bofs := offset % dataExtentSize
	if bofs > 0 {
		tbuf := make([]byte, bofs)
		r, code = self.Read(tbuf, offset-bofs)
		if !code.Ok() {
			return
		}
		tbuf, code = r.Bytes(nil)
		if !code.Ok() {
			return
		}
		buf = util.ConcatBytes(tbuf, buf)
		offset -= bofs
	}

	if len(buf) > dataExtentSize {
		buf = buf[:dataExtentSize]
	}

	end := offset + uint64(len(buf))

	// Read leftovers, if any, from the block
	blockend := offset - bofs + dataExtentSize
	extraend := uint64(0)

	if blockend > end {
		extra := blockend - end
		tbuf := make([]byte, extra)
		r, code = self.Read(tbuf, end)
		if !code.Ok() {
			return
		}
		tbuf, code = r.Bytes(nil)
		if !code.Ok() {
			return
		}
		mlog.Printf2("fs/fh", " got leftovers %v", len(tbuf))
		extraend = uint64(len(tbuf))
		buf = util.ConcatBytes(buf, tbuf)
	}

	// Special case: If file is small AND we're writing only at
	// most to first small bytes, do it there
	meta := self.inode.Meta()
	if meta.StSize <= embeddedSize && end <= embeddedSize {
		meta.Data = buf
		meta.StSize = uint64(len(buf))
		self.inode.SetMeta(meta)
	} else {
		if len(meta.Data) > 0 {
			meta.Data = []byte{}
			self.inode.SetMeta(meta)
		}
		k := NewBlockKeyOffset(self.inode.ino, offset)
		tr := self.Fs().GetTransaction()
		tr.Set(ibtree.IBKey(k), string(buf))
		self.Fs().CommitTransaction(tr)
		self.inode.SetSize(end)
	}

	blen := uint64(len(buf))
	written = uint32(blen - bofs - extraend)
	mlog.Printf2("fs/fh", " wrote %v", written)
	return
}
