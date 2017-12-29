/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2017 Markus Stenberg
 *
 * Created:       Thu Dec 28 12:52:43 2017 mstenber
 * Last modified: Fri Dec 29 09:25:23 2017 mstenber
 * Edit time:     21 min
 *
 */

package fs

import (
	"log"
	"os"

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

func (self *Fs) StatFs(header *InHeader, out *StatfsOut) Status {
	bsize := blockSize
	out.Bsize = uint32(bsize)
	avail := self.storage.Backend.GetBytesAvailable()
	out.Bfree = uint64(avail / bsize)
	out.Bavail = uint64(avail / bsize)
	used := self.storage.Backend.GetBytesUsed()
	out.Blocks = uint64(used / bsize)
	return OK
}

func (self *Fs) Lookup(header *InHeader, name string, out *EntryOut) (code Status) {
	parent := self.GOCInode(header.NodeId)
	if parent == nil {
		log.Printf("Lookup @nondir %v", header.NodeId)
		return ENOTDIR
	}
	defer parent.Release()

	child := parent.GetChildByName(name)
	if child == nil {
		return ENOENT
	}

	child.FillEntry(out)

	return OK
}

func (self *Fs) Forget(nodeID, nlookup uint64) {
	self.GetInode(nodeID).Forget(nlookup)
}

func (self *Fs) GetAttr(input *GetAttrIn, out *AttrOut) (code Status) {
	// TBD
	return ENOSYS
}

func (self *Fs) Open(input *OpenIn, out *OpenOut) (status Status) {
	// TBD
	return OK
}

func (self *Fs) SetAttr(input *SetAttrIn, out *AttrOut) (code Status) {
	// TBD
	return ENOSYS
}

func (self *Fs) Readlink(header *InHeader) (out []byte, code Status) {
	// TBD
	return nil, ENOSYS
}

func (self *Fs) Mknod(input *MknodIn, name string, out *EntryOut) (code Status) {
	// TBD
	return ENOSYS
}

func (self *Fs) Mkdir(input *MkdirIn, name string, out *EntryOut) (code Status) {
	// TBD
	return ENOSYS
}

func (self *Fs) Unlink(header *InHeader, name string) (code Status) {
	// TBD
	return ENOSYS
}

func (self *Fs) Rmdir(header *InHeader, name string) (code Status) {
	// TBD
	return ENOSYS
}

func (self *Fs) Symlink(header *InHeader, pointedTo string, linkName string, out *EntryOut) (code Status) {
	// TBD
	return ENOSYS
}

func (self *Fs) Rename(input *RenameIn, oldName string, newName string) (code Status) {
	// TBD
	return ENOSYS
}

func (self *Fs) Link(input *LinkIn, name string, out *EntryOut) (code Status) {
	// TBD
	return ENOSYS
}

func (self *Fs) GetXAttrSize(header *InHeader, attr string) (size int, code Status) {
	// TBD
	return 0, ENOSYS
}

func (self *Fs) GetXAttrData(header *InHeader, attr string) (data []byte, code Status) {
	// TBD
	return nil, ENOATTR
}

func (self *Fs) SetXAttr(input *SetXAttrIn, attr string, data []byte) Status {
	// TBD
	return ENOSYS
}

func (self *Fs) ListXAttr(header *InHeader) (data []byte, code Status) {
	// TBD
	return nil, ENOSYS
}

func (self *Fs) RemoveXAttr(header *InHeader, attr string) Status {
	// TBD
	return ENOSYS
}

func (self *Fs) Access(input *AccessIn) (code Status) {
	// TBD
	return ENOSYS
}

func (self *Fs) Create(input *CreateIn, name string, out *CreateOut) (code Status) {
	// TBD
	return ENOSYS
}

func (self *Fs) OpenDir(input *OpenIn, out *OpenOut) (status Status) {
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

func (self *Fs) Release(input *ReleaseIn) {
	// TBD
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

func (self *Fs) ReadDir(input *ReadIn, l *DirEntryList) Status {
	// TBD
	return ENOSYS
}

func (self *Fs) ReadDirPlus(input *ReadIn, l *DirEntryList) Status {
	// TBD
	return ENOSYS
}

func (self *Fs) ReleaseDir(input *ReleaseIn) {
	// TBD
}

func (self *Fs) FsyncDir(input *FsyncIn) (code Status) {
	// TBD
	return ENOSYS
}

func (self *Fs) Fallocate(in *FallocateIn) (code Status) {
	// TBD
	return ENOSYS
}
