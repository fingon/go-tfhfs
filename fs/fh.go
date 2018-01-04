/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2017 Markus Stenberg
 *
 * Created:       Tue Jan  2 10:07:37 2018 mstenber
 * Last modified: Thu Jan  4 14:15:31 2018 mstenber
 * Edit time:     178 min
 *
 */

package fs

import (
	"encoding/binary"
	"log"

	"github.com/fingon/go-tfhfs/ibtree"
	"github.com/fingon/go-tfhfs/mlog"
	"github.com/hanwen/go-fuse/fuse"
)

// inodeFH represents a single open instance of a file/directory.
type inodeFH struct {
	inode *inode
	fh    uint64
	flags uint32

	// last position in directory
	pos uint64

	// last key in directory at pos (if any)
	lastKey *blockKey
}

func (self *inodeFH) ReadNextinode() (inode *inode, name string) {
	// dentry at lastName (if set) or pos (if not set);
	// return true if reading was successful (and pos got advanced)
	tr := self.Fs().GetTransaction()
	kp := self.lastKey
	mlog.Printf2("fs/fh", "fh.ReadNextinode %v", kp == nil)
	if kp == nil {
		i := uint64(0)
		self.inode.IterateSubTypeKeys(BST_DIR_NAME2INODE,
			func(key blockKey) bool {
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
		mlog.Printf2("fs/fh", " calling NextKey %x", *kp)
		nkeyp := tr.NextKey(ibtree.IBKey(*kp))
		if nkeyp == nil {
			mlog.Printf2("fs/fh", " next missing")
			return nil, ""
		}
		nkey := blockKey(*nkeyp)
		kp = &nkey
	}
	if kp == nil {
		mlog.Printf2("fs/fh", " empty")
		return nil, ""
	}
	if kp.Ino() != self.inode.ino || kp.SubType() != BST_DIR_NAME2INODE {
		mlog.Printf2("fs/fh", " end - %x", *kp)
		return nil, ""
	}
	inop := tr.Get(ibtree.IBKey(*kp))
	ino := binary.BigEndian.Uint64([]byte(*inop))
	name = string(kp.SubTypeData()[filenameHashSize:])
	mlog.Printf2("fs/fh", " got %v %s", ino, name)
	inode = self.inode.tracker.GetInode(ino)
	return
}

func (self *inodeFH) ReadDirEntry(l *fuse.DirEntryList) bool {
	mlog.Printf2("fs/fh", "fh.ReadDirEntry fh:%v inode:%v", self.fh, self.inode.ino)
	inode, name := self.ReadNextinode()
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
		nkey := NewblockKeyDirFilename(self.inode.ino, name)
		mlog.Printf2("fs/fh", " #%d %x", self.pos, nkey)
		self.pos++
		self.lastKey = &nkey
	} else {
		mlog.Printf2("fs/fh", "AddDirEntry failed")
	}
	return ok
}

func (self *inodeFH) ReadDirPlus(input *fuse.ReadIn, l *fuse.DirEntryList) bool {
	inode, name := self.ReadNextinode()
	defer inode.Release()
	if inode == nil {
		return false
	}
	defer inode.Release()
	meta := inode.Meta()
	e := fuse.DirEntry{Mode: meta.StMode, Name: name, Ino: inode.ino}
	entry, _ := l.AddDirLookupEntry(e)
	if entry == nil {
		mlog.Printf2("fs/fh", "AddDirLookupEntry failed")
		return false
	}
	*entry = fuse.EntryOut{}
	self.Ops().Lookup(&input.InHeader, name, entry)

	// Move on with things
	self.pos++
	nkey := NewblockKeyDirFilename(self.inode.ino, name)
	self.lastKey = &nkey
	return true
}

func (self *inodeFH) Fs() *Fs {
	return self.inode.Fs()
}

func (self *inodeFH) Ops() *fsOps {
	return self.inode.Ops()
}

func (self *inodeFH) Release() {
	delete(self.inode.tracker.fh2ifile, self.fh)
	self.inode.Release()
}

func (self *inodeFH) SetPos(pos uint64) {
	if self.pos == pos {
		mlog.Printf2("fs/fh", "fh.SetPos still at %d", pos)
		return
	}
	mlog.Printf2("fs/fh", "inodeFH.SetPos %d", pos)
	self.pos = pos
	// TBD - does this need something else too?
	self.lastKey = nil
}

func (self *inodeFH) Read(buf []byte, offset uint64) (rr fuse.ReadResult, code fuse.Status) {
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
		k := NewblockKeyOffset(self.inode.ino, offset)
		e := offset / dataExtentSize
		offset -= e * dataExtentSize
		end -= e * dataExtentSize
		if end > dataExtentSize {
			end = dataExtentSize
		}
		tr := self.Fs().GetTransaction()
		bidp := tr.Get(ibtree.IBKey(k))
		if bidp == nil {
			mlog.Printf2("fs/fh", "Key %x not found at all", k)
		} else {
			bl := self.Fs().storage.GetBlockById(*bidp)
			if bl == nil {
				log.Panicf("Block %x not found at all", *bidp)
			}
			b = bl.GetData()
			if b[0] != byte(BDT_EXTENT) {
				log.Panicf("Wrong extent type in read - block content: %x", b)
			}
			b = b[1:]
		}
	}

	copy(buf, b[offset:])
	read := len(b[offset:])

	// offset / end are now relative to current extent, which is
	// in b.

	zeros := int(end) - read - int(offset)
	if zeros > 0 {
		// implicitly pad it with zeros
		mlog.Printf2("fs/fh", " padding result with %d zeros", zeros)
		for i := 0; i < zeros; i++ {
			buf[read+i] = 0
		}
	}

	if end <= offset {
		mlog.Printf2("fs/fh", " nothing to read")
		rr = fuse.ReadResultData(buf[0:0])
	} else {
		mlog.Printf2("fs/fh", " read %v ([%v:%v])", read+zeros, offset, end)
		rr = fuse.ReadResultData(buf[0 : read+zeros])
	}
	return
}

