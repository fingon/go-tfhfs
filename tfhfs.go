/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2017 Markus Stenberg
 *
 * Created:       Fri Dec 29 13:18:26 2017 mstenber
 * Last modified: Thu Jan  4 12:07:12 2018 mstenber
 * Edit time:     38 min
 *
 */

package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"runtime/pprof"

	"github.com/fingon/go-tfhfs/fs"
	"github.com/fingon/go-tfhfs/mlog"
	"github.com/fingon/go-tfhfs/storage"
	"github.com/hanwen/go-fuse/fuse"
)

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage:\n\n%s MOUNTDIR STORAGEDIR\n", os.Args[0])
		flag.PrintDefaults()
	}
	password := flag.String("password", "siikret", "Password")
	salt := flag.String("salt", "salt", "Salt")
	backendp := flag.String("backend", "badger", "Backend to use (possible: file, badger, bolt, inmemory)")
	cpuprofile := flag.String("cpuprofile", "", "CPU profile file")
	memprofile := flag.String("memprofile", "", "Memory profile file")

	flag.Parse()

	if *cpuprofile != "" {
		f, err := os.Create(*cpuprofile)
		if err != nil {
			log.Fatal(err)
		}
		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()
	}
	mountpoint := flag.Arg(0)
	storedir := flag.Arg(1)
	if flag.NArg() < 2 {
		flag.Usage()
		os.Exit(1)
	}
	var backend storage.BlockBackend
	switch *backendp {
	case "bolt":
		backend = storage.BoltBlockBackend{}.Init(storedir)
	case "badger":
		backend = storage.BadgerBlockBackend{}.Init(storedir)
	case "file":
		be := &storage.FileBlockBackend{}
		be.Init(storedir)
		backend = be
	case "inmemory":
		backend = storage.InMemoryBlockBackend{}.Init()
	default:
		log.Panicf("Invalid backend: %s", *backendp)
	}

	st := fs.NewCryptoStorage(*password, *salt, backend)
	myfs := fs.NewFs(st, "xxx")

	defer myfs.Close()
	opts := &fuse.MountOptions{}
	if mlog.IsEnabled() {
		opts.Debug = true
	}
	server, err := fuse.NewServer(&myfs.Ops, mountpoint, opts)
	if err != nil {
		log.Panic(err)
	}
	server.Serve()

	if *memprofile != "" {
		f, err := os.Create(*memprofile)
		if err != nil {
			log.Fatal(err)
		}
		pprof.WriteHeapProfile(f)
		f.Close()
		return
	}
}
