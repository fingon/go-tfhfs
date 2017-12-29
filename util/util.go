/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2017 Markus Stenberg
 *
 * Created:       Fri Dec 29 09:03:12 2017 mstenber
 * Last modified: Fri Dec 29 09:08:59 2017 mstenber
 * Edit time:     3 min
 *
 */

package util

import "encoding/binary"

func ConcatBytes(bytes ...[]byte) []byte {
	nl := 0
	for _, b := range bytes {
		nl += len(b)
	}
	r := make([]byte, 0, nl)
	for _, b := range bytes {
		r = append(r, b...)
	}
	return r
}

func Uint32Bytes(n uint32) []byte {
	nb := make([]byte, 4)
	binary.BigEndian.PutUint32(nb, n)
	return nb
}

func Uint64Bytes(n uint64) []byte {
	nb := make([]byte, 8)
	binary.BigEndian.PutUint64(nb, n)
	return nb
}
