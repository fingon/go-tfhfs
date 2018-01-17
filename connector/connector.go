/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2018 Markus Stenberg
 *
 * Created:       Wed Jan 17 14:19:35 2018 mstenber
 * Last modified: Wed Jan 17 16:24:24 2018 mstenber
 * Edit time:     58 min
 *
 */

package connector

import (
	"context"

	"github.com/fingon/go-tfhfs/mlog"
	"github.com/fingon/go-tfhfs/pb"
	"github.com/fingon/go-tfhfs/storage"
	"github.com/fingon/go-tfhfs/util"

	"google.golang.org/grpc"
)

type Connection struct {
	Family, Address, RootName, OtherRootName string
}

// Connector glues together two tfhfs servers ('left' and 'right').
//
// The are both connected to, and then LeftName on left server is
// synchronized with RightName on right server and vice versa. This is
// repeated every SyncInterval if there is need.
type Connector struct {
	Left, Right  Connection
	SyncInterval int
}

func (self *Connector) Run() error {
	mlog.Printf2("connector/connector", "%v.Run", self)
	var wg util.SimpleWaitGroup
	var err1, err2 error
	wg.Go(func() {
		err1 = self.Sync(&self.Left, &self.Right)
	})
	wg.Go(func() {
		err2 = self.Sync(&self.Right, &self.Left)
	})
	wg.Wait()
	if err1 != nil {
		return err1
	}
	if err2 != nil {
		return err2
	}
	return nil
}

func (self *Connector) getClient(c *Connection) (pb.FsClient, error) {
	mlog.Printf("getClient %v", c.Address)
	conn, err := grpc.Dial(c.Address, grpc.WithInsecure())
	//, grpc.WithDialer(func(string, time.Duration) (net.Conn, error) {
	//return net.Dial(c.Family, c.Address))
	if err != nil {
		mlog.Printf2("connector/connector", " unable to connect src: %s", err)
		return nil, err
	}
	defer conn.Close()
	return pb.NewFsClient(conn), nil

}

func (self *Connector) Sync(from *Connection, to *Connection) (err error) {
	mlog.Printf2("connector/connector", "Sync %v => %v", from, to)
	fclient, err := self.getClient(from)
	if err != nil {
		return
	}
	tclient, err := self.getClient(to)
	if err != nil {
		return
	}

	bg := context.Background()

	fid, err := fclient.GetBlockIdByName(bg, &pb.BlockName{Name: from.RootName})
	if err != nil {
		mlog.Printf2("connector/connector", " unable to get root %s from src: %s", from.RootName, err)
		return
	}

	tid, err := tclient.GetBlockIdByName(bg, &pb.BlockName{Name: to.OtherRootName})
	if err != nil {
		mlog.Printf2("connector/connector", " unable to get root %s from dst", to.OtherRootName)
		return
	}

	// Nothing to be done
	if fid != tid {
		err = self.copyBlockTo(fclient, tclient, fid.Id, to.OtherRootName)
		if err != nil {
			return
		}

		r, err := tclient.SetNameToBlockId(bg, &pb.SetNameRequest{Name: to.OtherRootName, Id: fid.Id})
		if err != nil {
			return err
		}
		if !r.Ok {
			return nil
		}
	}
	r2, err := tclient.MergeBlockNameTo(bg, &pb.MergeRequest{FromName: to.OtherRootName, ToName: to.RootName})
	if err != nil {
		return
	}
	if !r2.Ok {
		return
	}
	mlog.Printf2("connector/connector", " VICTORY!")
	return
}

func (self *Connector) copyBlockTo(fclient, tclient pb.FsClient, bid, inName string) (err error) {
	mlog.Printf2("connector/connector", "copyBlockTo %x @%x", bid, inName)
	bg := context.Background()

	// Cheap part first - check if it is there already
	b, err := tclient.GetBlockById(bg, &pb.GetBlockRequest{Id: bid, WantMissing: true})
	if err != nil {
		return
	}
	if b.Id == "" {
		fb, err2 := fclient.GetBlockById(bg, &pb.GetBlockRequest{Id: bid, WantData: true})
		if err2 != nil {
			return err2
		}

		b, err = tclient.StoreBlock(bg, &pb.StoreRequest{Name: inName, Block: &pb.Block{Id: bid, Data: fb.Data, Status: int32(storage.BS_WEAK)}})
		if err != nil {
			return
		}

	}

	return self.upgradeBlock(fclient, tclient, bid, inName, b)
}

func (self *Connector) upgradeBlock(fclient, tclient pb.FsClient, bid, inName string, b *pb.Block) (err error) {
	bg := context.Background()
	for {
		if b.MissingIds != nil {
			var wg util.SimpleWaitGroup
			for _, mbid := range b.MissingIds {
				mbid := mbid
				wg.Go(func() {
					err2 := self.copyBlockTo(fclient, tclient, mbid, inName)
					if err2 != nil {
						err = err2
					}
				})
			}
			wg.Wait()

			if err != nil {
				return
			}
		}

		b, err = tclient.UpgradeBlockNonWeak(bg, &pb.BlockId{Id: bid})
		if b.MissingIds == nil || len(b.MissingIds) == 0 {
			return
		}

	}
}
