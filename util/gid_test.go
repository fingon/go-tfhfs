/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2018 Markus Stenberg
 *
 * Created:       Thu Jan  4 13:00:27 2018 mstenber
 * Last modified: Thu Jan  4 13:00:44 2018 mstenber
 * Edit time:     0 min
 *
 */

package util

import "testing"

func BenchmarkGetGoroutineID(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		GetGoroutineID()
	}
}
