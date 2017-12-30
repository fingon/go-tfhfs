/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2017 Markus Stenberg
 *
 * Created:       Fri Dec 29 15:43:45 2017 mstenber
 * Last modified: Sat Dec 30 15:29:57 2017 mstenber
 * Edit time:     26 min
 *
 */

package fs

import (
	"testing"

	"github.com/fingon/go-tfhfs/mlog"
	"github.com/fingon/go-tfhfs/storage"
	"github.com/hanwen/go-fuse/fuse"
	"github.com/stvp/assert"
)

// ProdFs exercises filesystem, trying to go for as high coverage as
// possible.
//
// NOTE: The filesystem HAS to be empty to start with.
func ProdFs(t *testing.T, fs *Fs) {
	root := NewFSUser(fs)
	arr, err := root.ReadDir("/")
	assert.Nil(t, err)
	assert.Equal(t, len(arr), 0)

	err = root.Mkdir("/public", 0777)
	assert.Nil(t, err)

	err = root.Mkdir("/private", 0007)
	assert.Nil(t, err)

	err = root.Mkdir("/nobody", 0)
	assert.Nil(t, err)

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

	err = u1.Mkdir("/u1", 0777)
	err = u1.Mkdir("/u1/u", 0700)
	err = u1.Mkdir("/u1/g", 0070)
	err = u1.Mkdir("/u1/o", 0007)

	fi, err = u2.Stat("/u1/u/.")
	assert.True(t, err != nil)

	fi, err = u1.Stat("/u1/u/.")
	assert.Nil(t, err)

	fi, err = u3.Stat("/u1/g/.")
	assert.True(t, err != nil)

	fi, err = u2.Stat("/u1/g/.")
	assert.Nil(t, err)

	fi, err = u2.Stat("/u1/o/.")
	assert.Nil(t, err)

	fi, err = u3.Stat("/u1/o/.")
	assert.Nil(t, err)

	var sfo fuse.StatfsOut
	code := fs.StatFs(&root.InHeader, &sfo)
	assert.True(t, code.Ok())
}

func TestFs(t *testing.T) {
	t.Parallel()
	backend := storage.InMemoryBlockBackend{}.Init()
	st := storage.Storage{Backend: backend}.Init()
	fs := NewFs(st, "xxx")
	ProdFs(t, fs)
}