func (self *inodeFH) Write(buf []byte, offset uint64) (written uint32, code fuse.Status) {
	mlog.Printf2("fs/fh", "fh.Write %v @%v", len(buf), offset)
	var r fuse.ReadResult

	end := offset + uint64(len(buf))
	need := dataExtentSize + dataHeaderMaximumSize
	meta := self.inode.Meta()
	if end <= embeddedSize && meta.StSize <= embeddedSize {
		need = embeddedSize + 1
	}

	// obuf is the master slice to which we gather data, using
	// wbuf slice which moves gradually onward
	obuf := make([]byte, need)
	obuf[0] = byte(BDT_EXTENT)

	wbuf := obuf[1:]

	// Grab start of block, if any
	bofs := int(offset % dataExtentSize)
	offset -= uint64(bofs)
	if bofs > 0 {
		r, code = self.Read(wbuf[:bofs], offset)
		if !code.Ok() {
			return
		}
		tbuf, _ := r.Bytes(nil)
		wbuf = wbuf[len(tbuf):]
		mlog.Printf2("fs/fh", " read %v bytes to start (wanted %v)", r.Size(), bofs)
	}

	// Bytes to write
	w := len(buf)
	if w > (dataExtentSize - bofs) {
		w = dataExtentSize - bofs
	}
	if len(buf) > w {
		buf = buf[:w]
	}

	copy(wbuf, buf)
	wbuf = wbuf[len(buf):]

	// Now obuf contains header(< bofs) + buf

	// Read leftovers, if any, from the block
	blockend := offset + dataExtentSize
	if blockend > end && end < meta.StSize {
		extra := blockend - end
		r, code = self.Read(wbuf, extra)
		if !code.Ok() {
			return
		}
		tbuf, _ := r.Bytes(nil)
		wbuf = wbuf[len(tbuf):]
	}

	// bbuf is actually what we want to store
	mlog.Printf2("fs/fh", " obuf %v wbuf %v", len(obuf), len(wbuf))
	bbuf := obuf[:len(obuf)-len(wbuf)]
	mlog.Printf2("fs/fh", " bbuf %v", len(bbuf))

	if meta.StSize <= embeddedSize && end <= embeddedSize {
		// in .Data this will live long -> make new copy of
		// the (small) slice
		nbuf := bbuf[1:]
		meta.Data = nbuf
		meta.StSize = uint64(len(nbuf))
		self.inode.SetMeta(meta)
		mlog.Printf2("fs/fh", " meta %d bytes", len(nbuf))
	} else {
		if len(meta.Data) > 0 {
			meta.Data = nil
			self.inode.SetMeta(meta)
		}
		k := NewblockKeyOffset(self.inode.ino, offset)
		tr := self.Fs().GetTransaction()
		bid := self.Fs().getBlockDataId(bbuf, nil)
		mlog.Printf2("fs/fh", " %x = %d bytes, bid %x", k, len(bbuf), bid)
		// mlog.Printf2("fs/fh", " %x", buf)
		tr.Set(ibtree.IBKey(k), string(bid))
		self.Fs().CommitTransaction(tr)
		self.inode.SetSize(end)
	}

	written = uint32(w)
	mlog.Printf2("fs/fh", " wrote %v", written)
	return
}
