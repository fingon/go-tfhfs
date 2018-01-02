/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2017 Markus Stenberg
 *
 * Created:       Thu Dec 28 11:20:29 2017 mstenber
 * Last modified: Tue Jan  2 18:45:50 2018 mstenber
 * Edit time:     134 min
 *
 */

// fs package implements fuse.RawFileSystem on top of the other
// modules of go-tfhfs project.
//
// The low-level API is more or less mandatory as e.g. list of files
// from huge directory is not feasible to provide in one chunk, and we
// want to have fairly dynamic relationship with the tree.
package fs

import (
	"crypto/sha256"
	"log"
	"os"

	"github.com/fingon/go-tfhfs/codec"
	"github.com/fingon/go-tfhfs/ibtree"
	"github.com/fingon/go-tfhfs/mlog"
	"github.com/fingon/go-tfhfs/storage"
	"github.com/hanwen/go-fuse/fuse"
)

const iterations = 1234
const inodeDataLength = 8
const blockSubTypeOffset = inodeDataLength

type Fs struct {
	InodeTracker
	server          *fuse.Server
	tree            *ibtree.IBTree
	storage         *storage.Storage
	rootName        string
	treeRoot        *ibtree.IBNode
	treeRootBlockId ibtree.BlockId
	bidMap          map[string]bool
}

var _ fuse.RawFileSystem = &Fs{}

func (self *Fs) LoadNodeFromString(data string) *ibtree.IBNodeData {
	bd := []byte(data)
	dt := BlockDataType(bd[0])
	if dt != BDT_NODE {
		return nil
	}
	nd := &ibtree.IBNodeData{}
	_, err := nd.UnmarshalMsg(bd[1:])
	if err != nil {
		log.Panic(err)
	}
	return nd
}

// ibtree.IBTreeBackend API
func (self *Fs) LoadNode(id ibtree.BlockId) *ibtree.IBNodeData {
	mlog.Printf2("fs/fs", "fs.LoadNode %x", id)
	b := self.storage.GetBlockById(string(id))
	if b == nil {
		return nil
	}
	return self.LoadNodeFromString(b.GetData())
}

func (self *Fs) getBlockDataId(blockType BlockDataType, data string) ibtree.BlockId {
	b := []byte(data)
	h := sha256.Sum256(b)
	bid := h[:]
	nb := make([]byte, len(b)+1)
	nb[0] = byte(blockType)
	copy(nb[1:], b)
	block := self.storage.ReferOrStoreBlock(string(bid), string(nb))
	self.storage.ReleaseBlockId(block.Id)
	// By default this won't increase references; however, stuff
	// that happens 'elsewhere' (e.g. taking root reference) does,
	// and due to thattransitively, also this does.
	self.bidMap[block.Id] = true
	return ibtree.BlockId(block.Id)
}

// ibtree.IBTreeBackend API
func (self *Fs) SaveNode(nd ibtree.IBNodeData) ibtree.BlockId {
	b, err := nd.MarshalMsg(nil)
	if err != nil {
		log.Panic(err)
	}
	return self.getBlockDataId(BDT_NODE, string(b))
}

func (self *Fs) GetTransaction() *ibtree.IBTransaction {
	// mlog.Printf2("fs/fs", "GetTransaction of %p", self.treeRoot)
	return ibtree.NewTransaction(self.treeRoot)
}

func (self *Fs) CommitTransaction(t *ibtree.IBTransaction) {
	self.treeRoot, self.treeRootBlockId = t.Commit()
	mlog.Printf2("fs/fs", "CommitTransaction %p", self.treeRoot)
	self.treeRoot.PrintToMLogDirty()
	self.storage.SetNameToBlockId(self.rootName, string(self.treeRootBlockId))
}

// ListDir provides testing utility as output of ReadDir/ReadDirPlus
// is binary garbage and I am too lazy to write a decoder for it.
func (self *Fs) ListDir(ino uint64) (ret []string) {
	mlog.Printf2("fs/fs", "Fs.ListDir #%d", ino)
	inode := self.GetInode(ino)
	defer inode.Release()

	file := inode.GetFile(uint32(os.O_RDONLY))
	defer file.Release()
	for {
		inode, name := file.ReadNextInode()
		if inode == nil {
			return
		}
		file.pos++
		defer inode.Release()
		mlog.Printf2("fs/fs", " %s", name)
		ret = append(ret, name)
	}
	return
}

// These are pre-flush references to blocks; I didn't come up with
// better scheme than this, so we keep that and clear it at flush
// time.
func (self *Fs) hasExternalReferences(id string) bool {
	return self.bidMap[id]
}

func (self *Fs) iterateReferencesCallback(data string, cb storage.BlockReferenceCallback) {
	nd := self.LoadNodeFromString(data)
	if nd == nil {
		return
	}
	if !nd.Leafy {
		for _, c := range nd.Children {
			cb(c.Value)
		}
		return
	}
	for _, c := range nd.Children {
		k := BlockKey(c.Key)
		switch k.SubType() {
		case BST_FILE_OFFSET2EXTENT:
			cb(c.Value)
		}
	}
}

func (self *Fs) StorageFlush() int {
	// self.storage.SetNameToBlockId(self.rootName, string(self.treeRootBlockId))
	// ^ done in each commit, so pointless here?
	rv := self.storage.Flush()
	self.bidMap = make(map[string]bool)

	return rv
}

func NewFs(st *storage.Storage, rootName string) *Fs {
	fs := &Fs{storage: st, rootName: rootName}
	fs.InodeTracker.Init(fs)
	fs.tree = ibtree.IBTree{}.Init(fs)
	fs.bidMap = make(map[string]bool)
	st.HasExternalReferencesCallback = func(id string) bool {
		return fs.hasExternalReferences(id)
	}
	st.IterateReferencesCallback = func(data string, cb storage.BlockReferenceCallback) {
		fs.iterateReferencesCallback(data, cb)
	}
	rootbid := st.GetBlockIdByName(rootName)
	if rootbid != "" {
		bid := ibtree.BlockId(rootbid)
		fs.treeRoot = fs.tree.LoadRoot(bid)
		fs.treeRootBlockId = bid
	}
	if fs.treeRoot == nil {
		fs.treeRoot = fs.tree.NewRoot()
		// getInode succeeds always; Get does not
		root := fs.getInode(fuse.FUSE_ROOT_ID)
		var meta InodeMeta
		meta.StMode = 0777 | fuse.S_IFDIR
		root.SetMeta(&meta)
	}
	return fs
}

func NewBadgerCryptoFs(storedir, password, salt, rootName string) *Fs {
	c1 := codec.EncryptingCodec{}.Init([]byte(password), []byte(salt), iterations)
	c2 := &codec.CompressingCodec{}
	c := codec.CodecChain{}.Init(c1, c2)

	backend := storage.BadgerBlockBackend{}.Init(storedir)

	st := storage.Storage{Codec: c, Backend: backend}.Init()
	return NewFs(st, rootName)
}
