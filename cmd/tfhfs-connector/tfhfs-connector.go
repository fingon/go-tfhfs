/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2018 Markus Stenberg
 *
 * Created:       Wed Jan 17 18:00:47 2018 mstenber
 * Last modified: Thu Jan 18 11:17:38 2018 mstenber
 * Edit time:     10 min
 *
 */

package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/fingon/go-tfhfs/connector"
	"github.com/fingon/go-tfhfs/mlog"
)

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage:\n\n%s LEFTADDR LEFTNAME LEFTOTHERNAME RIGHTADDR RIGHTNAME RIGHTOTHERNAME\n", os.Args[0])
		flag.PrintDefaults()
	}
	interval := flag.Duration("interval", time.Second*10, "Interval at which synchronization is run (0 = once)")
	flag.Parse()
	if flag.NArg() < 6 {
		flag.Usage()
		os.Exit(1)
	}

	c1 := connector.Connection{Address: flag.Arg(0),
		RootName:      flag.Arg(1),
		OtherRootName: flag.Arg(2)}
	c2 := connector.Connection{Address: flag.Arg(3),
		RootName:      flag.Arg(4),
		OtherRootName: flag.Arg(5)}
	c := connector.Connector{Left: c1, Right: c2}
	for {
		ops, err := c.Run()
		if err != nil {
			log.Panic(err)
		}
		mlog.Printf2("cmd/tfhfs-connector/tfhfs-connector", "Ran %d ops", ops)
		if *interval == 0 {
			break
		}
		mlog.Printf2("cmd/tfhfs-connector/tfhfs-connector", "Waiting %v", *interval)
		time.Sleep(*interval)
	}
}
