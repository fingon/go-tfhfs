/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2018 Markus Stenberg
 *
 * Created:       Wed Feb 21 17:11:02 2018 mstenber
 * Last modified: Tue Mar 20 13:22:47 2018 mstenber
 * Edit time:     47 min
 *
 */

package tree

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/fingon/go-tfhfs/codec"
	"github.com/fingon/go-tfhfs/storage"
	"github.com/stvp/assert"
)

func TestTreeSuperblockSafety(t *testing.T) {
	t.Parallel()

	be := NewTreeBackend()
	tbe := be.(*treeBackend)
	config := storage.BackendConfiguration{}
	be.Init(config)
	slices := make([]LocationSlice, 0)
	for tbe.numberOfSuperBlocks() < 3 {
		sl := tbe.allocateSlice(42)
		// Ensure the allocation does not cross superblock boundary
		for i := 0; i < tbe.numberOfSuperBlocks(); i++ {
			ofs := superBlockOffset(i)
			for _, se := range sl {
				if se.Offset < ofs {
					e := se.Offset + se.Size
					assert.True(t, e <= ofs)
				} else {
					esb := ofs + superBlockSize
					assert.True(t, se.Offset >= esb)
				}
			}
		}
		slices = append(slices, sl)
	}
}

func TestTreeGrowthAndShrinking(t *testing.T) {
	t.Parallel()

	dir, _ := ioutil.TempDir("", "tree")
	defer os.RemoveAll(dir)

	// A lot of growth / shrinking in one transaction seems to hit
	// superblock boundary. This unit test ensures that is no
	// longer the case.
	be := NewTreeBackend()
	tbe := be.(*treeBackend)
	config := storage.BackendConfiguration{Directory: dir}
	be.Init(config)
	slices := make([]LocationSlice, 0)
	n := 100000
	for len(slices) < n {
		sl := tbe.allocateSlice(42)
		slices = append(slices, sl)
	}
	flush := func() {
		tbe.unchangedRoot = nil
		tbe.Flush()
		be2 := NewTreeBackend()
		tbe2 := be2.(*treeBackend)
		be2.Init(config)
		assert.Equal(t, tbe2.Superblock, tbe.Superblock)
		// TBD: Check also that content matches?
	}
	flush()
	zapIndex := func(ofs, div int) {
		i := 0
		for _, sl := range slices {
			if i%div == ofs {
				tbe.freeSlice(sl)
			}
			i++
		}
	}
	zapIndex(0, 3)
	flush()

	zapIndex(1, 3)
	flush()
	zapIndex(2, 3)
	flush()
}

func TestTree(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		inmem bool
		n     string
		codec bool
	}{{false, "plain", false},
		{true, "im-plain", false},
		{false, "encrypted", true}} {

		var dir string
		if !test.inmem {
			dir, _ = ioutil.TempDir("", "tree")
			defer os.RemoveAll(dir)
		}

		config := storage.BackendConfiguration{Directory: dir}

		if test.codec {
			config.Codec = codec.EncryptingCodec{}.Init([]byte("foo"), []byte("salty"), 123)
		}

		t.Run(test.n, func(t *testing.T) {
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

		})
	}
}
