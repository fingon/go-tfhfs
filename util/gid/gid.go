/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2018 Markus Stenberg
 *
 * Created:       Thu Jan  4 12:49:31 2018 mstenber
 * Last modified: Tue Jan  9 09:21:47 2018 mstenber
 * Edit time:     0 min
 *
 */

package gid

import (
	"bytes"
	"runtime"
	"strconv"
)

// From http://blog.sgmansfield.com/2015/12/goroutine-ids/
func GetGoroutineID() uint64 {
	b := make([]byte, 64)
	b = b[:runtime.Stack(b, false)]
	b = bytes.TrimPrefix(b, []byte("goroutine "))
	b = b[:bytes.IndexByte(b, ' ')]
	n, _ := strconv.ParseUint(string(b), 10, 64)
	return n
}
