/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2017 Markus Stenberg
 *
 * Created:       Tue Jan  2 10:07:37 2018 mstenber
 * Last modified: Thu Jan 25 12:28:55 2018 mstenber
 * Edit time:     438 min
 *
 */

package fs

import (
	"encoding/binary"
	"fmt"
	"log"

	"github.com/fingon/go-tfhfs/ibtree"
	"github.com/fingon/go-tfhfs/ibtree/hugger"
	"github.com/fingon/go-tfhfs/mlog"
	"github.com/fingon/go-tfhfs/storage"
	"github.com/fingon/go-tfhfs/util"
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
	lastKey *BlockKey

	// statistics for unit tests (these cost some memory but so what)
	readNextInodeBruteForceCount int
}

func (self *inodeFH) String() string {
	return fmt.Sprintf("fh{#%d %v", self.fh, self.inode)
}

func (self *inodeFH) ReadNextinode() (inode *inode, nkey BlockKey) {
	// dentry at lastName (if set) or pos (if not set);
	// return true if reading was successful (and pos got advanced)
	tr := self.Fs().GetTransaction()
	defer tr.Close()
	kp := self.lastKey
	mlog.Printf2("fs/fh", "fh.ReadNextinode %v", kp == nil)
	if kp == nil {
		self.readNextInodeBruteForceCount++
		i := uint64(0)
		self.inode.IterateSubTypeKeys(BST_DIR_NAME2INODE,
			func(key BlockKey) bool {
				mlog.Printf2("fs/fh", " #%d %v", i, key.SubTypeData()[filenameHashSize:])
				if i == self.pos {
					nkey = key
					mlog.Printf2("fs/fh", " found what we looked for")
					return false
				}
				i++
				return true
			})
	} else {
		mlog.Printf2("fs/fh", " calling NextKey %x", *kp)
		nkeyp := tr.IB().NextKey(kp.IB())
		if nkeyp == nil {
			mlog.Printf2("fs/fh", " next missing")
			return
		}
		nkey = BlockKey(*nkeyp)
	}
	if nkey == "" || nkey.Ino() != self.inode.ino || nkey.SubType() != BST_DIR_NAME2INODE {
		mlog.Printf2("fs/fh", " end - %x", nkey)
		return
	}
	inop := tr.IB().Get(nkey.IB())
	ino := binary.BigEndian.Uint64([]byte(*inop))
	inode = self.inode.tracker.GetInode(ino)
	return
}

func (self *inodeFH) ReadDirEntry(l *fuse.DirEntryList) bool {
	mlog.Printf2("fs/fh", "fh.ReadDirEntry fh:%v inode:%v", self.fh, self.inode.ino)
	inode, nkey := self.ReadNextinode()
	if inode == nil {
		mlog.Printf2("fs/fh", " nothing found")
		return false
	}
	defer inode.Release()
	name := nkey.Filename()
	meta := inode.Meta()
	e := fuse.DirEntry{Mode: meta.StMode, Name: name, Ino: inode.ino}
	ok, npos := l.AddDirEntry(e)
	if !ok {
		mlog.Printf2("fs/fh", "AddDirEntry failed")
		return false
	}
	mlog.Printf2("fs/fh", " #%d %s", self.pos, name)
	self.pos = npos
	self.lastKey = &nkey
	return true
}

