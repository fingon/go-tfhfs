/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2017 Markus Stenberg
 *
 * Created:       Fri Dec 29 15:43:45 2017 mstenber
 * Last modified: Wed Jan 17 13:19:56 2018 mstenber
 * Edit time:     212 min
 *
 */

package fs

import (
	"bytes"
	"fmt"
	"log"
	"math/rand"
	"os"
	"testing"
	"time"

	"github.com/fingon/go-tfhfs/mlog"
	"github.com/fingon/go-tfhfs/storage"
	"github.com/fingon/go-tfhfs/storage/factory"
	"github.com/fingon/go-tfhfs/util"
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
	written := 0
	for i := 0; i < tn; i += wsize {
		wb := wd[i : i+wsize]
		n, err := f.Write(wb)
		written += n
		assert.Nil(t, err)
		assert.Equal(t, n, wsize)
	}

	fi, err := u.Stat("/public/file")
	if err != nil {
		log.Panic("unable to stat the just created file: ", err)
	}
	assert.Equal(t, int(fi.Size()), written)
	since := time.Now().Sub(fi.ModTime())
	assert.True(t, since >= 0, "modtime not in past")
	assert.True(t, since.Minutes() < 10, "modtime too much in past")
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
				log.Panicf("content mismatch @%d - got:%x <> expected:%x", i, rb, eb)
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

	// Embedded-only tests
	tn := embeddedSize / 2
	ProdFsFile1(t, u, tn, 1, 1)
	ProdFsFile1(t, u, tn, 7, 3)
	ProdFsFile1(t, u, tn, 3, 7)

	// Small writes for small blocks
	tn = 3*embeddedSize + 5
	assert.True(t, tn%3 != 0)
	assert.True(t, tn%7 != 0)
	// ProdFsFile1(t, u, tn, 1, 1)
	ProdFsFile1(t, u, tn, 7, 3)
	ProdFsFile1(t, u, tn, 3, 7)

	// Then bit larger writes for larger things
	tn = 3 * dataExtentSize
	// ProdFsFile1(t, u, tn, 41, 13)
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

	var wg util.SimpleWaitGroup

	wg.Go(func() {
		err := root.Mkdir("/public", 0777)
		assert.Nil(t, err)
	})

	wg.Go(func() {
		err := root.Mkdir("/private", 0007)
		assert.Nil(t, err)
	})

	wg.Go(func() {
		err := root.Mkdir("/nobody", 0)
		assert.Nil(t, err)

	})
	mlog.Printf2("fs/rawfs_test", "ProdFs wait 1")
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

	f, err := root.OpenFile("/private/rootfile", uint32(os.O_CREATE|os.O_TRUNC|os.O_WRONLY), 0666)
	assert.Nil(t, err)
	_, err = f.Write([]byte("data"))
	assert.Nil(t, err)
	f.Close()

	err = root.Link("/private/rootfile", "/public/rootfilelink")
	assert.Nil(t, err)

	fi, err = root.Stat("/public/rootfilelink")
	assert.Nil(t, err)
	assert.False(t, fi.IsDir())

	err = root.Symlink("rootfilelink", "/public/rootfilesymlink")
	assert.Nil(t, err)

	fi, err = root.Stat("/public/rootfilesymlink")
	assert.Nil(t, err)
	assert.False(t, fi.IsDir())

	s, err := root.Readlink("/public/rootfilesymlink")
	assert.Equal(t, s, "rootfilelink")

	err = root.Rename("/public/rootfilesymlink", "/rootfilesymlink2")
	assert.Nil(t, err)

	s, err = root.Readlink("/rootfilesymlink2")
	assert.Equal(t, s, "rootfilelink")

	err = root.Chown("/private/rootfile", 13, 7)
	assert.Nil(t, err)

	err = root.Chtimes("/private/rootfile", time.Now(), time.Now())
	assert.Nil(t, err)

	err = root.Chmod("/private/rootfile", os.FileMode(0))
	assert.Nil(t, err)

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
	wg.Go(func() {
		u1.Mkdir("/u1/u", 0700)
	})
	wg.Go(func() {
		u1.Mkdir("/u1/g", 0070)
	})
	wg.Go(func() {
		u1.Mkdir("/u1/o", 0007)
	})
	mlog.Printf2("fs/rawfs_test", "ProdFs wait 2")
	wg.Wait()

	wg.Go(func() {
		_, err := u2.Stat("/u1/u/.")
		assert.True(t, err != nil)
	})

	wg.Go(func() {
		_, err := u1.Stat("/u1/u/.")
		assert.Nil(t, err)
	})

	wg.Go(func() {
		_, err := u3.Stat("/u1/g/.")
		assert.True(t, err != nil)
	})

	wg.Go(func() {
		_, err := u2.Stat("/u1/g/.")
		assert.Nil(t, err)
	})

	wg.Go(func() {
		_, err := u2.Stat("/u1/o/.")
		assert.True(t, err != nil)
	})

	wg.Go(func() {
		_, err := u3.Stat("/u1/o/.")
		assert.Nil(t, err)

	})

	wg.Go(func() {
		var sfo fuse.StatfsOut
		code := fs.Ops.StatFs(&root.InHeader, &sfo)
		assert.True(t, code.Ok())
	})

	wg.Go(func() {
		// Initially no xattr - list should be empty

		l, err := root.ListXAttr("/public")
		assert.Nil(t, err)
		assert.Equal(t, len(l), 0)
	})

	wg.Go(func() {
		_, err = root.GetXAttr("/public", "foo")
		assert.True(t, err != nil)
	})

	mlog.Printf2("fs/rawfs_test", "ProdFs wait 3")
	wg.Wait()

	// Set xattr

	err = root.SetXAttr("/public", "foo", []byte("bar"))
	assert.Nil(t, err)

	// Xattr should be accessible
	wg.Go(func() {
		b, err := root.GetXAttr("/public", "foo")
		assert.Nil(t, err)
		assert.Equal(t, string(b), "bar")

	})

	wg.Go(func() {
		l, err := root.ListXAttr("/public")
		assert.Nil(t, err)
		assert.Equal(t, len(l), 1,
			"(post-SetXAttr) wrong # of xattrs: 1 != ", len(l))
		assert.Equal(t, string(l[0]), "foo")
	})

	mlog.Printf2("fs/rawfs_test", "ProdFs wait 4")
	wg.Wait()

	// Remove xattr - it should be gone

	err = root.RemoveXAttr("/public", "foo")
	assert.Nil(t, err)

	wg.Go(func() {
		l, err := root.ListXAttr("/public")
		assert.Nil(t, err)
		assert.Equal(t, len(l), 0)
	})

	wg.Go(func() {
		_, err := root.GetXAttr("/public", "foo")
		assert.True(t, err != nil)
	})

	mlog.Printf2("fs/rawfs_test", "ProdFs wait 5")
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
		root := NewFSUser(fs)
		_, err := root.Stat("/public")
		assert.Nil(t, err)

	}
	add := func(s string, gen inodeNumberGenerator) {
		t.Run(s,
			func(t *testing.T) {
				t.Parallel()
				mlog.Printf2("fs/rawfs_test", "starting")
				RootName := "toor"
				backend := factory.New("inmemory", "")
				st := storage.Storage{Backend: backend}.Init()
				fs := NewFs(st, RootName, 0)
				defer fs.Close()
				if gen != nil {
					fs.generator = gen

				}
				ProdFs(t, fs)
				fs.Flush()

				mlog.Printf2("fs/rawfs_test", "checking current state valid")
				check(t, fs)

				mlog.Printf2("fs/rawfs_test", "omstart from storage")
				fs2 := NewFs(st, RootName, 0)
				check(t, fs2)
				fs2.storage.Backend = nil
			})
	}
	add("seq+1", &DummyGenerator{index: 2, incr: 1})
	add("seq-1", &DummyGenerator{index: 12345, incr: -1})
	add("random", nil)
}

