/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2018 Markus Stenberg
 *
 * Created:       Wed Feb 21 17:11:02 2018 mstenber
 * Last modified: Wed Feb 21 17:13:35 2018 mstenber
 * Edit time:     3 min
 *
 */

package tree

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/fingon/go-tfhfs/storage"
)

func TestTree(t *testing.T) {
	t.Parallel()

	dir, _ := ioutil.TempDir("", "tree")
	defer os.RemoveAll(dir)

	config := storage.BackendConfiguration{Directory: dir}

	be := NewTreeBackend()
	be.Init(config)
	be.Flush()
	defer be.Close()
}
