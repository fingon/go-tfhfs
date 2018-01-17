/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2018 Markus Stenberg
 *
 * Created:       Wed Jan 17 15:22:21 2018 mstenber
 * Last modified: Wed Jan 17 16:18:59 2018 mstenber
 * Edit time:     20 min
 *
 */

package connector_test

import (
	"log"
	"os"
	"testing"

	"github.com/fingon/go-tfhfs/connector"
	"github.com/fingon/go-tfhfs/fs"
	"github.com/fingon/go-tfhfs/mlog"
	"github.com/fingon/go-tfhfs/server"
	"github.com/fingon/go-tfhfs/storage"
	"github.com/fingon/go-tfhfs/storage/factory"
	"github.com/stvp/assert"
)

type system struct {
	be     storage.Backend
	st     *storage.Storage
	fs     *fs.Fs
	server *server.Server
}

func newSystem(rootName string, family, address string) *system {
	mlog.Printf2("connector/connector_test", "newSystem root:%v family:%v address:%v", rootName, family, address)
	be := factory.New("inmemory", "")
	st := storage.Storage{Backend: be}.Init()
	fs := fs.NewFs(st, rootName, 0)
	server := server.Server{Address: address, Family: family, Fs: fs,
		Storage: st}.Init()
	return &system{be, st, fs, server}
}

func (self *system) Close() {
	mlog.Printf2("connector/connector_test", "%v.Close", self)
}

// This is about as far from unit test as you can be; it setups whole
// infrastructure (e.g. in-memory storage -> storage -> fs -> server
// twice + connects them via Connector).
//
// While in principle I dislike system tests that masquerade as unit
// tests, this is highly convenient way to test this stuff (and
// mocking interfaces etc. would be relatively painful in comparison).
func TestConnector(t *testing.T) {
	mlog.Printf2("connector/connector_test", "TestConnector started")
	t.Parallel()
	//dir, _ := ioutil.TempDir("", "connector")
	//defer os.RemoveAll(dir)

	family := "tcp"

	a1 := "127.0.0.1:12345"
	r1 := "s1"

	s1 := newSystem(r1, family, a1)
	u1 := fs.NewFSUser(s1.fs)
	defer s1.Close()

	a2 := "127.0.0.1:12346"
	// a2 := filepath.Join(dir, "s2")
	r2 := "r2"
	s2 := newSystem(r2, family, a2)
	u2 := fs.NewFSUser(s2.fs)
	defer s2.Close()

	mlog.Printf2("connector/connector_test", "! creating file to sync")
	path := "/foo"
	content := "bar"
	f, err := u1.OpenFile(path, uint32(os.O_CREATE|os.O_TRUNC|os.O_WRONLY), 0600)
	if err != nil {
		log.Panic(err)
	}
	f.Write([]byte(content))
	f.Close()

	mlog.Printf2("connector/connector_test", "! synchronizing")
	c := connector.Connector{Left: connector.Connection{Family: family,
		Address:  a1,
		RootName: r1, OtherRootName: r2},
		Right: connector.Connection{Family: family,
			Address:  a2,
			RootName: r2, OtherRootName: r1}}
	err = c.Run()
	if err != nil {
		log.Panic(err)
	}

	mlog.Printf2("connector/connector_test", "! reading synchronized file")
	f, err = u2.OpenFile(path, uint32(os.O_RDONLY), 0)
	if err != nil {
		log.Panic(err)
	}
	b := make([]byte, len(content)+1)
	n, err := f.Read(b)
	if err != nil {
		log.Panic(err)
	}
	b = b[:n]
	assert.Equal(t, string(b), content)
}
