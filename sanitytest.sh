#!/bin/bash -ue
#-*-sh-*-
#
# Author: Markus Stenberg <fingon@iki.fi>
#
# Copyright (c) 2017 Markus Stenberg
#
# Created:       Tue Jan  2 15:24:47 2018 mstenber
# Last modified: Fri Jan 12 12:19:19 2018 mstenber
# Edit time:     53 min
#

STORAGEDIR=/tmp/sanity-tfhfs-storage
MOUNTDIR=/tmp/x
MLOG=.
ARGS=${ARGS:-}

out () {
    cd $ORIGDIR
    echo $*
    sleep 1
    umount $MOUNTDIR
    exit 1
}

mount2 () {
    if [ $# == 1 ]; then
        if [ "$1" = "d" ]; then
            # debug
            MLOG=$MLOG ./tfhfs `echo $ARGS` $MOUNTDIR $STORAGEDIR >& ,log2 &
            # profile
            return
        fi
        if [ "$1" = "p" ]; then
            # profiling
            ./tfhfs `echo $ARGS` -memprofile mem.prof -cpuprofile cpu.prof $MOUNTDIR $STORAGEDIR >& ,log2 &
            return
        fi
        if [ "$1" = "r" ]; then
            # race detector
            go run `echo $ARGS` -race ./tfhfs.go $MOUNTDIR $STORAGEDIR >& ,log2 &
            return
        fi
    fi
    # fast
    ./tfhfs `echo $ARGS` $MOUNTDIR $STORAGEDIR >& ,log2 &
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
MLOG=$MLOG ./tfhfs `echo $ARGS`  $MOUNTDIR $STORAGEDIR >& ,log &
waitmount
ORIGDIR=`pwd`
cd $MOUNTDIR
mkdir dir
echo foo > foo
echo bar > bar
echo baz > baz
ln -v -s foo symlink || out "symlink broken"
ln -v foo hardlink || out "hardlink broken"
[ -d "dir" ] || out "dir not dir"
[ -f "foo" ] || out "foo not present"
[ ! -f "nonexistent" ] || out "nonexistent present"
ls -il $MOUNTDIR
GOT=`ls -l $MOUNTDIR | wc -l`
[ $GOT = "7" ] || out "not 7 lines in ls ($GOT)"
cp /bin/ls $MOUNTDIR || out "ls cp failed"
cmp -s /bin/ls $MOUNTDIR/ls || out "copied ls differs"
ls -l $MOUNTDIR/ls
cd $ORIGDIR
umount $MOUNTDIR
sleep 1
mount2 $*
waitmount
[ -f $MOUNTDIR/ls  ] || out "second mount: copied ls not present"
ls -l $MOUNTDIR/ls
cmp -s /bin/ls $MOUNTDIR/ls || out "second mount:  ls differs"
