/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2018 Markus Stenberg
 *
 * Created:       Tue Jan 16 11:07:34 2018 mstenber
 * Last modified: Tue Jan 16 11:10:34 2018 mstenber
 * Edit time:     2 min
 *
 */

package fs

import (
	"bytes"
	"fmt"
	"log"
	"testing"

	"github.com/jacobsa/crypto/cmac"
)

func BenchmarkCmac(b *testing.B) {
	// 50 = ~ minimal, 1000 =~ typical treenode, 20k =~ typical file extent
	key := make([]byte, 32)
	result := make([]byte, 0, 32)
	cmac, err := cmac.New(key)
	if err != nil {
		log.Panic(err)
	}
	for _, i := range []int{50, 1000, 20000} {
		data := bytes.Repeat([]byte{0}, i)
		b.Run(fmt.Sprintf("%d", i), func(b *testing.B) {
			b.SetBytes(int64(i))
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				cmac.Reset()
				cmac.Write(data)
				cmac.Sum(result[:])
			}
		})
	}
}
