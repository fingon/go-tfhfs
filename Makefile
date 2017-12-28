#
# Author: Markus Stenberg <fingon@iki.fi>
#
# Copyright (c) 2017 Markus Stenberg
#
# Created:       Fri Aug 11 16:08:26 2017 mstenber
# Last modified: Thu Dec 28 19:55:34 2017 mstenber
# Edit time:     34 min
#
#

GREENPACKS=$(wildcard */*_greenpack.go)
GREENPACK_OPTS=-alltuple
# ^ remove -alltuple someday if we want to pretend to be compatible over versions

SUBDIRS=btree codec storage

all: generate test

bench: .done.buildable
	go test ./... -bench .

get: .done.getprebuild

generate: .done.buildable

html-cover-%: .done.cover.%
	go tool cover -html=$<

prof-%: .done.cpuprof.%
	go tool pprof $<


test: .done.buildable
	go test -cpuprofile $@.new ./...

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

.done.buildable: .done.greenpack .done.get2
