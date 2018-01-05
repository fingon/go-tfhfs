#
# Author: Markus Stenberg <fingon@iki.fi>
#
# Copyright (c) 2017 Markus Stenberg
#
# Created:       Fri Aug 11 16:08:26 2017 mstenber
# Last modified: Fri Jan  5 16:57:29 2018 mstenber
# Edit time:     68 min
#
#

GREENPACKS=$(wildcard */*_greenpack.go)
GREENPACK_OPTS=-alltuple
# ^ remove -alltuple someday if we want to pretend to be compatible over versions

GENERATED=\
	fs/fstreerootpointer_gen.go

SUBDIRS=codec fs ibtree storage

all: generate test tfhfs

bench: .done.buildable
	go test ./... -bench .

get: .done.getprebuild

generate: .done.generated

fs/fstreerootpointer_gen.go: Makefile xxx/pointer.go
	( echo "package fs" ; \
		egrep -A 9999 '^import' xxx/pointer.go | \
		egrep -v '^(type XXX|// XXX)Type' | \
		sed 's/XXXType/(*fsTreeRoot)/g;s/XXX/fsTreeRoot/g' | \
		cat ) > $@.new
	mv $@.new $@

html-cover-%: .done.cover.%
	go tool cover -html=$<

prof-%: .done.cpuprof.%
	go tool pprof $<

test: .done.test

tfhfs: tfhfs.go $(wildcard */*.go)
	go build -o tfhfs tfhfs.go

tfhfs-darwin: .done.test tfhfs.go $(wildcard */*.go)
	GOOS=darwin go build -o tfhfs-darwin tfhfs.go

tfhfs-linux: .done.test tfhfs.go $(wildcard */*.go)
	GOOS=linux go build -o tfhfs-linux tfhfs.go

update-deps:
	for SUBDIR in $(SUBDIRS); do (cd $$SUBDIR && go get -u . ); done
	for LINE in `cat go-get-deps.txt`; do go get -u $$LINE; done

.done.cover.%: .done.buildable $(wildcard %/*.go)
	(cd $* && go test . -coverprofile=../$@.new)
	mv $@.new $@


.done.cpuprof.%: .done.buildable $(wildcard %/*.go)
	(cd $* && go test -cpuprofile=../$@.new)
	mv $@.new $@

.done.getprebuild: go-get-deps.txt
	for LINE in `cat go-get-deps.txt`; do go get $$LINE; done
	touch $@

.done.get2: $(wildcard %/*.go)
	for SUBDIR in $(SUBDIRS); do (cd $$SUBDIR && go get . ); done
	touch $@


.done.greenpack: .done.getprebuild $(GREENPACKS)
	for FILE in $(GREENPACKS); do greenpack $(GREENPACK_OPTS) -file $$FILE ; done
	touch $@

.done.buildable: .done.greenpack .done.mlog .done.generated .done.get2
	touch $@

.done.generated: $(GENERATED)
	touch $@

.done.test: .done.buildable $(wildcard */*.go)
	go test ./...
	touch $@

.done.mlog: Makefile $(wildcard */*.go)
	find . -type f -name '*.go' | sed 's/^\.\///' | xargs python3 mlog/fix-print2.py
	touch $@
