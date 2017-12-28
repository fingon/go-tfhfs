/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2017 Markus Stenberg
 *
 * Created:       Thu Dec 28 11:20:29 2017 mstenber
 * Last modified: Thu Dec 28 14:34:49 2017 mstenber
 * Edit time:     77 min
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
	"encoding/binary"
	"log"

	"github.com/fingon/go-tfhfs/codec"
	"github.com/fingon/go-tfhfs/ibtree"
	"github.com/fingon/go-tfhfs/storage"
	"github.com/hanwen/go-fuse/fuse"
)

const iterations = 1234
const inodeDataLength = 8
const objectSubTypeOffset = inodeDataLength

type Fs struct {
	server   *fuse.Server
	tree     *ibtree.IBTree
	storage  *storage.Storage
	rootName string
	treeRoot *ibtree.IBNode
}

type FsTreeKey string

func (self FsTreeKey) ObjectSubType() ObjectSubType {
	return ObjectSubType(self[objectSubTypeOffset])
}

func (self FsTreeKey) Ino() uint64 {
	b := []byte(self[:inodeDataLength])
	return binary.BigEndian.Uint64(b)
}

func NewFsTreeKey(ino uint64, st ObjectSubType, data string) FsTreeKey {
	b := make([]byte, inodeDataLength+1, inodeDataLength+1+len(data))
	binary.BigEndian.PutUint64(b, ino)
	b[inodeDataLength] = byte(st)
	b = append(b, []byte(data)...)
	return FsTreeKey(b)
}

func (self FsTreeKey) SubTypeData() string {
	return string(self[inodeDataLength+1:])
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
	nb := make([]byte, 0, len(b)+1)
	nb[0] = byte(BDT_NODE)
	copy(nb[1:], b)
	block := self.storage.ReferOrStoreBlock(string(bid), string(nb))
	self.storage.ReleaseBlockId(block.Id)
	// By default this won't increase references; however, stuff
	// that happens 'elsewhere' (e.g. taking root reference) does,
	// and due to thattransitively, also this does.
	return ibtree.BlockId(block.Id)
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
		k := FsTreeKey(c.Key)
		switch k.ObjectSubType() {
		case OST_FILE_OFFSET2EXTENT:
			cb(c.Value)
		}
	}
}

func NewFs(st *storage.Storage, rootName string) *Fs {
	fs := &Fs{storage: st, rootName: rootName}
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
	} else {
		fs.treeRoot = fs.tree.NewRoot()
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
