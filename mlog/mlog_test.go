/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2017 Markus Stenberg
 *
 * Created:       Sat Dec 30 14:31:18 2017 mstenber
 * Last modified: Tue Jan  9 09:26:12 2018 mstenber
 * Edit time:     28 min
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
	dumpGids = false
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

func TestMLogRecursion(t *testing.T) {
	dumpGids = false
	var b bytes.Buffer
	logger := log.New(&b, "", 0)
	reset()
	defer SetLogger(logger)()
	defer SetPattern(".")()
	Printf("d0")
	func() {
		Printf("d1")
		func() {
			Printf("d2")
		}()
		Printf("D1")
	}()
	Printf("D0")
	assert.Equal(t, string(b.Bytes()), "d0\n.d1\n..d2\n.D1\nD0\n")
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

func BenchmarkChannelSendReceive(b *testing.B) {
	c := make(chan int, 1)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c <- 1
		<-c
	}
}

func BenchmarkChannelPing(b *testing.B) {
	c := make(chan int)
	b.ResetTimer()
	go func() {
		for {
			<-c
		}
	}()
	for i := 0; i < b.N; i++ {
		c <- 1
	}
	close(c)
}

func BenchmarkNewGoroutineChannelPing(b *testing.B) {
	c := make(chan int)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		go func() {
			<-c
		}()
		c <- 1
	}
	close(c)
}
