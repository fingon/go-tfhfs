/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2018 Markus Stenberg
 *
 * Created:       Wed Feb 21 17:11:02 2018 mstenber
 * Last modified: Thu Feb 22 11:16:54 2018 mstenber
 * Edit time:     21 min
 *
 */

package tree

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/fingon/go-tfhfs/storage"
	"github.com/stvp/assert"
)

func TestTree(t *testing.T) {
	t.Parallel()

	dir, _ := ioutil.TempDir("", "tree")
	defer os.RemoveAll(dir)

	config := storage.BackendConfiguration{Directory: dir}

	be := NewTreeBackend()
	tbe := be.(*treeBackend)
	be.Init(config)

	assert.Equal(t, tbe.BytesUsed, uint64(0))
	assert.Equal(t, tbe.BytesTotal, uint64(0))
	be.Flush() // empty tree = still no data needed
	assert.Equal(t, tbe.BytesUsed, uint64(0))
	assert.Equal(t, tbe.BytesTotal, uint64(0))

	// Add dummy block

	b := storage.Block{Id: "foo"}
	//bd := bytes.Repeat([]byte("bar"), 1234)
	bd := []byte("bar")
	b.Data.Set(&bd)
	be.StoreBlock(&b)
	be.Flush()
	assert.Equal(t, tbe.BytesTotal, uint64(superBlockSize+2*blockSize), "total")
	assert.Equal(t, tbe.BytesUsed, uint64(superBlockSize+2*blockSize), "used")

	bd2 := be.GetBlockData(&b)
	assert.Equal(t, bd, bd2)

	be.Flush() // no change -> should stay same
	assert.Equal(t, tbe.BytesTotal, uint64(superBlockSize+2*blockSize))
	assert.Equal(t, tbe.BytesUsed, uint64(superBlockSize+2*blockSize))

	// Update the block; this causes new root, which needs space
	// to be written in, but used total should stay same
	b.RefCount = 1
	be.UpdateBlock(&b)
	be.Flush()
	assert.Equal(t, tbe.BytesTotal, uint64(superBlockSize+3*blockSize))
	assert.Equal(t, tbe.BytesUsed, uint64(superBlockSize+2*blockSize))

	// Delete the block; this should cause us to have only root
	// tree which is now empty in lone block in addition to
	// superblock.
	be.DeleteBlock(&b)
	be.Flush()
	assert.Equal(t, tbe.BytesTotal, uint64(superBlockSize+3*blockSize))
	assert.Equal(t, tbe.BytesUsed, uint64(superBlockSize+1*blockSize))

	defer be.Close()
}