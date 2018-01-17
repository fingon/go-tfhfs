/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2018 Markus Stenberg
 *
 * Created:       Tue Jan 16 14:38:35 2018 mstenber
 * Last modified: Wed Jan 17 13:28:26 2018 mstenber
 * Edit time:     98 min
 *
 */

package server

import (
	"context"
	"log"
	"net"

	"github.com/fingon/go-tfhfs/fs"
	"github.com/fingon/go-tfhfs/ibtree/hugger"
	. "github.com/fingon/go-tfhfs/pb"
	"github.com/fingon/go-tfhfs/storage"
	"google.golang.org/grpc"
)

const rootName = "sync"

type Server struct {
	// We have our own tree (rooted at 'rootName')
	hugger.Hugger

	Family, Address string
	Fs              *fs.Fs
	Storage         *storage.Storage
	grpcServer      *grpc.Server
}

func (self *Server) Init() *Server {
	self.RootName = rootName
	self.Hugger.Storage = self.Storage
	lis, err := net.Listen(self.Family, self.Address)
	if err != nil {
		log.Panic(err)
	}
	grpcServer := grpc.NewServer()
	RegisterFsServer(grpcServer, self)
	grpcServer.Serve(lis)
	self.grpcServer = grpcServer
	// Load the root
	self.Hugger.RootIsNew()
	return self
}

func (self *Server) Close() {
	self.grpcServer.Stop()
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
	id := self.Storage.GetBlockIdByName(name.Name)
	return &BlockId{Id: id}, nil
}

func (self *Server) getBlock(id string, wantData, wantMissing bool) *Block {
	b := self.Storage.GetBlockById(id)
	if b == nil {
		return nil
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
	// TBD
	return &MergeResult{}, nil
}

func (self *Server) SetNameToBlockId(ctx context.Context, req *SetNameRequest) (*SetNameResult, error) {
	res := &SetNameResult{Ok: true}
	self.Storage.SetNameToBlockId(req.Name, req.Id)
	return res, nil
}

func (self *Server) StoreBlock(ctx context.Context, req *StoreRequest) (*Block, error) {
	var bl *storage.StorageBlock
	self.Update(func(tr *hugger.Transaction) {
		bdata := []byte(req.Block.Data)
		st := storage.BlockStatus(req.Block.Status)
		bl = tr.GetStorageBlock(st, bdata, nil)
		k := fs.NewBlockKeyNameBlock(req.Name, bl.Id()).IB()
		tr.IB().Set(k, bl.Id())
	})
	return self.getBlock(bl.Id(), false, true), nil
}

func (self *Server) UpgradeBlockNonWeak(ctx context.Context, bid *BlockId) (*Block, error) {
	b := self.Storage.GetBlockById(bid.Id)
	if b != nil {
		b.SetStatus(storage.BS_NORMAL)
	}
	return self.getBlock(bid.Id, false, true), nil
}
