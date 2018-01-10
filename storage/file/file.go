/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2018 Markus Stenberg
 *
 * Created:       Wed Jan  3 15:44:41 2018 mstenber
 * Last modified: Wed Jan 10 11:32:59 2018 mstenber
 * Edit time:     82 min
 *
 */

package file

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/fingon/go-tfhfs/mlog"
	"github.com/fingon/go-tfhfs/storage"
	"github.com/fingon/go-tfhfs/util"
)

// fileBackend stores the blocks in file directory hierarchy.
//
// Name encoding:
//
// - names/ directory has files with base64 encoded name of link,
// containing raw bytes for the block id.
//
// Block encoding:
//
// - blocks/ directory contains data blocks, with hex dumped block ids
// as names, followed by underscore, # of links, underscore, and type.
//
// Number of characters used for subdirectory name can be also chosen,
// as keeping all blocks in same location does not make sense.

const directoryBytes = 2 // 65536 subdirs should be plenty

type fileBackend struct {
	storage.DirectoryBackendBase
	created     map[string]bool
	createdLock util.MutexLocked
}

var _ storage.Backend = &fileBackend{}

func NewFileBackend() storage.Backend {
	self := &fileBackend{}
	return self
}

func (self *fileBackend) Init(config storage.BackendConfiguration) {
	(&self.DirectoryBackendBase).Init(config)
}

func (self *fileBackend) delay() {
	if self.DelayPerOp > 0 {
		time.Sleep(self.DelayPerOp)
	}
}

func (self *fileBackend) DeleteBlock(bl *storage.Block) {
	self.delay()
	_, path := self.blockPath(bl, bl.Stored)
	err := os.Remove(path)
	if err != nil {
		log.Panic(err)
	}
}

func (self *fileBackend) mkdirAllRec(path string) {
	if self.created == nil {
		self.created = make(map[string]bool)
	}
	if path == "" {
		return
	}
	if self.created[path] {
		return
	}
	if path != self.Directory {
		dir, _ := filepath.Split(path)
		if len(dir) < len(path) {
			self.mkdirAllRec(dir)
		}
	}
	os.Mkdir(path, 0700)
	self.created[path] = true
}

func (self *fileBackend) mkdirAll(path string) {
	defer self.createdLock.Locked()()
	self.mkdirAllRec(path)
}

func (self *fileBackend) GetBlockData(bl *storage.Block) []byte {
	self.delay()
	_, path := self.blockPath(bl, bl.Stored)
	b, err := ioutil.ReadFile(path)
	if err != nil {
		log.Panic(err)
	}
	return b
}

func (self *fileBackend) GetBlockById(id string) *storage.Block {
	mlog.Printf2("storage/file/file", "fbb.GetBlockById %x", id)
	self.delay()
	dir := fmt.Sprintf("%s/blocks/%x", self.Directory, id[:directoryBytes])
	prefix := fmt.Sprintf("%x_", id[directoryBytes:])
	fis, err := ioutil.ReadDir(dir)
	if err != nil {
		// I suppose if we cannot access the directory block
		// does not exist
		mlog.Printf2("storage/file/file", " even ReadDir does not work: %v", err)
		return nil
	}
	for _, v := range fis {
		n := v.Name()
		mlog.Printf2("storage/file/file", " considering %v", n)
		if n[:len(prefix)] != prefix {
			continue
		}
		arr := strings.Split(n, "_")
		refcount, err := strconv.Atoi(arr[1])
		if err != nil {
			continue
		}
		status, err := strconv.Atoi(arr[2])
		if err != nil {
			continue
		}
		mlog.Printf2("storage/file/file", " found")
		meta := storage.BlockMetadata{RefCount: int32(refcount),
			Status: storage.BlockStatus(status)}
		return &storage.Block{Id: id, Backend: self,
			BlockMetadata: meta}
	}
	return nil
}

func (self *fileBackend) GetBlockIdByName(name string) string {
	mlog.Printf2("storage/file/file", "fbb.GetBlockIdByName %v", name)
	path := fmt.Sprintf("%s/names/%x", self.Directory, name)
	b, err := ioutil.ReadFile(path)
	if err != nil {
		mlog.Printf2("storage/file/file", " nope, %v", err)
		return ""
	}
	return string(b)
}

func (self *fileBackend) SetInFlush(value bool) {
}

func (self *fileBackend) SetNameToBlockId(name, block_id string) {
	mlog.Printf2("storage/file/file", "fbb.SetNameToBlockId %v %x", name, block_id)
	dir := fmt.Sprintf("%s/names", self.Directory)
	path := fmt.Sprintf("%s/%x", dir, name)
	self.mkdirAll(dir)
	if block_id == "" {
		err := os.Remove(path)
		if err != nil {
			log.Panic(err)
		}
		return
	}
	err := ioutil.WriteFile(path, []byte(block_id), 0600)
	if err != nil {
		log.Panic(err)
	}
	mlog.Printf2("storage/file/file", " wrote to %v", path)
}

func (self *fileBackend) StoreBlock(bl *storage.Block) {
	self.delay()
	dir, path := self.blockPath(bl, nil)
	self.mkdirAll(dir)
	err := ioutil.WriteFile(path, *bl.Data.Get(), 0600)
	if err != nil {
		log.Panic(err)
	}
	mlog.Printf2("storage/file/file", "fbb.StoreBlock %x to %v", bl.Id, path)
}

func (self *fileBackend) UpdateBlock(bl *storage.Block) int {
	mlog.Printf2("storage/file/file", "fbb.UpdateBlock %x", bl.Id)
	self.delay()
	if bl.Stored == nil {
		log.Panic(".Stored is not set")
	}
	_, oldpath := self.blockPath(bl, bl.Stored)
	mlog.Printf2("storage/file/file", " oldpath:%v", oldpath)
	_, newpath := self.blockPath(bl, nil)
	mlog.Printf2("storage/file/file", " newpath:%v", newpath)
	err := os.Rename(oldpath, newpath)
	if err != nil {
		log.Panic(err)
	}
	mlog.Printf2("storage/file/file", "fbb.UpdateBlock %x", bl.Id)
	return 1
}

func (self *fileBackend) Close() {
	self.delay()
}

func (self *fileBackend) blockPath(b *storage.Block, metadata *storage.BlockMetadata) (dir string, full string) {
	if metadata == nil {
		metadata = &b.BlockMetadata
	}
	dir = fmt.Sprintf("%s/blocks/%x", self.Directory, b.Id[:directoryBytes])
	full = fmt.Sprintf("%s/%x_%v_%v",
		dir, b.Id[directoryBytes:], metadata.RefCount, metadata.Status)
	return
}
