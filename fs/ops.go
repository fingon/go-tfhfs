/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2017 Markus Stenberg
 *
 * Created:       Thu Dec 28 12:52:43 2017 mstenber
 * Last modified: Sat Dec 30 00:20:16 2017 mstenber
 * Edit time:     150 min
 *
 */

package fs

import (
	"bytes"
	"os"
	"syscall"
	"time"

	. "github.com/hanwen/go-fuse/fuse"
)

func (self *Fs) Init(server *Server) {
	self.server = server
}

func (self *Fs) String() string {
	return os.Args[0]
}

func (self *Fs) SetDebug(dbg bool) {
	// TBD - do we need debug functionality someday?
}

func (self *Fs) StatFs(input *InHeader, out *StatfsOut) Status {
	bsize := blockSize
	out.Bsize = uint32(bsize)
	avail := self.storage.Backend.GetBytesAvailable()
	out.Bfree = uint64(avail / bsize)
	out.Bavail = uint64(avail / bsize)
	used := self.storage.Backend.GetBytesUsed()
	out.Blocks = uint64(used / bsize)
	return OK
}

func (self *Fs) access(inode *Inode, mode uint32, orOwn bool, ctx *Context) Status {
	if inode == nil {
		return ENOENT
	}
	meta := inode.Meta()
	if meta == nil {
		return ENOENT
	}
	if ctx.Uid == 0 {
		return OK
	}
	perms := meta.StMode & 0x7
	if ctx.Uid == meta.StUid {
		if orOwn {
			return OK
		}
		perms |= (meta.StMode >> 6) & 0x7
	}
	if ctx.Gid == meta.StGid {
		perms |= (meta.StMode >> 3) & 0x7
	}
	if (perms & mode) == mode {
		return OK
	}
	return EPERM
}

// lookup gets child of a parent.
func (self *Fs) lookup(parent *Inode, name string, ctx *Context) (child *Inode, code Status) {
	code = self.access(parent, X_OK, false, ctx)
	if !code.Ok() {
		return
	}
	if !parent.IsDir() {
		code = ENOTDIR
		return
	}
	code = OK
	if name == "." {
		child = parent
	} else {
		child = parent.GetChildByName(name)
		if child == nil {
			code = ENOENT
		}
	}
	return
}

func (self *Fs) Lookup(input *InHeader, name string, out *EntryOut) (code Status) {
	parent := self.GetInode(input.NodeId)
	defer parent.Release()

	child, code := self.lookup(parent, name, &input.Context)
	defer child.Release()

	if code.Ok() {
		code = child.FillEntryOut(out)
	}
	return
}

func (self *Fs) Forget(nodeID, nlookup uint64) {
	self.GetInode(nodeID).Forget(nlookup)
}

func (self *Fs) GetAttr(input *GetAttrIn, out *AttrOut) (code Status) {
	inode := self.GetInode(input.NodeId)
	if inode == nil {
		return ENOENT
	}
	defer inode.Release()

	if input.Flags()&FUSE_GETATTR_FH != 0 {
		// fh := input.Fh()
		// ...
	}
	code = inode.FillAttrOut(out)
	return
}

func (self *Fs) SetAttr(input *SetAttrIn, out *AttrOut) (code Status) {
	inode := self.GetInode(input.NodeId)
	if inode == nil {
		return ENOENT
	}
	defer inode.Release()

	if input.Valid&(FATTR_ATIME|FATTR_MTIME|FATTR_ATIME_NOW|FATTR_MTIME_NOW) != 0 {
		var atime, mtime *time.Time

		now := time.Now()

		if input.Valid&FATTR_ATIME != 0 {
			if input.Valid&FATTR_ATIME_NOW != 0 {
				atime = &now
			} else {
				t := time.Unix(int64(input.Atime),
					int64(input.Atimensec))
				atime = &t
			}
		}

		if input.Valid&FATTR_MTIME != 0 {
			if input.Valid&FATTR_MTIME_NOW != 0 {
				mtime = &now
			} else {
				t := time.Unix(int64(input.Mtime),
					int64(input.Mtimensec))
				mtime = &t
			}
		}
		code = self.access(inode, W_OK, true, &input.Context)
		if code.Ok() {
			code = inode.SetTimes(atime, mtime)
		}
	}

	meta := inode.Meta()
	newmeta := meta.InodeMetaData
	mode_filter := uint32(0)

	// FATTR_FH?
	if input.Valid&FATTR_UID != 0 {
		newmeta.StUid = input.Uid
		if input.Uid != 0 && newmeta.StUid != meta.StUid {
			code = EPERM
			// Non-root setting uid = bad.
			return
		}
	}
	if input.Valid&FATTR_GID != 0 {
		newmeta.StGid = input.Gid
		// Eventually: Check group setting permission for uid
		if input.Uid != 0 {
			mode_filter = syscall.S_ISUID | syscall.S_ISGID
		}
	}
	if input.Valid&FATTR_SIZE != 0 {
		newmeta.StSize = input.Size
	}

	oldmode := meta.StMode
	mode := oldmode
	if input.Valid&FATTR_MODE != 0 {
		mode = uint32(07777) & input.Mode
	}
	mode = mode & ^mode_filter
	newmeta.StMode = mode

	if newmeta != meta.InodeMetaData {
		code = self.access(inode, W_OK, true, &input.Context)
		if !code.Ok() {
			code = EPERM
			return
		}
		meta.InodeMetaData = newmeta
		inode.SetMeta(meta)
		// Eventually: truncate data if size decreases
	}
	code = inode.FillAttrOut(out)
	return
}

