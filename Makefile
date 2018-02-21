#
# Author: Markus Stenberg <fingon@iki.fi>
#
# Copyright (c) 2017 Markus Stenberg
#
# Created:       Fri Aug 11 16:08:26 2017 mstenber
# Last modified: Fri Feb 16 10:19:24 2018 mstenber
# Edit time:     116 min
#
#

GREENPACKS=$(wildcard */*_greenpack.go) $(wildcard */*/*_greenpack.go)
GREENPACK_OPTS=-alltuple
# ^ remove -alltuple someday if we want to pretend to be compatible over versions

GENERATED=\
	ibtree/hugger/treerootpointer_gen.go \
	fs/inodemetapointer_gen.go \
	storage/blockpointerfuture_gen.go \
	storage/jobtype_string.go \
	util/bytesliceatomiclist_gen.go \
	util/byteslicefuture_gen.go \
	util/byteslicepointer_gen.go \
	util/stringlist_gen.go \
	util/maprunnercallbacklist_gen.go

SUBDIRS=\
  codec fs ibtree ibtree/hugger mlog server \
  storage storage/badger storage/bolt util

all: generate test tfhfs tfhfs-connector

bench: .done.buildable
	go test ./... -bench .

get: .done.getprebuild

generate: .done.buildable

storage/jobtype_string.go: storage/storagejob.go
	( cd storage && stringer -type=jobType . )


ibtree/hugger/treerootpointer_gen.go: Makefile xxx/pointer.go
	( echo "package hugger" ; \
		egrep -A 9999 '^import' xxx/pointer.go | \
		sed 's/XXXType/(*treeRoot)/g;s/XXX/treeRoot/g' | \
		cat ) > $@.new
	mv $@.new $@

fs/inodemetapointer_gen.go: Makefile xxx/pointer.go
	( echo "package fs" ; \
		egrep -A 9999 '^import' xxx/pointer.go | \
		sed 's/XXXType/(*InodeMeta)/g;s/XXX/InodeMeta/g' | \
		cat ) > $@.new
	mv $@.new $@

storage/blockpointerfuture_gen.go: Makefile xxx/future.go
	( echo "package storage" ; \
		egrep -A 9999 '^import' xxx/future.go | \
		sed 's/YYYType/*Block/g;s/YYY/BlockPointer/g' | \
		cat ) > $@.new
	mv $@.new $@


util/bytesliceatomiclist_gen.go: Makefile xxx/atomiclist.go
	( echo "package util" ; \
		egrep -A 9999 '^import' xxx/atomiclist.go | \
		egrep -v '/util"' | \
		sed 's/util\.//g;s/XXXType/[]byte/g;s/XXX/ByteSlice/g;s/xxx/byteSlice/g' | \
		cat ) > $@.new
	mv $@.new $@


util/byteslicefuture_gen.go: Makefile xxx/future.go
	( echo "package util" ; \
		egrep -A 9999 '^import' xxx/future.go | \
		egrep -v '/util"' | \
		sed 's/util\.//g;s/YYYType/[]byte/g;s/YYY/ByteSlice/g' | \
		cat ) > $@.new
	mv $@.new $@


util/byteslicepointer_gen.go: Makefile xxx/pointer.go
	( echo "package util" ; \
		egrep -A 9999 '^import' xxx/pointer.go | \
		sed 's/XXXType/(*[]byte)/g;s/XXX/ByteSlice/g' | \
		cat ) > $@.new
	mv $@.new $@

util/stringlist_gen.go: Makefile xxx/list.go
	( echo "package util" ; \
		egrep -A 9999 '^import' xxx/list.go | \
		sed 's/YYYType/string/g;s/YYY/String/g' | \
		cat ) > $@.new
	mv $@.new $@

util/maprunnercallbacklist_gen.go: Makefile xxx/list.go
	( echo "package util" ; \
		egrep -A 9999 '^import' xxx/list.go | \
		sed 's/YYYType/MapRunnerCallback/g;s/YYY/MapRunnerCallback/g' | \
		cat ) > $@.new
	mv $@.new $@

html-cover-%: .done.cover.%
	go tool cover -html=$<

prof-%: .done.cpuprof.%
	go tool pprof $<

test: .done.test

tfhfs: cmd/tfhfs/tfhfs.go $(wildcard */*.go)
	go build -o ./tfhfs cmd/tfhfs/tfhfs.go


tfhfs-darwin: .done.test tfhfs
	GOOS=darwin go build -o tfhfs-darwin cmd/tfhfs/tfhfs.go

tfhfs-linux: .done.test tfhfs
	GOOS=linux go build -o tfhfs-linux cmd/tfhfs/tfhfs.go

tfhfs-connector: cmd/tfhfs-connector/tfhfs-connector.go $(wildcard */*.go)
	go build -o ./tfhfs-connector cmd/tfhfs-connector/tfhfs-connector.go

update-deps:
	for SUBDIR in $(SUBDIRS); do (cd $$SUBDIR && go get -t -u . ); done
	for LINE in `cat go-get-deps.txt`; do go get -u $$LINE; done

.done.cover.%: .done.buildable $(wildcard %/*.go)
	(cd $* && go test . -coverprofile=../$@.new)
	mv $@.new $@


.done.cpuprof.%: .done.buildable $(wildcard %/*.go)
	(cd $* && go test -cpuprofile=../$@.new)
	mv $@.new $@

.done.getprebuild: Makefile go-get-deps.txt
	for LINE in `cat go-get-deps.txt`; do go get $$LINE; done
	touch $@

.done.get2: Makefile $(wildcard %/*.go)
	for SUBDIR in $(SUBDIRS); do (cd $$SUBDIR && go get -t . ); done
	touch $@


.done.greenpack: .done.getprebuild $(GREENPACKS)
	for FILE in $(GREENPACKS); do greenpack $(GREENPACK_OPTS) -file $$FILE ; done
	touch $@

.done.buildable: .done.greenpack .done.protoc  .done.generated .done.mlog .done.get2
	touch $@

.done.generated: $(GENERATED)
	touch $@

.done.test: .done.buildable .done.generated $(wildcard */*.go)
	go test ./...
	touch $@

.done.mlog: Makefile $(wildcard */*.go)
	find . -type f -name '*.go' | sed 's/^\.\///' | xargs python3 mlog/fix-print2.py
	touch $@

.done.protoc: Makefile pb/$(wildcard *.proto)
	(cd pb && protoc --go_out=. --twirp_out=. *.proto )
	touch $@

fstest: tfhfs tfhfs-connector
	./sanitytest.sh d
	cd /tmp/x && sudo prove -f -o -r ~mstenber/git/fstest/tests && umount /tmp/x

prep-perf:
	rm -rf /tmp/perf
	mkdir -p /tmp/perf/size
	mkdir -p /tmp/perf/amount
	cp ~/software/mac/install-highsierra-app.tgz /tmp/perf/size
	rsync -a ~/share/1/Maildir/.Junk /tmp/perf/amount

perf.md: .done.perf
	cp .done.perf $@

.done.perf: tfhfs
	support/perf_fs.py | tee $@.new
	egrep -q 'Took ' $@.new
	mv $@.new $@
