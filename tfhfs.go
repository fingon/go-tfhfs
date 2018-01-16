/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2017 Markus Stenberg
 *
 * Created:       Fri Dec 29 13:18:26 2017 mstenber
 * Last modified: Tue Jan 16 14:51:49 2018 mstenber
 * Edit time:     51 min
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
	"github.com/fingon/go-tfhfs/server"
	"github.com/fingon/go-tfhfs/storage/factory"
	"github.com/hanwen/go-fuse/fuse"
)

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage:\n\n%s MOUNTDIR STORAGEDIR\n", os.Args[0])
		flag.PrintDefaults()
	}
	password := flag.String("password", "siikret", "Password")
	salt := flag.String("salt", "salt", "Salt")
	rootName := flag.String("rootname", "root", "Name of the root reference")
	backendp := flag.String("backend", "badger",
		fmt.Sprintf("Backend to use (possible: %v)", factory.List()))
	cpuprofile := flag.String("cpuprofile", "", "CPU profile file")
	memprofile := flag.String("memprofile", "", "Memory profile file")
	cachesize := flag.Int("cachesize", 10000, "Number of btree nodes to cache (~few k each)")
	family := flag.String("family", "tcp", "Address family to use for server")
	address := flag.String("address", "localhost::12345", "Address to use for server")

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

	// storage backend
	backend := factory.New(*backendp, storedir)

	// actual filesystem
	st := fs.NewCryptoStorage(*password, *salt, backend)
	myfs := fs.NewFs(st, *rootName, *cachesize)
	opts := &fuse.MountOptions{AllowOther: true}
	if mlog.IsEnabled() {
		opts.Debug = true
	}

	// grpc server
	rpcServer := (&server.Server{Family: *family, Address: *address, Fs: myfs, Storage: st}).Init()

	// fuse server
	server, err := fuse.NewServer(&myfs.Ops, mountpoint, opts)
	if err != nil {
		log.Panic(err)
	}

	// loop is here
	server.Serve()

	// then close things in order (could use defer, but rather get
	// things cleared before we get out for memory profiling etc)
	rpcServer.Close()

	// myfs will take care of backend clearing as well
	myfs.Close()

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
