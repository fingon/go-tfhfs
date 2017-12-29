/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2017 Markus Stenberg
 *
 * Created:       Fri Dec 29 13:18:26 2017 mstenber
 * Last modified: Fri Dec 29 13:40:43 2017 mstenber
 * Edit time:     16 min
 *
 */

package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/fingon/go-tfhfs/fs"
	"github.com/hanwen/go-fuse/fuse"
)

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage:\n\n%s MOUNTDIR STORAGEDIR\n", os.Args[0])
		flag.PrintDefaults()
	}
	password := flag.String("password", "siikret", "Password")
	salt := flag.String("salt", "salt", "Salt")
	flag.Parse()
	mountpoint := flag.Arg(0)
	storedir := flag.Arg(1)
	if flag.NArg() < 2 {
		flag.Usage()
		os.Exit(1)
	}
	badgerfs := fs.NewBadgerCryptoFs(storedir, *password, *salt, "xxx")
	lfs := fuse.NewLockingRawFileSystem(badgerfs)
	opts := &fuse.MountOptions{Debug: true}
	server, err := fuse.NewServer(lfs, mountpoint, opts)
	if err != nil {
		log.Panic(err)
	}
	server.Serve()
}
