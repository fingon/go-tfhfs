/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2018 Markus Stenberg
 *
 * Created:       Tue Mar 13 10:48:50 2018 mstenber
 * Last modified: Tue Mar 13 11:22:19 2018 mstenber
 * Edit time:     20 min
 *
 */

package tree

import (
	"bytes"
	"fmt"
	"log"
	"os"

	"github.com/fingon/go-tfhfs/mlog"
)

// treePersister provides convenience API for pretend files.
type treePersister interface {
	Close()
	ReadData(location LocationSlice) []byte
	Size() uint64
	WriteData(location LocationSlice, data []byte)
}

type inMemoryFile struct {
	b []byte
}

func (self *inMemoryFile) Close() {
}

func (self *inMemoryFile) ReadData(location LocationSlice) []byte {
	l := uint64(0)
	mlog.Printf2("storage/tree/persist", "p.ReadData")
	for _, v := range location {
		mlog.Printf2("storage/tree/persist", " %v", v)
		l += v.Size
	}
	b := make([]byte, l)
	ofs := uint64(0)
	for _, v := range location {
		copy(b[ofs:ofs+v.Size], self.b[v.Offset:])
		ofs += v.Size
	}
	return b
}

func (self *inMemoryFile) Size() uint64 {
	if self.b == nil {
		return 0
	}
	return uint64(len(self.b))
}

func (self *inMemoryFile) WriteData(location LocationSlice, data []byte) {
	ofs := uint64(0)
	mlog.Printf2("storage/tree/persist", "p.WriteData")
	for _, v := range location {
		mlog.Printf2("storage/tree/persist", " %v", v)
		eofs := v.Offset + v.Size
		if eofs > self.Size() {
			self.b = append(self.b, bytes.Repeat([]byte{0}, int(eofs-self.Size()))...)
		}
		copy(self.b[v.Offset:], data[ofs:ofs+v.Size])
		ofs += v.Size
	}
}

var _ treePersister = &inMemoryFile{}

type systemFile struct {
	f    *os.File
	path string
}

var _ treePersister = &systemFile{}

func (self systemFile) Init(directory string) *systemFile {
	self.path = fmt.Sprintf("%s/db", directory)
	f, err := os.OpenFile(self.path, os.O_RDWR|os.O_CREATE, 0755)
	if err != nil {
		mlog.Panicf("Unable to open %s: %s", self.path, err)
	}
	self.f = f
	return &self
}

func (self *systemFile) Close() {
	self.f.Close()
}

func (self *systemFile) ReadData(location LocationSlice) []byte {
	l := uint64(0)
	for _, v := range location {
		l += v.Size
	}
	b := make([]byte, l)
	ofs := uint64(0)
	for _, v := range location {
		_, err := self.f.Seek(int64(v.Offset), 0)
		if err != nil {
			log.Panic(err)
		}
		_, err = self.f.Read(b[ofs : ofs+v.Size])
		if err != nil {
			log.Panic(err)
		}
		ofs += v.Size
	}
	return b
}

func (self *systemFile) Size() uint64 {
	fi, err := self.f.Stat()
	if err != nil {
		mlog.Panicf("Unable to stat %s: %s", self.path, err)
	}
	return uint64(fi.Size())
}

func (self *systemFile) WriteData(location LocationSlice, data []byte) {
	ofs := uint64(0)
	for _, v := range location {
		_, err := self.f.Seek(int64(v.Offset), 0)
		if err != nil {
			log.Panic(err)
		}
		_, err = self.f.Write(data[ofs : ofs+v.Size])
		if err != nil {
			log.Panic(err)
		}
		ofs += v.Size
	}
}
