#
# Author: Markus Stenberg <fingon@iki.fi>
#
# Copyright (c) 2017 Markus Stenberg
#
# Created:       Fri Aug 11 16:08:26 2017 mstenber
# Last modified: Sat Dec 23 20:44:10 2017 mstenber
# Edit time:     9 min
#
#

all: protoc
	go test ./...

clean:
	rm -rf .done.* *.pb.go

coverage.out: $(wildcard *.go)
	go test -coverprofile=coverage.out

cover: coverage.out
	go tool cover -html=coverage.out

get: .done.get

protoc: .done.protoc

.done.get: go-get-deps.txt
	for LINE in `cat go-get-deps.txt`; do go get -u $$LINE; done
	touch $@

.done.protoc: .done.get tfhfs_proto/$(wildcard *.proto)
	(cd tfhfs_proto && protoc --go_out=. *.proto )
	touch $@
