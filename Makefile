#
# Author: Markus Stenberg <fingon@iki.fi>
#
# Copyright (c) 2017 Markus Stenberg
#
# Created:       Fri Aug 11 16:08:26 2017 mstenber
# Last modified: Wed Aug 16 17:07:32 2017 mstenber
# Edit time:     8 min
#
#

all: protoc
	cd btree && go test
	go test

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

.done.protoc: .done.get $(wildcard *.proto)
	protoc --go_out=. *.proto
	touch $@
