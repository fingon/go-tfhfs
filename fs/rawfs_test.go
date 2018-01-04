/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2017 Markus Stenberg
 *
 * Created:       Fri Dec 29 15:43:45 2017 mstenber
 * Last modified: Thu Jan  4 14:19:37 2018 mstenber
 * Edit time:     115 min
 *
 */

package fs

import (
	"bytes"
	"log"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/fingon/go-tfhfs/mlog"
	"github.com/fingon/go-tfhfs/storage"
	"github.com/hanwen/go-fuse/fuse"
	"github.com/stvp/assert"
)

func ProdFsFile1(t *testing.T, u *FSUser, tn, wsize, rsize int) {
	mlog.Printf2("fs/rawfs_test", "ProdFsFile1 tn:%v", tn)
	f, err := u.OpenFile("/public/file", uint32(os.O_CREATE|os.O_TRUNC|os.O_WRONLY), 0777)
	assert.Nil(t, err)
	assert.Equal(t, len(u.fs.fh2ifile), 1)

	wd := bytes.Repeat([]byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}, tn/5)

	mlog.Printf2("fs/rawfs_test", " writing file with wsize %d", wsize)
	for i := 0; i < tn; i += wsize {
		wb := wd[i : i+wsize]
		n, err := f.Write(wb)
		assert.Nil(t, err)
		assert.Equal(t, n, wsize)
	}

	fi, err := u.Stat("/public/file")
	since := time.Now().Sub(fi.ModTime())
	assert.True(t, since >= 0, "modtime not in past")
	assert.True(t, since.Minutes() < 1, "modtime too much in past")
	assert.Nil(t, err)
	mlog.Printf2("fs/rawfs_test", " write size %v", fi.Size())

	for j := 0; j < rsize; j++ {
		mlog.Printf2("fs/rawfs_test", " reading file with offset %d rsize %d", j, rsize)
		ofs, err := f.Seek(int64(j), 0)
		assert.Equal(t, int(ofs), j)
		assert.Nil(t, err)
		for i := j; i < tn; i += rsize {
			rb := make([]byte, rsize)
			end := int64(i + rsize)
			ersize := rsize
			if end > fi.Size() {
				end = fi.Size()
				ersize = int(end) - i
			}
			eb := wd[i:end]
			n, err := f.Read(rb)
			rb = rb[:n]
			assert.Nil(t, err)
			assert.Equal(t, n, ersize)
			assert.Equal(t, len(rb), len(eb))
			if !bytes.Equal(rb, eb) {
				log.Panicf("content mismatch - got:%x <> expected:%x", rb, eb)
			}
		}
	}

	f.Close()
	assert.Equal(t, len(u.fs.fh2ifile), 0)
}

func ProdFsFile(t *testing.T, u *FSUser) {
	mlog.Printf2("fs/rawfs_test", "ProdFsFile")
	_, err := u.OpenFile("/public/file", uint32(os.O_RDONLY), 0777)
	assert.True(t, err != nil)
	assert.Equal(t, len(u.fs.fh2ifile), 0, "failed open should not add files")

	// Small writes for small blocks
	tn := 3*embeddedSize + 5
	assert.True(t, tn%3 != 0)
	assert.True(t, tn%7 != 0)
	ProdFsFile1(t, u, tn, 1, 1)
	ProdFsFile1(t, u, tn, 7, 3)
	ProdFsFile1(t, u, tn, 3, 7)

	// Then bit larger writes for larger things
	tn = 3 * dataExtentSize
	ProdFsFile1(t, u, tn, 41, 13)
	ProdFsFile1(t, u, tn, 257, 41)
}