func (self *inodeFH) ReadDirPlus(input *fuse.ReadIn, l *fuse.DirEntryList) bool {
	inode, nkey := self.ReadNextinode()
	if inode == nil {
		return false
	}
	defer inode.Release()
	name := nkey.Filename()
	meta := inode.Meta()
	e := fuse.DirEntry{Mode: meta.StMode, Name: name, Ino: inode.ino}
	entry, npos := l.AddDirLookupEntry(e)
	if entry == nil {
		mlog.Printf2("fs/fh", "AddDirLookupEntry failed")
		return false
	}
	inode.FillEntryOut(entry)

	// Move on with things
	self.pos = npos
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

func (self *inodeFH) read(buf []byte, offset uint64) (r int, code fuse.Status) {
	mlog.Printf2("fs/fh", "fh.read %v @%v", len(buf), offset)
	end := offset + uint64(len(buf))
	meta := self.inode.Meta()
	size := meta.StSize
	if end > size {
		end = size
		mlog.Printf2("fs/fh", " hit EOF -> end at %v", end)
	}

	var b []byte
	if size <= EmbeddedSize {
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
		defer tr.Close()
		bidp := tr.IB().Get(k.IB())
		if bidp == nil {
			mlog.Printf2("fs/fh", "Key %x not found at all", k)
		} else {
			bl := self.Fs().storage.GetBlockById(*bidp)
			if bl == nil {
				mlog.Panicf("Block %x not found at all", *bidp)
			}
			defer bl.Close()
			b = bl.Data()
			if b[0] != byte(BDT_EXTENT) {
				log.Panicf("Wrong extent type in read - block content: %x", b)
			}
			b = b[1:]
		}
	}

	var read int

	if len(b) >= int(offset) {
		copy(buf, b[offset:])
		read = len(b[offset:])
	}

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
		r = 0
	} else {
		mlog.Printf2("fs/fh", " read %v ([%v:%v])", read+zeros, offset, end)
		r = read + zeros
	}
	return
}

func (self *inodeFH) Read(buf []byte, offset uint64) (rr fuse.ReadResult, code fuse.Status) {
	// ofs == offset in buf, offset == offset in file
	ofs := 0
	for ofs < len(buf) {
		e := offset / dataExtentSize
		defer self.inode.offsetMap.Locked(e)()

		r, code := self.read(buf[ofs:], offset)
		if !code.Ok() {
			return nil, code
		}
		if r == 0 {
			break
		}
		offset += uint64(r)
		ofs += r
		ne := offset / dataExtentSize
		// we should hit this only once per extent or bad
		// things happen with the offsetMap lock (only really
		// occurs in EOF anyway so it is ok)
		if e == ne {
			break
		}
	}
	rr = fuse.ReadResultData(buf[:ofs])
	return

}

func (self *inodeFH) writeInTransaction(meta *InodeMeta, tr *hugger.Transaction, buf, odata, obuf, wbuf []byte, bofs int, offset, end uint64) {
	if bofs > 0 {
		if odata != nil {
			if len(odata) > bofs {
				odata = odata[:bofs]
			}
			copy(wbuf, odata)
		} else {
			r, code := self.read(wbuf[:bofs], offset)
			if !code.Ok() {
				return
			}
			mlog.Printf2("fs/fh", " read %v bytes to start (wanted %v)", r, bofs)
		}
		wbuf = wbuf[bofs:]
	}

	// This was copied in earlier
	wbuf = wbuf[len(buf):]

	// Now obuf contains header(< bofs) + buf

	// Read leftovers, if any, from the block
	blockend := offset + dataExtentSize
	if blockend > meta.StSize {
		blockend = meta.StSize
	}
	if blockend > end {
		extra := blockend - end
		r, code := self.read(wbuf[:extra], end)
		if !code.Ok() {
			return
		}
		wbuf = wbuf[r:]
	}

	// bbuf is actually what we want to store
	mlog.Printf2("fs/fh", " obuf %v wbuf %v", len(obuf), len(wbuf))
	bbuf := obuf[:len(obuf)-len(wbuf)]
	mlog.Printf2("fs/fh", " bbuf %v", len(bbuf))

	if meta.StSize <= EmbeddedSize && end <= EmbeddedSize {
		// in .Data this will live long -> make new copy of
		// the (small) slice
		nbuf := bbuf[1:]
		meta.Data = nbuf
	} else {
		k := NewBlockKeyOffset(self.inode.ino, offset)
		bl := tr.GetStorageBlock(storage.BS_NORMAL, bbuf, nil, &util.StringList{})
		bid := bl.Id()
		self.Fs().SetCachedNodeData(ibtree.BlockId(bid), nil)
		mlog.Printf2("fs/fh", " %x = %d bytes, bid %x", k, len(bbuf), bid)
		// mlog.Printf2("fs/fh", " %x", buf)
		tr.IB().Set(k.IB(), bid)
	}

}
func (self *inodeFH) Write(buf []byte, offset uint64) (written uint32, code fuse.Status) {
	wwritten := len(buf)
	for int(written) < wwritten {
		w, code := self.write(buf[written:], offset+uint64(written))
		if !code.Ok() {
			return 0, code
		}
		written += w
	}
	return
}

