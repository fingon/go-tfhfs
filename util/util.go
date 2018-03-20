/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2017 Markus Stenberg
 *
 * Created:       Fri Dec 29 09:03:12 2017 mstenber
 * Last modified: Tue Mar 20 13:36:02 2018 mstenber
 * Edit time:     4 min
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

func IMin(i int, ints ...int) int {
	for _, v := range ints {
		if v < i {
			i = v
		}
	}
	return i
}

func IMax(i int, ints ...int) int {
	for _, v := range ints {
		if v > i {
			i = v
		}
	}
	return i
}

func IOr(i int, ints ...int) int {
	for _, v := range ints {
		if v != 0 {
			return v
		}
	}
	return 0
}

func SOr(i string, strings ...string) string {
	for _, v := range strings {
		if v != "" {
			return v
		}
	}
	return ""
}
