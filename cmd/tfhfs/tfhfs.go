/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2017 Markus Stenberg
 *
 * Created:       Fri Dec 29 13:18:26 2017 mstenber
 * Last modified: Tue Mar 20 16:00:57 2018 mstenber
 * Edit time:     68 min
 *
 */

package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"runtime"
	"runtime/pprof"

	"github.com/fingon/go-tfhfs/fs"
	"github.com/fingon/go-tfhfs/mlog"
	"github.com/fingon/go-tfhfs/server"
	"github.com/fingon/go-tfhfs/storage"
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
	cachesize := flag.Int("cachesize", 10000, "Number of btree nodes to cache (~few k each, may be up to 2x this due to 2 places using same variable)")
	//family := flag.String("family", "tcp", "Address family to use for server")
	address := flag.String("address", "", "Address to use for server")
	profile := flag.Bool("profile", false, "Whether to enable profiling 'bonus stuff'")

	flag.Parse()

	if *profile {
		runtime.SetBlockProfileRate(1000)    // microsecond
		runtime.SetMutexProfileFraction(100) // 1/100 is enough
	}
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

	// actual filesystem
	beconf := storage.BackendConfiguration{Directory: storedir, CacheSize: *cachesize}
	conf := factory.CryptoStorageConfiguration{BackendConfiguration: beconf,
		BackendName: *backendp, Password: *password, Salt: *salt}
	st := factory.NewCryptoStorage(conf)
	myfs := fs.NewFs(st, *rootName, *cachesize)
	opts := &fuse.MountOptions{AllowOther: true}
	if mlog.IsEnabled() {
		opts.Debug = true
	}

	// twirp server
	var serv *server.Server

	if *address != "" {
		serv = (&server.Server{Address: *address, Fs: myfs, Storage: st}).Init()
	}

	// fuse server
	fuseServer, err := fuse.NewServer(&myfs.Ops, mountpoint, opts)
	if err != nil {
		log.Panic(err)
	}

	// loop is here
	fuseServer.Serve()

	// then close things in order (could use defer, but rather get
	// things cleared before we get out for memory profiling etc)
	if serv != nil {
		serv.Close()
	}

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
