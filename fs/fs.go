/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2017 Markus Stenberg
 *
 * Created:       Thu Dec 28 11:20:29 2017 mstenber
 * Last modified: Fri Dec 29 14:30:04 2017 mstenber
 * Edit time:     102 min
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

	"github.com/fingon/go-tfhfs/codec"
	"github.com/fingon/go-tfhfs/ibtree"
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
}

var _ fuse.RawFileSystem = &Fs{}

// ibtree.IBTreeBackend API
func (self *Fs) LoadNode(id ibtree.BlockId) *ibtree.IBNodeData {
	b := self.storage.GetBlockById(string(id))
	if b == nil {
		return nil
	}
	bd := []byte(b.GetData())
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
func (self *Fs) SaveNode(nd ibtree.IBNodeData) ibtree.BlockId {
	b, err := nd.MarshalMsg(nil)
	if err != nil {
		log.Panic(err)
	}
	h := sha256.Sum256(b)
	bid := h[:]
	nb := make([]byte, len(b)+1)
	nb[0] = byte(BDT_NODE)
	copy(nb[1:], b)
	block := self.storage.ReferOrStoreBlock(string(bid), string(nb))
	self.storage.ReleaseBlockId(block.Id)
	// By default this won't increase references; however, stuff
	// that happens 'elsewhere' (e.g. taking root reference) does,
	// and due to thattransitively, also this does.
	return ibtree.BlockId(block.Id)
}

func (self *Fs) GetTransaction() *ibtree.IBTransaction {
	return ibtree.NewTransaction(self.treeRoot)
}

func (self *Fs) CommitTransaction(t *ibtree.IBTransaction) {
	self.treeRoot, self.treeRootBlockId = t.Commit()
	self.storage.SetNameToBlockId(self.rootName, string(self.treeRootBlockId))
}

// We don't refer to blocks at all (TBD: Get rid of the feature? it is
// relic of Python era?)
func (self *Fs) hasExternalReferences(id string) bool {
	return false
}

func (self *Fs) iterateReferencesCallback(id string, cb storage.BlockReferenceCallback) {
	nd := self.LoadNode(ibtree.BlockId(id))
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

func NewFs(st *storage.Storage, rootName string) *Fs {
	fs := &Fs{storage: st, rootName: rootName}
	fs.InodeTracker.Init(fs)
	fs.tree = ibtree.IBTree{}.Init(fs)
	st.HasExternalReferencesCallback = func(id string) bool {
		return fs.hasExternalReferences(id)
	}
	st.IterateReferencesCallback = func(id string, cb storage.BlockReferenceCallback) {
		fs.iterateReferencesCallback(id, cb)
	}
	rootbid := st.GetBlockIdByName(rootName)
	if rootbid != "" {
		fs.treeRoot = fs.tree.LoadRoot(ibtree.BlockId(rootbid))
	}
	if fs.treeRoot == nil {
		fs.treeRoot = fs.tree.NewRoot()
		root := fs.GetInode(fuse.FUSE_ROOT_ID)
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
