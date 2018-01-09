#!/bin/bash -ue
#-*-sh-*-
#
# Author: Markus Stenberg <fingon@iki.fi>
#
# Copyright (c) 2018 Markus Stenberg
#
# Created:       Mon Jan  8 12:57:07 2018 mstenber
# Last modified: Tue Jan  9 19:01:17 2018 mstenber
# Edit time:     10 min
#

# Attempt to break various go tests by running them repeatedly.
# 
# For example, fs.test: It has some parallel bits (due to plenty
# of goroutine action) and random bits (due to random seqno test) so
# running it repeatedly results sometimes in crashes

rm ,log* || true

for ITER in `seq 1 100`
do
    echo -n Iteration $ITER ..
    if go test $* >& ,log$ITER
    then
        echo "+"
        rm -f ,log$ITER
    else
        echo "-"
    fi
done

