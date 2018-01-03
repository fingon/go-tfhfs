/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2018 Markus Stenberg
 *
 * Created:       Wed Jan  3 10:22:32 2018 mstenber
 * Last modified: Wed Jan  3 10:30:06 2018 mstenber
 * Edit time:     5 min
 *
 */

package fs

import (
	"bytes"
	sha256base "crypto/sha256"
	"fmt"
	"testing"

	sha256simd "github.com/minio/sha256-simd"
)

func BenchmarkShaBase256(b *testing.B) {
	// 50 = ~ minimal, 1000 =~ typical treenode, 20k =~ typical file extent
	for _, i := range []int{50, 1000, 20000} {
		data := bytes.Repeat([]byte{0}, i)
		b.Run(fmt.Sprintf("builtin-%d", i), func(b *testing.B) {
			b.SetBytes(int64(i))
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				sha256base.Sum256(data)
			}
		})
		b.Run(fmt.Sprintf("simd-%d", i), func(b *testing.B) {
			b.SetBytes(int64(i))
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				sha256simd.Sum256(data)
			}
		})
	}
}
