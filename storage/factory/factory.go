/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2018 Markus Stenberg
 *
 * Created:       Fri Jan  5 12:22:52 2018 mstenber
 * Last modified: Fri Jan  5 16:29:49 2018 mstenber
 * Edit time:     9 min
 *
 */

package factory

import (
	"github.com/fingon/go-tfhfs/storage"
	"github.com/fingon/go-tfhfs/storage/badger"
	"github.com/fingon/go-tfhfs/storage/bolt"
	"github.com/fingon/go-tfhfs/storage/file"
	"github.com/fingon/go-tfhfs/storage/inmemory"
)

type factoryCallback func(dir string) storage.Backend

var backendFactories = map[string]factoryCallback{
	"inmemory": func(dir string) storage.Backend {
		return inmemory.NewInMemoryBackend()
	},
	"badger": func(dir string) storage.Backend {
		return badger.NewBadgerBackend(dir)
	},
	"bolt": func(dir string) storage.Backend {
		return bolt.NewBoltBackend(dir)
	},
	"file": func(dir string) storage.Backend {
		return file.NewFileBackend(dir)
	}}

func List() []string {
	keys := make([]string, 0, len(backendFactories))
	for k, _ := range backendFactories {
		keys = append(keys, k)
	}
	return keys
}

func New(name, dir string) storage.Backend {
	return backendFactories[name](dir)
}
