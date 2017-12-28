/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2017 Markus Stenberg
 *
 * Created:       Thu Dec 28 12:52:43 2017 mstenber
 * Last modified: Thu Dec 28 13:00:11 2017 mstenber
 * Edit time:     3 min
 *
 */

package fs

import (
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
	return ENOSYS
}

func (self *Fs) Lookup(header *InHeader, name string, out *EntryOut) (code Status) {
	return ENOSYS
}

func (self *Fs) Forget(nodeID, nlookup uint64) {
}

func (self *Fs) GetAttr(input *GetAttrIn, out *AttrOut) (code Status) {
	return ENOSYS
}

func (self *Fs) Open(input *OpenIn, out *OpenOut) (status Status) {
	return OK
}

func (self *Fs) SetAttr(input *SetAttrIn, out *AttrOut) (code Status) {
	return ENOSYS
}

func (self *Fs) Readlink(header *InHeader) (out []byte, code Status) {
	return nil, ENOSYS
}

func (self *Fs) Mknod(input *MknodIn, name string, out *EntryOut) (code Status) {
	return ENOSYS
}

func (self *Fs) Mkdir(input *MkdirIn, name string, out *EntryOut) (code Status) {
	return ENOSYS
}

func (self *Fs) Unlink(header *InHeader, name string) (code Status) {
	return ENOSYS
}

func (self *Fs) Rmdir(header *InHeader, name string) (code Status) {
	return ENOSYS
}

func (self *Fs) Symlink(header *InHeader, pointedTo string, linkName string, out *EntryOut) (code Status) {
	return ENOSYS
}

func (self *Fs) Rename(input *RenameIn, oldName string, newName string) (code Status) {
	return ENOSYS
}

func (self *Fs) Link(input *LinkIn, name string, out *EntryOut) (code Status) {
	return ENOSYS
}

func (self *Fs) GetXAttrSize(header *InHeader, attr string) (size int, code Status) {
	return 0, ENOSYS
}

func (self *Fs) GetXAttrData(header *InHeader, attr string) (data []byte, code Status) {
	return nil, ENOATTR
}

func (self *Fs) SetXAttr(input *SetXAttrIn, attr string, data []byte) Status {
	return ENOSYS
}

func (self *Fs) ListXAttr(header *InHeader) (data []byte, code Status) {
	return nil, ENOSYS
}

func (self *Fs) RemoveXAttr(header *InHeader, attr string) Status {
	return ENOSYS
}

func (self *Fs) Access(input *AccessIn) (code Status) {
	return ENOSYS
}

func (self *Fs) Create(input *CreateIn, name string, out *CreateOut) (code Status) {
	return ENOSYS
}

func (self *Fs) OpenDir(input *OpenIn, out *OpenOut) (status Status) {
	return ENOSYS
}

func (self *Fs) Read(input *ReadIn, buf []byte) (ReadResult, Status) {
	return nil, ENOSYS
}

func (self *Fs) Flock(input *FlockIn, flags int) Status {
	return ENOSYS
}

func (self *Fs) Release(input *ReleaseIn) {
}

func (self *Fs) Write(input *WriteIn, data []byte) (written uint32, code Status) {
	return 0, ENOSYS
}

func (self *Fs) Flush(input *FlushIn) Status {
	return OK
}

func (self *Fs) Fsync(input *FsyncIn) (code Status) {
	return ENOSYS
}

func (self *Fs) ReadDir(input *ReadIn, l *DirEntryList) Status {
	return ENOSYS
}

func (self *Fs) ReadDirPlus(input *ReadIn, l *DirEntryList) Status {
	return ENOSYS
}

func (self *Fs) ReleaseDir(input *ReleaseIn) {
}

func (self *Fs) FsyncDir(input *FsyncIn) (code Status) {
	return ENOSYS
}

func (self *Fs) Fallocate(in *FallocateIn) (code Status) {
	return ENOSYS
}
