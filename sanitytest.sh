#!/bin/bash -ue
#-*-sh-*-
#
# Author: Markus Stenberg <fingon@iki.fi>
#
# Copyright (c) 2017 Markus Stenberg
#
# Created:       Tue Jan  2 15:24:47 2018 mstenber
# Last modified: Wed Jan  3 12:18:39 2018 mstenber
# Edit time:     27 min
#

STORAGEDIR=/tmp/sanity-tfhfs-storage
MOUNTDIR=/tmp/x
MLOG=.

out () {
    echo $*
    umount $MOUNTDIR
    exit 1
}

mount2 () {
    if [ $# == 1 ]; then
        if [ "$1" = "d" ]; then
            # debug
            MLOG=$MLOG ./tfhfs $MOUNTDIR $STORAGEDIR >& ,log2 &
            # profile
            return
        fi
        if [ "$1" = "p" ]; then
            # profiling
            ./tfhfs -memprofile mem.prof -cpuprofile cpu.prof $MOUNTDIR $STORAGEDIR >& ,log2 &
            return
        fi
    fi
    # fast
    ./tfhfs $MOUNTDIR $STORAGEDIR >& ,log2 &
}

waitmount () {
    mount | grep -q $MOUNTDIR && return || (echo "Waiting for mount.."; sleep 1 ; waitmount)
}

make tfhfs
mount | grep -q $MOUNTDIR && umount $MOUNTDIR || true
rm -rf $MOUNTDIR
mkdir -p $MOUNTDIR
rm -rf $STORAGEDIR
mkdir -p $STORAGEDIR
MLOG=$MLOG ./tfhfs $MOUNTDIR $STORAGEDIR >& ,log &
waitmount
ORIGDIR=`pwd`
cd $MOUNTDIR
mkdir dir
echo foo > foo
echo bar > bar
echo baz > baz
[ -d "dir" ] || out "dir not dir"
[ -f "foo" ] || out "foo not present"
[ ! -f "nonexistent" ] || out "nonexistent present"
ls -l $MOUNTDIR
GOT=`ls -l $MOUNTDIR | wc -l`
[ $GOT = "5" ] || out "not 5 lines in ls ($GOT)"
cp /bin/ls $MOUNTDIR || out "ls cp failed"
cmp -s /bin/ls $MOUNTDIR/ls || out "copied ls differs"
ls -l $MOUNTDIR/ls
cd $ORIGDIR
umount $MOUNTDIR

mount2 $*
waitmount
[ -f $MOUNTDIR/ls  ] || out "second mount: copied ls not present"
ls -l $MOUNTDIR/ls
cmp -s /bin/ls $MOUNTDIR/ls || out "second mount:  ls differs"
