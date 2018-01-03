/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2018 Markus Stenberg
 *
 * Created:       Wed Jan  3 13:27:19 2018 mstenber
 * Last modified: Wed Jan  3 13:33:28 2018 mstenber
 * Edit time:     4 min
 *
 */

package storage

import (
	"bytes"
	"strings"
	"testing"

	"github.com/fingon/go-tfhfs/mlog"
)

func BenchmarkStringToByteSlice20K(b *testing.B) {
	s := strings.Repeat("42", 10000)
	b.SetBytes(int64(len(s)))
	by := []byte{}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		by = []byte(s)
	}
	mlog.Printf("hopefully mlog not on", by)
}

func BenchmarkByteSliceToString20K(b *testing.B) {
	by := bytes.Repeat([]byte{42}, 20000)
	b.SetBytes(int64(len(by)))
	s := "xxx"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s = string(by)
	}
	mlog.Printf("hopefully mlog not on", s)
}
