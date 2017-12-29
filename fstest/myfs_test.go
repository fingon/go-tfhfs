/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2017 Markus Stenberg
 *
 * Created:       Fri Dec 29 15:43:45 2017 mstenber
 * Last modified: Fri Dec 29 17:48:01 2017 mstenber
 * Edit time:     15 min
 *
 */

package fstest

import (
	"testing"

	"github.com/fingon/go-tfhfs/fs"
	"github.com/fingon/go-tfhfs/storage"
	"github.com/stvp/assert"
)

// ProdFs exercises filesystem, trying to go for as high coverage as
// possible.
//
// NOTE: The filesystem HAS to be empty to start with.
func ProdFs(t *testing.T, fs *fs.Fs) {
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
}

func TestFs(t *testing.T) {
	t.Parallel()
	backend := storage.InMemoryBlockBackend{}.Init()
	st := storage.Storage{Backend: backend}.Init()
	fs := fs.NewFs(st, "xxx")
	ProdFs(t, fs)
}
