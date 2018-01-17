/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2018 Markus Stenberg
 *
 * Created:       Thu Jan 11 08:32:34 2018 mstenber
 * Last modified: Wed Jan 17 16:58:36 2018 mstenber
 * Edit time:     14 min
 *
 */

package storage

import (
	"fmt"
	"log"

	"github.com/fingon/go-tfhfs/mlog"
)

type jobType int

const (
	jobFlush jobType = iota
	jobGetBlockById
	jobGetBlockIdByName
	jobSetNameToBlockId
	jobSetStorageBlockStatus
	jobReferOrStoreBlock            // ReferOrStoreBlock, ReferOrStoreBlock0
	jobUpdateBlockIdRefCount        // ReferBlockId, ReleaseBlockId
	jobUpdateBlockIdStorageRefCount // ReleaseStorageBlockId
	jobStoreBlock                   // StoreBlock, StoreBlock0
	jobQuit
)

func (self jobType) String() string {
	switch self {
	case jobFlush:
		return "jobFlush"
	case jobGetBlockById:
		return "jobGetBlockById"
	case jobGetBlockIdByName:
		return "jobGetBlockIdByName"
	case jobSetStorageBlockStatus:
		return "jobSetStorageBlockStatus"
	case jobSetNameToBlockId:
		return "jobSetNameToBlockId"
	case jobReferOrStoreBlock:
		return "jobReferOrStoreBlock"
	case jobUpdateBlockIdRefCount:
		return "jobUpdateBlockIdRefCount"
	case jobUpdateBlockIdStorageRefCount:
		return "jobUpdateBlockIdStorageRefCount"
	case jobStoreBlock:
		return "jobStoreBlock"
	case jobQuit:
		return "jobQuit"
	default:
		return fmt.Sprintf("%d", int(self))
	}
}

type jobOut struct {
	sb *StorageBlock
	id string
	ok bool
}

type jobIn struct {
	// see job* above
	jobType jobType

	sb *StorageBlock

	// in jobReferOrStoreBlock, jobUpdateBlockIdRefCount, jobStoreBlock
	count int32

	// block id
	id string

	// block name
	name string

	// block data
	data []byte

	status BlockStatus

	out chan *jobOut
}

func (self *Storage) run() {
	for job := range self.jobChannel {
		mlog.Printf2("storage/storagejob", "st.run job %v", job.jobType)
		switch job.jobType {
		case jobQuit:
			job.out <- nil
			return
		case jobFlush:
			self.flush()
			job.out <- nil
		case jobGetBlockById:
			b := self.getBlockById(job.id)
			job.out <- &jobOut{sb: NewStorageBlock(b)}
		case jobGetBlockIdByName:
			job.out <- &jobOut{id: self.getName(job.name).newValue}
		case jobReferOrStoreBlock:
			b := self.getBlockById(job.id)
			if b != nil {
				b.addRefCount(job.count)
				job.out <- &jobOut{sb: NewStorageBlock(b)}
				continue
			}
			mlog.Printf2("storage/storagejob", "fallthrough to storing block")
			fallthrough
		case jobStoreBlock:
			b := &Block{Id: job.id,
				storage: self,
			}
			//nd := make([]byte, len(job.data))
			//mlog.Printf2("storage/storagejob", "allocated size:%d", len(job.data))
			//copy(nd, job.data)
			//b.Data.Set(&nd)
			b.Data.Set(&job.data)
			self.blocks[job.id] = b
			b.Status = job.status
			b.addRefCount(job.count)
			job.out <- &jobOut{sb: NewStorageBlock(b)}
		case jobUpdateBlockIdRefCount:
			b := self.getBlockById(job.id)
			if b == nil {
				log.Panicf("block id %x disappeared", job.id)
			}
			b.addRefCount(job.count)
		case jobUpdateBlockIdStorageRefCount:
			b := self.getBlockById(job.id)
			if b == nil {
				log.Panicf("block id %x disappeared", job.id)
			}
			b.addExternalStorageRefCount(job.count)
			b.addStorageRefCount(job.count)
		case jobSetNameToBlockId:
			self.setNameToBlockId(job.name, job.id)
		case jobSetStorageBlockStatus:
			jo := &jobOut{ok: job.sb.block.setStatus(job.status)}
			job.out <- jo
		default:
			log.Panicf("Unknown job type: %d", job.jobType)
		}
		mlog.Printf2("storage/storagejob", " st.run job done")
	}
}

