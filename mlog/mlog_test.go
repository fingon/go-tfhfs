/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2017 Markus Stenberg
 *
 * Created:       Sat Dec 30 14:31:18 2017 mstenber
 * Last modified: Sat Dec 30 15:36:44 2017 mstenber
 * Edit time:     16 min
 *
 */

package mlog

import (
	"bytes"
	"log"
	"regexp"
	"runtime"
	"sync"
	"testing"

	"github.com/stvp/assert"
)

func TestMlog(t *testing.T) {
	add := func(pattern string, outputted bool) {
		t.Run(pattern, func(t *testing.T) {
			log.Printf("pattern:%s outputted:%v", pattern, outputted)
			var b bytes.Buffer
			logger := log.New(&b, "", 0)
			defer SetLogger(logger)()
			defer SetPattern(pattern)()
			Printf("foo %s", "bar")
			assert.True(t, len(b.Bytes()) == 0 == !outputted)
			if outputted {
				assert.Equal(t, string(b.Bytes()), "foo bar\n")
			}

		})
	}
	add("", false)
	add("zzzglorb", false)
	add("mlog_test", true)
}

func BenchmarkMlogDisabled(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Printf("x")
	}
}

func BenchmarkMlogDisabled3(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Printf2("x", "y", 42)
	}
}

func BenchmarkMlogNotMatching(b *testing.B) {
	defer SetPattern("zzglorb")()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Printf("x")
	}
}

func BenchmarkRuntimeCaller(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		runtime.Caller(1)
	}
}

func BenchmarkRuntimeRegexFind(b *testing.B) {
	s := "foobar"
	r := regexp.MustCompile("z")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.Find([]byte(s))
	}
}

func BenchmarkMutexLockUnlock(b *testing.B) {
	var m sync.Mutex
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.Lock()
		m.Unlock()
	}
}