// ProdFs exercises filesystem, trying to go for as high coverage as
// possible.
//
// NOTE: The filesystem HAS to be empty to start with.
func ProdFs(t *testing.T, fs *Fs) {
	root := NewFSUser(fs)
	arr, err := root.ReadDir("/")
	assert.Nil(t, err)
	assert.Equal(t, len(arr), 0)

	var wg sync.WaitGroup

	wg.Add(3)
	go func() {
		defer wg.Done()
		err = root.Mkdir("/public", 0777)
		assert.Nil(t, err)
	}()

	go func() {
		defer wg.Done()
		err = root.Mkdir("/private", 0007)
		assert.Nil(t, err)
	}()

	go func() {
		defer wg.Done()
		err = root.Mkdir("/nobody", 0)
		assert.Nil(t, err)

	}()
	mlog.Printf("ProdFs wait 1")
	wg.Wait()

	arr, err = root.ReadDir("/")
	assert.Nil(t, err)
	assert.Equal(t, len(arr), 3)
	// fnvhash order :p
	assert.Equal(t, arr[0].Name(), "private")
	assert.Equal(t, arr[1].Name(), "nobody")
	assert.Equal(t, arr[2].Name(), "public")

	err = root.Mkdir("/goat", 0777)
	assert.Nil(t, err)

	fi, err := root.Stat("/")
	assert.Nil(t, err)
	mlog.Printf2("fs/rawfs_test", "got %o", int(fi.Mode()))
	assert.True(t, fi.IsDir())

	fi, err = root.Stat("/goat")
	assert.Nil(t, err)
	assert.True(t, fi.IsDir())
	assert.Equal(t, fi.Name(), "goat")

	err = root.Remove("/goat")
	assert.Nil(t, err)

	fi, err = root.Stat("/goat")
	assert.True(t, err != nil)

	err = root.Remove("/asdf")
	assert.True(t, err != nil)

	u1 := NewFSUser(fs)
	u1.Uid = 13
	u1.Gid = 7

	u2 := NewFSUser(fs)
	u2.Uid = 42
	u2.Gid = 7

	u3 := NewFSUser(fs)
	u3.Uid = 123
	u3.Gid = 123

	u1.Mkdir("/u1", 0777)
	wg.Add(3)
	go func() {
		defer wg.Done()
		u1.Mkdir("/u1/u", 0700)
	}()
	go func() {
		defer wg.Done()
		u1.Mkdir("/u1/g", 0070)
	}()
	go func() {
		defer wg.Done()
		u1.Mkdir("/u1/o", 0007)
	}()
	mlog.Printf("ProdFs wait 2")
	wg.Wait()

	wg.Add(9)
	go func() {
		defer wg.Done()
		_, err := u2.Stat("/u1/u/.")
		assert.True(t, err != nil)
	}()

	go func() {
		defer wg.Done()
		_, err := u1.Stat("/u1/u/.")
		assert.Nil(t, err)
	}()

	go func() {
		defer wg.Done()
		_, err := u3.Stat("/u1/g/.")
		assert.True(t, err != nil)
	}()

	go func() {
		defer wg.Done()
		_, err := u2.Stat("/u1/g/.")
		assert.Nil(t, err)
	}()

	go func() {
		defer wg.Done()
		_, err := u2.Stat("/u1/o/.")
		assert.Nil(t, err)
	}()

	go func() {
		defer wg.Done()
		_, err = u3.Stat("/u1/o/.")
		assert.Nil(t, err)

	}()

	go func() {
		defer wg.Done()
		var sfo fuse.StatfsOut
		code := fs.Ops.StatFs(&root.InHeader, &sfo)
		assert.True(t, code.Ok())
	}()

	go func() {
		defer wg.Done()
		// Initially no xattr - list should be empty

		l, err := root.ListXAttr("/public")
		assert.Nil(t, err)
		assert.Equal(t, len(l), 0)
	}()

	go func() {
		defer wg.Done()
		_, err = root.GetXAttr("/public", "foo")
		assert.True(t, err != nil)
	}()

	mlog.Printf("ProdFs wait 3")
	wg.Wait()

	// Set xattr

	err = root.SetXAttr("/public", "foo", []byte("bar"))
	assert.Nil(t, err)

	wg.Add(2)
	// Xattr should be accessible
	go func() {
		defer wg.Done()
		b, err := root.GetXAttr("/public", "foo")
		assert.Nil(t, err)
		assert.Equal(t, string(b), "bar")

	}()

	go func() {
		defer wg.Done()
		l, err := root.ListXAttr("/public")
		assert.Nil(t, err)
		assert.Equal(t, len(l), 1)
		assert.Equal(t, string(l[0]), "foo")
	}()

	mlog.Printf("ProdFs wait 4")
	wg.Wait()

	// Remove xattr - it should be gone

	err = root.RemoveXAttr("/public", "foo")
	assert.Nil(t, err)

	wg.Add(2)
	go func() {
		defer wg.Done()
		l, err := root.ListXAttr("/public")
		assert.Nil(t, err)
		assert.Equal(t, len(l), 0)
	}()

	go func() {
		defer wg.Done()
		_, err := root.GetXAttr("/public", "foo")
		assert.True(t, err != nil)
	}()

	mlog.Printf("ProdFs wait 5")
	wg.Wait()

	ProdFsFile(t, root)
}

type DummyGenerator struct {
	index uint64
	incr  int64
}

func (self *DummyGenerator) CreateInodeNumber() uint64 {
	self.index = uint64(int64(self.index) + self.incr)
	return self.index
}

func TestFs(t *testing.T) {
	check := func(t *testing.T, fs *Fs) {
		mlog.Printf2("fs/rawfs_test", "Root: %s = %x", fs.rootName, fs.treeRootBlockId)
		fs.treeRoot.Get().PrintToMLogAll()
		root := NewFSUser(fs)
		_, err := root.Stat("/public")
		assert.Nil(t, err)

	}
	add := func(s string, gen inodeNumberGenerator) {
		t.Run(s,
			func(t *testing.T) {
				t.Parallel()
				mlog.Printf2("fs/rawfs_test", "starting")
				rootName := "toor"
				backend := storage.InMemoryBlockBackend{}.Init()
				st := storage.Storage{Backend: backend}.Init()
				fs := NewFs(st, rootName)
				if gen != nil {
					fs.generator = gen

				}
				ProdFs(t, fs)
				fs.Flush()

				mlog.Printf2("fs/rawfs_test", "checking current state valid")
				check(t, fs)

				mlog.Printf2("fs/rawfs_test", "omstart from storage")
				fs2 := NewFs(st, rootName)
				check(t, fs2)
			})
	}
	add("seq+1", &DummyGenerator{index: 2, incr: 1})
	add("seq-1", &DummyGenerator{index: 12345, incr: -1})
	add("random", nil)
}
