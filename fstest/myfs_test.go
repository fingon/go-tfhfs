/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2017 Markus Stenberg
 *
 * Created:       Fri Dec 29 15:43:45 2017 mstenber
 * Last modified: Fri Dec 29 15:55:37 2017 mstenber
 * Edit time:     8 min
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
}

func TestFs(t *testing.T) {
	t.Parallel()
	backend := storage.InMemoryBlockBackend{}.Init()
	st := storage.Storage{Backend: backend}.Init()
	fs := fs.NewFs(st, "xxx")
	ProdFs(t, fs)
}
