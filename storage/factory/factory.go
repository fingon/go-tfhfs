/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2018 Markus Stenberg
 *
 * Created:       Fri Jan  5 12:22:52 2018 mstenber
 * Last modified: Wed Jan 10 11:34:07 2018 mstenber
 * Edit time:     11 min
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

type factoryCallback func() storage.Backend

var backendFactories = map[string]factoryCallback{
	"inmemory": func() storage.Backend {
		return inmemory.NewInMemoryBackend()
	},
	"badger": func() storage.Backend {
		return badger.NewBadgerBackend()
	},
	"bolt": func() storage.Backend {
		return bolt.NewBoltBackend()
	},
	"file": func() storage.Backend {
		return file.NewFileBackend()
	}}

func List() []string {
	keys := make([]string, 0, len(backendFactories))
	for k, _ := range backendFactories {
		keys = append(keys, k)
	}
	return keys
}

func New(name, dir string) storage.Backend {
	var config storage.BackendConfiguration
	config.Directory = dir
	return NewWithConfig(name, config)
}

func NewWithConfig(name string, config storage.BackendConfiguration) storage.Backend {
	be := backendFactories[name]()
	be.Init(config)
	return be
}
