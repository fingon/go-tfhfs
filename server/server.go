/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2018 Markus Stenberg
 *
 * Created:       Tue Jan 16 14:38:35 2018 mstenber
 * Last modified: Tue Jan 16 19:50:15 2018 mstenber
 * Edit time:     79 min
 *
 */

package server

import (
	"context"
	"log"
	"net"

	"github.com/fingon/go-tfhfs/fs"
	"github.com/fingon/go-tfhfs/ibtree"
	. "github.com/fingon/go-tfhfs/pb"
	"github.com/fingon/go-tfhfs/storage"
	"github.com/fingon/go-tfhfs/util"
	"google.golang.org/grpc"
)

const rootName = "sync"

type Server struct {
	Family, Address string
	Fs              *fs.Fs
	Storage         *storage.Storage
	grpcServer      *grpc.Server

	// lock controls access to stuff below here
	lock   util.MutexLocked
	tree   *ibtree.IBTree
	root   *ibtree.IBNode
	blocks map[string]*storage.StorageBlock
}

func (self *Server) Init() *Server {
	lis, err := net.Listen(self.Family, self.Address)
	if err != nil {
		log.Panic(err)
	}
	grpcServer := grpc.NewServer()
	RegisterFsServer(grpcServer, self)
	grpcServer.Serve(lis)
	self.grpcServer = grpcServer
	self.tree = ibtree.IBTree{NodeMaximumSize: 4096}.Init(self)
	bid := self.Storage.GetBlockIdByName(rootName)
	if bid != "" {
		self.root = self.tree.LoadRoot(ibtree.BlockId(bid))
		if self.root == nil {
			log.Panic("Loading of root block %x failed", bid)
		}
	} else {
		self.root = self.tree.NewRoot()
	}
	return self
}

func (self *Server) LoadNode(id ibtree.BlockId) *ibtree.IBNodeData {
	return self.Fs.LoadNode(id)
}

func (self *Server) SaveNode(nd *ibtree.IBNodeData) ibtree.BlockId {
	self.lock.AssertLocked()
	b := fs.IBNodeDataToBytes(nd)
	bl := self.Storage.ReferOrStoreBlockBytes0(b)
	if self.blocks == nil {
		self.blocks = make(map[string]*storage.StorageBlock)
	}
	self.blocks[bl.Id()] = bl
	return ibtree.BlockId(bl.Id())
}

func (self *Server) Close() {
	self.grpcServer.Stop()
}

func (self *Server) commit(t *ibtree.IBTransaction) {
	root, bid := t.Commit()
	if root == self.root {
		return
	}

	self.Storage.SetNameToBlockId(rootName, string(bid))

	for _, b := range self.blocks {
		b.Close()
	}
	self.blocks = nil
}

func (self *Server) ClearBlocksInName(ctx context.Context, n *BlockName) (*ClearResult, error) {
	defer self.lock.Locked()()
	t := ibtree.NewTransaction(self.root)
	k1 := fs.NewBlockKeyNameBlock(n.Name, "")
	k2 := fs.NewBlockKeyNameEnd(n.Name)
	t.DeleteRange(ibtree.IBKey(k1), ibtree.IBKey(k2))
	self.commit(t)
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
	if wantMissing && b.Status() == storage.BlockStatus_WEAK {
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

func (self *Server) MergeBlockNameTo(context.Context, *MergeRequest) (*MergeResult, error) {
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
	bdata := []byte(req.Block.Data)
	if req.Block.Id != "" {
		bl = self.Storage.ReferOrStoreBlock0(req.Block.Id, bdata)
	} else {
		bl = self.Storage.ReferOrStoreBlockBytes0(bdata)
	}
	defer self.lock.Locked()()
	self.blocks[bl.Id()] = bl
	t := ibtree.NewTransaction(self.root)
	k := fs.NewBlockKeyNameBlock(req.Name, bl.Id())
	t.Set(ibtree.IBKey(k), bl.Id())
	self.commit(t)
	return self.getBlock(bl.Id(), false, true), nil
}

func (self *Server) UpgradeBlockNonWeak(ctx context.Context, bid *BlockId) (*Block, error) {
	b := self.Storage.GetBlockById(bid.Id)
	if b != nil {
		b.SetStatus(storage.BlockStatus_NORMAL)
	}
	return self.getBlock(bid.Id, false, true), nil
}
