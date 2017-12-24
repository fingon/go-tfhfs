#
# Author: Markus Stenberg <fingon@iki.fi>
#
# Copyright (c) 2017 Markus Stenberg
#
# Created:       Fri Aug 11 16:08:26 2017 mstenber
# Last modified: Sun Dec 24 13:26:04 2017 mstenber
# Edit time:     15 min
#
#

GREENPACKS=$(wildcard */*_greenpack.go)
GREENPACK_OPTS=-alltuple
# ^ remove -alltuple someday if we want to pretend to be compatible over versions

all: generate
	go test ./...

clean:
	rm -rf .done.* *.pb.go

coverage.out: $(wildcard *.go)
	go test -coverprofile=coverage.out

cover: coverage.out
	go tool cover -html=coverage.out

get: .done.get

generate: .done.greenpack

.done.get: go-get-deps.txt
	for LINE in `cat go-get-deps.txt`; do go get -u $$LINE; done
	touch $@

#.done.protoc: .done.get tfhfs_proto/$(wildcard *.proto)
#	(cd tfhfs_proto && protoc --go_out=. *.proto )
#	touch $@

.done.greenpack: $(GREENPACKS)
	for FILE in $(GREENPACKS); do greenpack $(GREENPACK_OPTS) -file $$FILE ; done
	touch $@