func TestFsParallel(t *testing.T) {
	t.Parallel()

	// nu: # of users
	// iter: # of iterations
	// n: data size (up to *iter)
	// nw: write size (up to *iter)
	add := func(nu, iter, n, nw int) {
		cost := nu * iter * iter * (nw + n) / 1000 / 1000
		t.Run(fmt.Sprintf("%d u/%d iter/%d n/%d nw=%d cost", nu, iter, n, nw, cost),
			func(t *testing.T) {
				t.Parallel()

				RootName := "toor"
				backend := factory.New("inmemory", "")
				st := storage.Storage{Backend: backend}.Init()
				fs := NewFs(st, RootName, 0)
				defer fs.Close()

				randomReaderWriter := func(path string, u *FSUser) {
					f, err := u.OpenFile(path, uint32(os.O_CREATE|os.O_TRUNC|os.O_WRONLY), 0777)
					if err != nil {
						log.Panic("OpenFile:", err)
					}
					content := []byte{}
					for i := 0; i < iter; i++ {
						awlen := rand.Int() % ((i + 1) * nw)
						ofs := rand.Int() % ((i + 1) * n)
						s := fmt.Sprintf("%d", i)
						b := bytes.Repeat([]byte(s), 1+awlen/len(s))
						eofs := ofs + len(b)
						nlen := eofs
						if nlen < len(content) {
							nlen = len(content)
						}
						ncontent := make([]byte, nlen)
						mlog.Printf2("fs/rawfs_test", "%v Writing @%v %d bytes of %v", path, ofs, len(b), s)
						if ofs <= len(content) {
							copy(ncontent, content[:ofs])
						} else {
							copy(ncontent, content)
						}
						copy(ncontent[ofs:], b)
						if len(content) > eofs {
							copy(ncontent[eofs:], content[eofs:])
						}
						mlog.Printf2("fs/rawfs_test", " eofs:%v", eofs)
						_, err := f.Seek(int64(ofs), 0)
						if err != nil {
							log.Panic("seek:", err)
						}

						w, err := f.Write(b)
						if err != nil {
							log.Panic("write:", err)
						}
						assert.Equal(t, w, len(b))

						_, err = f.Seek(0, 0)
						if err != nil {
							log.Panic("seek2:", err)
						}

						mlog.Printf2("fs/rawfs_test", "%v Reading %d bytes, ensuring they are same", path, nlen)

						rcontent := make([]byte, nlen)
						r, err := f.Read(rcontent)
						if err != nil {
							log.Panic("read:", err)
						}
						assert.Equal(t, r, nlen)

						// mlog.Printf2("fs/rawfs_test", "exp: %x", ncontent)
						// mlog.Printf2("fs/rawfs_test", "got: %x", rcontent)
						assert.Equal(t, ncontent, rcontent, "exp<>read mismatch")
						if !bytes.Equal(ncontent, rcontent) {
							log.Panic(".. snif")
						}
						assert.True(t, len(content) <= len(ncontent))
						content = ncontent

						if nu == 1 {
							mlog.Printf2("fs/rawfs_test", "starting fs flush")
							fs.Flush()
							mlog.Printf2("fs/rawfs_test", "fs flush done")
							assert.Equal(t, fs.storage.TransientCount(), 0, "transients left")

						}
					}
				}

				var wg util.SimpleWaitGroup
				for i := 0; i < nu; i++ {
					u := NewFSUser(fs)
					u.Uid = uint32(42 + i)
					u.Gid = uint32(7 + i)

					wg.Go(func() {
						randomReaderWriter(fmt.Sprintf("/u%d", u.Uid), u)
					})
				}
				wg.Wait()
				fs.Flush()
				assert.Equal(t, fs.storage.TransientCount(), 0, "transients left")

			})
	}
	add(1, 500, 123, 1)
	add(3, 100, 123, 1)
	add(7, 50, 1234, 100)
	add(13, 10, 12345, 12345)

}
