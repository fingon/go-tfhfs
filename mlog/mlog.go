/*
 * Author: Markus Stenberg <fingon@iki.fi>
 *
 * Copyright (c) 2017 Markus Stenberg
 *
 * Created:       Sat Dec 30 13:41:33 2017 mstenber
 * Last modified: Tue Jan  9 09:27:12 2018 mstenber
 * Edit time:     92 min
 *
 */

// mlog is maybe-log, or Markus' log. It is basically small wrapper
// (mlog only implements Printf) of standard 'log', with two major
// improvements:
//
// - environment-variable-based and 'flag' options for choosing what
// to print; what is not printed will not cause any overhead either (
// by default, everything is off)
//
// - to facilitate tracing, call stack depth is used to determine
// indentation automatically
package mlog

import (
	"flag"
	"fmt"
	"log"
	"os"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/fingon/go-tfhfs/util/gid"
)

var logMode = log.Ltime | log.Lmicroseconds
var logger = log.New(os.Stderr, "", logMode)

const (
	StateUninitialized int32 = iota
	StateInitializing
	StateDisabled
	StateEnabled
)

// This can be used by anyone, with the atomic access
var status int32 = StateUninitialized

var mutex sync.Mutex

// Everything else must be used only with mutex held
var flagPattern *string
var pattern string
var patternRegexp *regexp.Regexp
var file2Debug map[string]*bool
var minDepth int
var callers []uintptr

const maxDepth = 100

func init() {
	flagPattern = flag.String("mlog", "", "Enable logging based on the given file/line regular expression")
	reset()
}

// Reset resets the module to its factory default state. It should not
// really have much visible impact on users though; first subsequent
// log call will re-initialize the internal datastructures and the
// later ones will perform as normal.
func reset() {
	mutex.Lock()
	defer mutex.Unlock()
	atomic.StoreInt32(&status, StateUninitialized)
	minDepth = maxDepth
	callers = make([]uintptr, maxDepth)
}

// IsEnabled can be used to check if mlog is in use at all before
// doing something expensive.
func IsEnabled() bool {
	st := atomic.LoadInt32(&status)
	return st != StateDisabled
}

// SetLogger allows overriding of the logger used as output when mlog
// actually wants to forward Printf somewhere. The returned undo
// function can be used to change the logger back to old one.
func SetLogger(l *log.Logger) (undo func()) {
	mutex.Lock()
	defer mutex.Unlock()
	oldLogger := logger
	logger = l
	return func() {
		mutex.Lock()
		defer mutex.Unlock()
		logger = oldLogger
	}
}

// SetLogger allows setting the mlog pattern by hand, overriding the
// environment variable-provided values. The returned undo function
// can be used to change the state back to old one.
func SetPattern(p string) (undo func()) {
	mutex.Lock()
	defer mutex.Unlock()
	oldPattern := pattern
	initializeWithPattern(p)
	return func() {
		mutex.Lock()
		defer mutex.Unlock()
		initializeWithPattern(oldPattern)

	}
}

func initializeWithPattern(p string) {
	if p == "" {
		// log.Printf("mlog disabled")
		atomic.StoreInt32(&status, StateDisabled)
		pattern = p
		return
	}
	// log.Printf("mlog enabled with pattern %v", p)
	patternRegexp = regexp.MustCompile(p)
	file2Debug = make(map[string]*bool)
	atomic.StoreInt32(&status, StateEnabled)
	pattern = p
}

func initialize() {
	if !atomic.CompareAndSwapInt32(&status, StateUninitialized, StateInitializing) {
		return
	}
	pattern := os.Getenv("MLOG")
	if *flagPattern != "" {
		pattern = *flagPattern
	}
	initializeWithPattern(pattern)

}

// Printf is drop-in replacement of log.Printf. However, it still does
// runtime.Caller() if MLOG is enabled at all, which may be
// suboptimal.
func Printf(format string, args ...interface{}) {
	st := atomic.LoadInt32(&status)
	if st == StateDisabled {
		return
	}
	// This is BY FAR the most expensive operation
	// (~microsecond-ish; regexp match is 1/10, and mutex unlock
	// 1/100 of that)
	_, file, _, ok := runtime.Caller(1)
	if !ok {
		return
	}
	Printf2(file, format, args...)
}

var dumpGids = true

// Printf2 is the premier choice instead of Printf. It is supplied
// with the name of the file, and therefore has no runtime penalty to
// speak of when using only partial MLOG match.
func Printf2(file string, format string, args ...interface{}) {
	st := atomic.LoadInt32(&status)
	if st == StateDisabled {
		return
	}
	mutex.Lock()
	if st < StateDisabled {
		initialize()
		st = atomic.LoadInt32(&status)
		if st <= StateDisabled {
			mutex.Unlock()
			return
		}
	}
	debug := true
	debugp := file2Debug[file]
	if debugp == nil {
		debug = patternRegexp.Find([]byte(file)) != nil
		file2Debug[file] = &debug
		// log.Printf("debugging of %s set to %s", file, debug)
	} else {
		debug = *debugp
		// log.Printf("debugging of %s was %s", file, debug)
	}
	depth := 0
	if debug {
		depth = runtime.Callers(1, callers)
		// log.Printf("depth:%d minDepth:%d", depth, minDepth)
		if depth < minDepth {
			minDepth = depth
		}
		depth -= minDepth

		// TBD: something like this worth it, or not?
		if depth > 0 {
			format = fmt.Sprint(strings.Repeat(".", depth), format)
		}

		// Bake in goroutine id
		if dumpGids {
			format = fmt.Sprintf("%8d %s", gid.GetGoroutineID(), format)
		}

		logger.Printf(format, args...)
	}
	mutex.Unlock()
}