func (self *Fs) Release(input *ReleaseIn) {
	self.GetFile(input.Fh).Release()
}

func (self *Fs) ReleaseDir(input *ReleaseIn) {
	self.GetFile(input.Fh).Release()
}

func (self *Fs) OpenDir(input *OpenIn, out *OpenOut) (code Status) {
	inode := self.GetInode(input.NodeId)
	defer inode.Release()

	code = self.access(inode, R_OK|X_OK, false, &input.Context)
	if !code.Ok() {
		return
	}

	out.Fh = inode.GetFile().fh
	return OK

}

func (self *Fs) Open(input *OpenIn, out *OpenOut) (code Status) {
	inode := self.GetInode(input.NodeId)
	defer inode.Release()

	code = self.access(inode, R_OK, false, &input.Context)
	if !code.Ok() {
		return
	}

	out.Fh = inode.GetFile().fh
	return OK
}

func (self *Fs) ReadDir(input *ReadIn, l *DirEntryList) Status {
	dir := self.GetFile(input.Fh)
	dir.SetPos(input.Offset)
	for dir.ReadDirEntry(l) {
	}
	return OK
}

func (self *Fs) ReadDirPlus(input *ReadIn, l *DirEntryList) Status {
	dir := self.GetFile(input.Fh)
	dir.SetPos(input.Offset)
	for dir.ReadDirPlus(input, l) {
	}
	return OK
}

func (self *Fs) Readlink(input *InHeader) (out []byte, code Status) {
	inode := self.GetInode(input.NodeId)
	defer inode.Release()

	code = self.access(inode, R_OK, false, &input.Context)
	if !code.Ok() {
		return
	}
	// Eventually check it is actually link?
	meta := inode.Meta()
	out = meta.Data
	code = OK
	return
}

func (self *Fs) Mkdir(input *MkdirIn, name string, out *EntryOut) (code Status) {
	inode := self.GetInode(input.NodeId)
	defer inode.Release()

	code = self.access(inode, W_OK|X_OK, false, &input.Context)
	if !code.Ok() {
		return
	}

	child := inode.GetChildByName(name)
	defer child.Release()
	if child != nil {
		code = EPERM // XXX should be EEXIST
		return
	}

	var meta InodeMeta
	meta.SetMkdirIn(input)
	child = self.CreateInode()
	defer child.Release()
	child.SetMeta(&meta)
	inode.AddChild(name, child)
	child.FillEntryOut(out)
	return OK
}

func (self *Fs) unlink(input *InHeader, name string, isdir *bool) (code Status) {
	inode := self.GetInode(input.NodeId)
	defer inode.Release()

	child, code := self.lookup(inode, name, &input.Context)
	defer child.Release()
	if !code.Ok() {
		return
	}

	code = self.access(inode, W_OK|X_OK, false, &input.Context)
	if !code.Ok() {
		return
	}
	if isdir != nil && *isdir != child.IsDir() {
		code = EPERM
		return
	}
	inode.RemoveChildByName(name)
	return OK
}

func (self *Fs) Unlink(input *InHeader, name string) (code Status) {
	b := false
	return self.unlink(input, name, &b)
}

func (self *Fs) Rmdir(input *InHeader, name string) (code Status) {
	b := true
	return self.unlink(input, name, &b)
}

func (self *Fs) GetXAttrSize(input *InHeader, attr string) (size int, code Status) {
	b, code := self.GetXAttrData(input, attr)
	if !code.Ok() {
		return
	}
	return len(b), code
}

func (self *Fs) GetXAttrData(input *InHeader, attr string) (data []byte, code Status) {
	inode := self.GetInode(input.NodeId)
	defer inode.Release()

	code = self.access(inode, R_OK, false, &input.Context)
	if !code.Ok() {
		return
	}

	return inode.GetXAttr(attr)
}

