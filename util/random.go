/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2018 Markus Stenberg
 *
 * Created:       Fri Mar 16 13:56:39 2018 mstenber
 * Last modified: Fri Mar 16 13:57:11 2018 mstenber
 * Edit time:     0 min
 *
 */

package util

import (
	"log"
	"math/rand"
	"os"
	"strconv"
	"time"

	"github.com/fingon/go-tfhfs/mlog"
)

func newRandWithSource(seedvalue int64) *rand.Rand {
	mlog.Printf2("util/random", "newRandWithSource %v", seedvalue)
	source := rand.NewSource(seedvalue)
	return rand.New(source)

}

func GetSeededRng() *rand.Rand {
	seed := os.Getenv("SEED")

	seedvalue := time.Now().UnixNano()
	if seed != "" {
		v, err := strconv.Atoi(seed)
		if err != nil {
			log.Panic(err)
		}
		seedvalue = int64(v)
	}
	log.Printf("Seed: %v (use SEED= to fix)", seedvalue)
	return newRandWithSource(seedvalue)
}