func (self *inodeFH) write(buf []byte, offset uint64) (written uint32, code fuse.Status) {
	unlockmeta := self.inode.metaWriteLock.Locked()
	e := offset / dataExtentSize
	unlock := self.inode.offsetMap.Locked(e)
	locked := self.inode.offsetMap.GetLockedByName(e)

	mlog.Printf2("fs/fh", "%v.Write %v @%v", self, len(buf), offset)

	tr := self.Fs().GetTransaction()

	done := false

	end := offset + uint64(len(buf))

	bofs := int(offset % dataExtentSize)
	offset -= uint64(bofs)

	// Already inside metaWriteLock
	meta := self.inode.Meta()
	if meta == nil {
		unlock()
		unlockmeta()
		tr.Close()
		return
	}

	done = false
	need := dataExtentSize + dataHeaderMaximumSize
	var odata []byte
	if meta.StSize <= EmbeddedSize && e == 0 {
		odata = meta.Data
		if end <= EmbeddedSize {
			need = EmbeddedSize + 1
		}
	}

	// obuf is the master slice to which we gather data, using
	// wbuf slice which moves gradually onward
	obuf := make([]byte, need)
	obuf[0] = byte(BDT_EXTENT)

	// wbuf is where we're writing in obuf
	wbuf := obuf[1:]

	// Bytes to write
	w := len(buf)
	if w > (dataExtentSize - bofs) {
		w = dataExtentSize - bofs
	}
	if len(buf) > w {
		buf = buf[:w]
	}

	copy(wbuf[bofs:], buf)

	if end > meta.StSize {
		self.inode.SetMetaSizeInTransaction(meta, end, tr)
	}
	written = uint32(w)

	mlog.Printf2("fs/fh", " wrote %v", written)
	if meta.StSize <= EmbeddedSize && end <= EmbeddedSize {
		self.writeInTransaction(meta, tr, buf, odata, obuf, wbuf, bofs, offset, end)
		done = true
	}

	// We're done; the rest is just persisting things to disk which we pretend is instant (cough).
	meta.setTimesNow(true, true, true)
	self.inode.SetMetaInTransaction(meta, tr)

	self.inode.metaWriteLock.ClearOwner()
	locked.ClearOwner()

	self.Fs().writeLimiter.Go(func() {
		mlog.Printf2("fs/fh", "%v.Write-2", self)
		self.inode.metaWriteLock.UpdateOwner()
		locked.UpdateOwner()
		defer unlock()
		defer tr.Close()

		// If file data is part of meta, we have to commit it
		// before metadata is unlocked; if not, last write
		// will implicitly use shared metadata _anyway_ if
		// there is conflicting one (and conflict resolution
		// will pick the later one).
		if done {
			tr.CommitUntilSucceeds()
			unlockmeta()
			return
		} else {
			unlockmeta()
			tr.CommitUntilSucceeds()
		}

		// It wasn't small file. Perform write inside transaction, but
		// do the read + write part ONLY once. The lock we're holding
		// should ensure nobody else touches this part of the file in
		// the meanwhile.
		// We inherit the block-lock, and release only when we're done

		tr := self.Fs().GetTransaction()
		self.writeInTransaction(meta, tr, buf, odata, obuf, wbuf, bofs, offset, end)
		tr.CommitUntilSucceeds()
		mlog.Printf2("fs/fh", " updated data block %v", e)
	})

	return
}
