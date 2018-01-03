/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2018 Markus Stenberg
 *
 * Created:       Wed Jan  3 15:44:41 2018 mstenber
 * Last modified: Wed Jan  3 17:31:36 2018 mstenber
 * Edit time:     68 min
 *
 */

package storage

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/fingon/go-tfhfs/mlog"
)

// FileBlockBackend stores the blocks in file directory hierarchy.
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

type FileBlockBackend struct {
	DirectoryBlockBackendBase
	created map[string]bool
}

var _ BlockBackend = &FileBlockBackend{}

func (self *FileBlockBackend) DeleteBlock(bl *Block) {
	_, path := self.blockPath(bl, bl.stored)
	err := os.Remove(path)
	if err != nil {
		log.Panic(err)
	}
}

func (self *FileBlockBackend) mkdirAll(path string) {
	if self.created == nil {
		self.created = make(map[string]bool)
	}
	if path == "" {
		return
	}
	if self.created[path] {
		return
	}
	if path != self.dir {
		dir, _ := filepath.Split(path)
		if len(dir) < len(path) {
			self.mkdirAll(dir)
		}
	}
	os.Mkdir(path, 0700)
	self.created[path] = true

}

func (self *FileBlockBackend) GetBlockData(bl *Block) string {
	_, path := self.blockPath(bl, bl.stored)
	b, err := ioutil.ReadFile(path)
	if err != nil {
		log.Panic(err)
	}
	return string(b)
}

func (self *FileBlockBackend) GetBlockById(id string) *Block {
	mlog.Printf2("storage/file", "fbb.GetBlockById %x", id)
	dir := fmt.Sprintf("%s/blocks/%x", self.dir, id[:directoryBytes])
	prefix := fmt.Sprintf("%x_", id[directoryBytes:])
	fis, err := ioutil.ReadDir(dir)
	if err != nil {
		// I suppose if we cannot access the directory block
		// does not exist
		mlog.Printf2("storage/file", " even ReadDir does not work: %v", err)
		return nil
	}
	for _, v := range fis {
		n := v.Name()
		mlog.Printf2("storage/file", " considering %v", n)
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
		mlog.Printf2("storage/file", " found")
		return &Block{Id: id, backend: self,
			BlockMetadata: BlockMetadata{RefCount: refcount,
				Status: BlockStatus(status)}}
	}
	return nil
}

func (self *FileBlockBackend) GetBlockIdByName(name string) string {
	mlog.Printf2("storage/file", "fbb.GetBlockIdByName %v", name)
	path := fmt.Sprintf("%s/names/%x", self.dir, name)
	b, err := ioutil.ReadFile(path)
	if err != nil {
		mlog.Printf2("storage/file", " nope, %v", err)
		return ""
	}
	return string(b)
}

func (self *FileBlockBackend) SetInFlush(value bool) {
}

func (self *FileBlockBackend) SetNameToBlockId(name, block_id string) {
	mlog.Printf2("storage/file", "fbb.SetNameToBlockId %v %x", name, block_id)
	dir := fmt.Sprintf("%s/names", self.dir)
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
	mlog.Printf2("storage/file", " wrote to %v", path)
}

func (self *FileBlockBackend) StoreBlock(bl *Block) {
	dir, path := self.blockPath(bl, nil)
	self.mkdirAll(dir)
	data := bl.GetCodecData()
	err := ioutil.WriteFile(path, []byte(data), 0600)
	if err != nil {
		log.Panic(err)
	}
	mlog.Printf2("storage/file", "fbb.StoreBlock %x to %v", bl.Id, path)
}

func (self *FileBlockBackend) UpdateBlock(bl *Block) int {
	mlog.Printf2("storage/file", "fbb.UpdateBlock %x", bl.Id)
	if bl.stored == nil {
		log.Panic(".stored is not set")
	}
	_, oldpath := self.blockPath(bl, bl.stored)
	mlog.Printf2("storage/file", " oldpath:%v", oldpath)
	_, newpath := self.blockPath(bl, nil)
	mlog.Printf2("storage/file", " newpath:%v", newpath)
	err := os.Rename(oldpath, newpath)
	if err != nil {
		log.Panic(err)
	}
	mlog.Printf2("storage/file", "fbb.UpdateBlock %x", bl.Id)
	return 1
}

func (self *FileBlockBackend) Close() {
}

func (self *FileBlockBackend) blockPath(b *Block, metadata *BlockMetadata) (dir string, full string) {
	if metadata == nil {
		metadata = &b.BlockMetadata
	}
	dir = fmt.Sprintf("%s/blocks/%x", self.dir, b.Id[:directoryBytes])
	full = fmt.Sprintf("%s/%x_%v_%v",
		dir, b.Id[directoryBytes:], metadata.RefCount, metadata.Status)
	return
}
