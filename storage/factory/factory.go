/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2018 Markus Stenberg
 *
 * Created:       Fri Jan  5 12:22:52 2018 mstenber
 * Last modified: Wed Feb  6 13:31:49 2019 mstenber
 * Edit time:     26 min
 *
 */

package factory

import (
	"github.com/fingon/go-tfhfs/codec"
	"github.com/fingon/go-tfhfs/mlog"
	"github.com/fingon/go-tfhfs/storage"
	"github.com/fingon/go-tfhfs/storage/badger"
	"github.com/fingon/go-tfhfs/storage/file"
	"github.com/fingon/go-tfhfs/storage/inmemory"
	"github.com/fingon/go-tfhfs/storage/tree"
	"github.com/fingon/go-tfhfs/util"
)

type factoryCallback func() storage.Backend

var backendFactories = map[string]factoryCallback{
	"tree": func() storage.Backend {
		return tree.NewTreeBackend()
	},
	"inmemory": func() storage.Backend {
		return inmemory.NewInMemoryBackend()
	},
	"badger": func() storage.Backend {
		return badger.NewBadgerBackend()
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
	mlog.Printf2("storage/factory/factory", "f.NewWithConfig %v %v", name, config)
	be := backendFactories[name]()
	be.Init(config)
	return be
}

type CryptoStorageConfiguration struct {
	storage.BackendConfiguration
	BackendName             string
	Password, Salt          string
	Iterations, QueueLength int
}

func NewCryptoStorage(config CryptoStorageConfiguration) *storage.Storage {
	mlog.Printf2("storage/factory/factory", "f.NewCryptoStorage")
	iterations := util.IOr(config.Iterations, 12345)
	queuelength := util.IOr(config.QueueLength, 100)
	salt := util.SOr(config.Salt, "asdf")
	beconfig := config.BackendConfiguration
	c := &codec.CodecChain{}
	if config.Password != "" {
		mlog.Printf2("storage/factory/factory", " with encryption + compression")
		c1 := codec.EncryptingCodec{}.Init([]byte(config.Password), []byte(salt), iterations)
		c2 := &codec.CompressingCodec{}
		c = c.Init(c1, c2)
	} else {
		mlog.Printf2("storage/factory/factory", " only compression")
		c2 := &codec.CompressingCodec{}
		c = c.Init(c2)
	}
	beconfig.Codec = c
	be := NewWithConfig(config.BackendName, beconfig)

	// If underlying backend takes care of codec, we give nop
	// codec to storage
	if be.Supports(storage.CodecFeature) {
		c = &codec.CodecChain{}
		mlog.Printf2("storage/factory/factory", " backend supports codec -> omitting from storage")
	}
	return storage.Storage{QueueLength: queuelength, Backend: be, Codec: c}.Init()
}