func (self *Fs) SetXAttr(input *SetXAttrIn, attr string, data []byte) (code Status) {
	inode := self.GetInode(input.NodeId)
	defer inode.Release()

	code = self.access(inode, W_OK, true, &input.Context)
	if !code.Ok() {
		return
	}

	return inode.SetXAttr(attr, data)
}

func (self *Fs) ListXAttr(input *InHeader) (data []byte, code Status) {
	inode := self.GetInode(input.NodeId)
	defer inode.Release()

	code = self.access(inode, R_OK, false, &input.Context)
	if !code.Ok() {
		return
	}
	b := bytes.NewBuffer([]byte{})
	inode.IterateSubTypeKeys(BST_XATTR,
		func(key BlockKey) bool {
			b.Write([]byte(key.SubTypeData()))
			b.WriteByte(0)
			return true
		})
	data = b.Bytes()
	code = OK
	return
}

func (self *Fs) RemoveXAttr(input *InHeader, attr string) (code Status) {
	inode := self.GetInode(input.NodeId)
	defer inode.Release()

	code = self.access(inode, W_OK, true, &input.Context)
	if !code.Ok() {
		return
	}
	return inode.RemoveXAttr(attr)
}

func (self *Fs) Rename(input *RenameIn, oldName string, newName string) (code Status) {
	inode := self.GetInode(input.NodeId)
	defer inode.Release()

	code = self.access(inode, W_OK|X_OK, true, &input.Context)
	if !code.Ok() {
		return
	}

	child, code := self.lookup(inode, oldName, &input.Context)
	defer child.Release()
	if !code.Ok() {
		return
	}

	new_inode := self.GetInode(input.Newdir)
	defer new_inode.Release()
	code = self.access(new_inode, W_OK|X_OK, true, &input.Context)
	if !code.Ok() {
		return
	}

	new_child, code := self.lookup(new_inode, newName, &input.Context)
	defer new_child.Release()
	if code.Ok() {
		ih := input.InHeader
		ih.NodeId = input.Newdir
		code = self.unlink(&ih, newName, nil)
		if !code.Ok() {
			return
		}
	}

	linkin := LinkIn{InHeader: input.InHeader,
		Oldnodeid: child.ino}
	linkin.NodeId = new_inode.ino
	code = self.Link(&linkin, newName, nil)
	if !code.Ok() {
		return
	}

	code = self.unlink(&input.InHeader, oldName, nil)
	if !code.Ok() {
		return
	}
	return OK
}

func (self *Fs) Link(input *LinkIn, name string, out *EntryOut) (code Status) {
	inode := self.GetInode(input.NodeId)
	defer inode.Release()

	code = self.access(inode, W_OK|X_OK, true, &input.Context)
	if !code.Ok() {
		return
	}

	child, code := self.lookup(inode, name, &input.Context)
	defer child.Release()
	if code.Ok() {
		// code = EEXIST  // should be..
		code = EPERM
		return
	}

	inode.AddChild(name, child)
	return OK

}

func (self *Fs) Access(input *AccessIn) (code Status) {
	inode := self.GetInode(input.NodeId)
	defer inode.Release()

	return self.access(inode, input.Mask, true, &input.Context)
}

func (self *Fs) Mknod(input *MknodIn, name string, out *EntryOut) (code Status) {
	// TBD
	return ENOSYS
}

func (self *Fs) Symlink(input *InHeader, pointedTo string, linkName string, out *EntryOut) (code Status) {
	// TBD
	return ENOSYS
}

func (self *Fs) Create(input *CreateIn, name string, out *CreateOut) (code Status) {
	// TBD
	return ENOSYS
}

func (self *Fs) Read(input *ReadIn, buf []byte) (ReadResult, Status) {
	// TBD
	return nil, ENOSYS
}

func (self *Fs) Flock(input *FlockIn, flags int) Status {
	// TBD
	return ENOSYS
}

func (self *Fs) Write(input *WriteIn, data []byte) (written uint32, code Status) {
	// TBD
	return 0, ENOSYS
}

func (self *Fs) Flush(input *FlushIn) Status {
	// TBD
	return OK
}

func (self *Fs) Fsync(input *FsyncIn) (code Status) {
	// TBD
	return ENOSYS
}

func (self *Fs) FsyncDir(input *FsyncIn) (code Status) {
	// TBD
	return ENOSYS
}

func (self *Fs) Fallocate(in *FallocateIn) (code Status) {
	// TBD
	return ENOSYS
}
