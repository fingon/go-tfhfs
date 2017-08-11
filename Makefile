#
# Author: Markus Stenberg <fingon@iki.fi>
#
# Copyright (c) 2017 Markus Stenberg
#
# Created:       Fri Aug 11 16:08:26 2017 mstenber
# Last modified: Fri Aug 11 16:08:29 2017 mstenber
# Edit time:     0 min
#
#

all:
	go test

coverage.out: $(wildcard *.go)
	go test -coverprofile=coverage.out

cover: coverage.out
	go tool cover -html=coverage.out
