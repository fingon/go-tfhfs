/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2017 Markus Stenberg
 *
 * Created:       Fri Dec 29 13:18:26 2017 mstenber
 * Last modified: Wed Jan  3 17:11:10 2018 mstenber
 * Edit time:     32 min
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
	badger := flag.Bool("badger", false, "Use Badger instead of raw files")
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
	if *badger {
		be := storage.BadgerBlockBackend{}.Init(storedir)
		backend = be
	} else {
		be := &storage.FileBlockBackend{}
		be.Init(storedir)
		backend = be
	}

	st := fs.NewCryptoStorage(*password, *salt, backend)
	myfs := fs.NewFs(st, "xxx")

	defer myfs.Close()
	opts := &fuse.MountOptions{}
	if mlog.IsEnabled() {
		opts.Debug = true
	}
	server, err := fuse.NewServer(myfs.LockedOps, mountpoint, opts)
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
