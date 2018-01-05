/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2018 Markus Stenberg
 *
 * Created:       Sat Jan  6 00:24:46 2018 mstenber
 * Last modified: Sat Jan  6 01:08:08 2018 mstenber
 * Edit time:     42 min
 *
 */

package storage

import (
	"runtime"

	"github.com/fingon/go-tfhfs/util"
)

/* Core of storage is in single goroutine Storage. However,
/* e.g. writing can be much, much faster if we do some fanout based on
/* number of processors available. */

type foJobType int

const (
	foJobQuit foJobType = iota
	foJobStore
	foJobUpdate
	foJobDelete
)

type foJobResult struct {
}

type foJob struct {
	t   foJobType
	bl  *Block
	out chan *foJobResult
}

type fanoutBackend struct {
	proxyBackend

	lock           util.MutexLocked
	spareWorkers   chan *fanoutWorker
	workingWorkers map[string]*fanoutWorker
}

func (self fanoutBackend) Init(backend Backend) *fanoutBackend {
	self.backend = backend
	n := runtime.NumCPU()
	self.spareWorkers = make(chan *fanoutWorker, n)
	for i := 0; i < n; i++ {
		self.spareWorkers <- fanoutWorker{}.Init(self.backend)
	}
	self.workingWorkers = make(map[string]*fanoutWorker)
	return &self
}

func (self *fanoutBackend) Close() {
	defer self.lock.Locked()()

	// Kill all workers gracefully
	for {
		select {
		case w := <-self.spareWorkers:
			w.Close()
		default:
			break
		}
	}
	for _, w := range self.workingWorkers {
		w.Close()
	}

	self.backend.Close()
}

func (self *fanoutBackend) getWorkerForBlock(bl *Block) *fanoutWorker {
	w, ok := self.workingWorkers[bl.Id]
	if ok {
		return w
	}
	w = (<-self.spareWorkers)
	self.workingWorkers[bl.Id] = w
	return w
}

func (self *fanoutBackend) blockJob(t foJobType, bl *Block) {
	defer self.lock.Locked()()
	w := self.getWorkerForBlock(bl)
	out := make(chan *foJobResult)
	w.c <- &foJob{t: t, bl: bl, out: out}
	w.queued++
	go func() {
		<-out
		defer self.lock.Locked()()
		w.queued--
		if w.queued == 0 {
			// not busy with the particular block id anymore
			delete(self.workingWorkers, bl.Id)
			self.spareWorkers <- w
		}
	}()
}

func (self *fanoutBackend) StoreBlock(bl *Block) {
	self.blockJob(foJobStore, bl)
}

func (self *fanoutBackend) DeleteBlock(bl *Block) {
	self.blockJob(foJobDelete, bl)
}

func (self *fanoutBackend) UpdateBlock(bl *Block) int {
	self.blockJob(foJobUpdate, bl)
	return 1
}

type fanoutWorker struct {
	c       chan *foJob
	backend Backend
	queued  int
}

func (self fanoutWorker) Init(backend Backend) *fanoutWorker {
	self.c = make(chan *foJob, 10)
	self.backend = backend
	go func() {
		for {
			for job := range self.c {
				var r *foJobResult
				switch job.t {
				case foJobStore:
					self.backend.StoreBlock(job.bl)
				case foJobDelete:
					self.backend.DeleteBlock(job.bl)
				case foJobUpdate:
					self.backend.UpdateBlock(job.bl)
				}
				job.out <- r
				if job.t == foJobQuit {
					return
				}
			}

		}
	}()
	return &self
}

func (self *fanoutWorker) Close() {
	out := make(chan *foJobResult)
	self.c <- &foJob{t: foJobQuit, out: out}
	<-out
}
