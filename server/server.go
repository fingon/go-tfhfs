/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2018 Markus Stenberg
 *
 * Created:       Tue Jan 16 14:38:35 2018 mstenber
 * Last modified: Wed Jan 17 17:14:40 2018 mstenber
 * Edit time:     135 min
 *
 */

package server

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/fingon/go-tfhfs/fs"
	"github.com/fingon/go-tfhfs/ibtree/hugger"
	. "github.com/fingon/go-tfhfs/pb"
	"github.com/fingon/go-tfhfs/storage"
)

const rootName = "sync"

type Server struct {
	// We have our own tree (rooted at 'rootName')
	hugger.Hugger

	Family, Address string
	Fs              *fs.Fs
	Storage         *storage.Storage
}

func (self Server) Init() *Server {
	self.RootName = rootName
	self.Hugger.Storage = self.Storage
	(&self.Hugger).Init(0)
	twirpHandler := NewFsServer(&self, nil)
	go func() {
		http.ListenAndServe(self.Address, twirpHandler)
	}()
	// Load the root
	self.Hugger.RootIsNew()
	return &self
}

func (self *Server) Close() {
	// TBD how to clean this up correctly
}

func (self *Server) ClearBlocksInName(ctx context.Context, n *BlockName) (*ClearResult, error) {
	self.Update(func(tr *hugger.Transaction) {
		k1 := fs.NewBlockKeyNameBlock(n.Name, "").IB()
		k2 := fs.NewBlockKeyNameEnd(n.Name).IB()
		tr.IB().DeleteRange(k1, k2)
	})
	return &ClearResult{}, nil
}

func (self *Server) GetBlockIdByName(ctx context.Context, name *BlockName) (*BlockId, error) {
	if name.Name == self.Fs.RootName {
		block := self.Fs.RootBlock()
		if block != nil {
			return &BlockId{Id: block.Id()}, nil

		}
	}
	id := self.Storage.GetBlockIdByName(name.Name)
	return &BlockId{Id: id}, nil
}

func (self *Server) getBlock(id string, wantData, wantMissing bool) *Block {
	b := self.Storage.GetBlockById(id)
	if b == nil {
		return &Block{}
	}
	defer b.Close()
	res := &Block{Id: id, Status: int32(b.Status())}
	if wantData {
		res.Data = string(b.Data())
	}
	if wantMissing && b.Status() == storage.BS_WEAK {
		missing := make([]string, 0)
		b.IterateReferences(func(id string) {
			b2 := self.Storage.GetBlockById(id)
			if b2 == nil {
				missing = append(missing, id)
				return
			}
			b2.Close()
		})
		res.MissingIds = missing
	}
	return res
}

func (self *Server) GetBlockById(ctx context.Context, req *GetBlockRequest) (*Block, error) {
	return self.getBlock(req.Id, req.WantData, req.WantMissing), nil
}

func (self *Server) MergeBlockNameTo(ctx context.Context, req *MergeRequest) (*MergeResult, error) {
	n0 := fmt.Sprintf("%s.%s", req.FromName, req.ToName)
	b0, _, _ := self.Fs.LoadNodeByName(n0)
	b, _, ok := self.Fs.LoadNodeByName(req.FromName)
	if !ok {
		log.Panic("nonexistent src")
	}
	if req.ToName != self.Fs.RootName {
		log.Panic("non-fs merges not supported")
	}
	self.Fs.Update(func(tr *hugger.Transaction) {
		fs.MergeTo3(tr, b0, b, false)
	})
	block := self.Fs.RootBlock()
	self.Storage.SetNameToBlockId(n0, block.Id())
	return &MergeResult{Ok: true}, nil
}

func (self *Server) SetNameToBlockId(ctx context.Context, req *SetNameRequest) (*SetNameResult, error) {
	res := &SetNameResult{Ok: true}
	self.Storage.SetNameToBlockId(req.Name, req.Id)
	return res, nil
}

func (self *Server) StoreBlock(ctx context.Context, req *StoreRequest) (*Block, error) {
	var bid string
	self.Update(func(tr *hugger.Transaction) {
		bdata := []byte(req.Block.Data)
		st := storage.BlockStatus(req.Block.Status)
		bl := tr.GetStorageBlock(st, bdata, nil)
		k := fs.NewBlockKeyNameBlock(req.Name, bl.Id()).IB()
		tr.IB().Set(k, bl.Id())
		bid = bl.Id()
	})

	return self.getBlock(bid, false, true), nil
}

func (self *Server) UpgradeBlockNonWeak(ctx context.Context, bid *BlockId) (*Block, error) {
	b := self.Storage.GetBlockById(bid.Id)
	if b != nil {
		b.SetStatus(storage.BS_NORMAL)
	}
	return self.getBlock(bid.Id, false, true), nil
}