func (self *Storage) Flush() {
	out := make(chan *jobOut)
	self.jobChannel <- &jobIn{jobType: jobFlush, out: out}
	<-out
}

func (self *Storage) GetBlockById(id string) *StorageBlock {
	out := make(chan *jobOut)
	self.jobChannel <- &jobIn{jobType: jobGetBlockById, out: out,
		id: id,
	}
	jr := <-out
	return jr.sb
}

func (self *Storage) GetBlockIdByName(name string) string {
	out := make(chan *jobOut)
	self.jobChannel <- &jobIn{jobType: jobGetBlockIdByName, out: out,
		name: name,
	}
	jr := <-out
	return jr.id
}

func (self *Storage) storeBlockInternal(jobType jobType, id string, status BlockStatus, data []byte, count int32) *StorageBlock {
	if data == nil {
		mlog.Printf2("storage/storagejob", "no data given")
	}
	out := make(chan *jobOut)
	self.jobChannel <- &jobIn{jobType: jobType, out: out,
		id: id, data: data, count: count, status: status,
	}
	jr := <-out
	return jr.sb
}

func (self *Storage) ReferOrStoreBlock(id string, status BlockStatus, data []byte) *StorageBlock {
	return self.storeBlockInternal(jobReferOrStoreBlock, id, status, data, 1)
}

func (self *Storage) ReferOrStoreBlock0(id string, status BlockStatus, data []byte) *StorageBlock {
	return self.storeBlockInternal(jobReferOrStoreBlock, id, status, data, 0)
}

func (self *Storage) ReferBlockId(id string) {
	self.jobChannel <- &jobIn{jobType: jobUpdateBlockIdRefCount,
		id: id, count: 1,
	}
}

func (self *Storage) ReferStorageBlockId(id string) {
	mlog.Printf2("storage/storagejob", "ReferStorageBlockId %x", id)
	self.jobChannel <- &jobIn{jobType: jobUpdateBlockIdStorageRefCount,
		id: id, count: 1,
	}
}

func (self *Storage) ReleaseBlockId(id string) {
	self.jobChannel <- &jobIn{jobType: jobUpdateBlockIdRefCount,
		id: id, count: -1,
	}
}

func (self *Storage) ReleaseStorageBlockId(id string) {
	mlog.Printf2("storage/storagejob", "ReleaseStorageBlockId %x", id)
	self.jobChannel <- &jobIn{jobType: jobUpdateBlockIdStorageRefCount,
		id: id, count: -1,
	}
}

func (self *Storage) SetNameToBlockId(name, block_id string) {
	self.jobChannel <- &jobIn{jobType: jobSetNameToBlockId,
		id: block_id, name: name,
	}
}

func (self *Storage) StoreBlock(id string, status BlockStatus, data []byte) *StorageBlock {
	return self.storeBlockInternal(jobStoreBlock, id, status, data, 1)
}

func (self *Storage) StoreBlock0(id string, status BlockStatus, data []byte) *StorageBlock {
	return self.storeBlockInternal(jobStoreBlock, id, status, data, 0)
}

func (self *Storage) setStorageBlockStatus(sb *StorageBlock, status BlockStatus) bool {

	out := make(chan *jobOut)
	self.jobChannel <- &jobIn{jobType: jobSetStorageBlockStatus, out: out,
		sb: sb, status: status,
	}
	jr := <-out
	return jr.ok
}
