#!/bin/bash -ue
#-*-sh-*-
#
# Author: Markus Stenberg <fingon@iki.fi>
#
# Copyright (c) 2017 Markus Stenberg
#
# Created:       Tue Jan  2 15:24:47 2018 mstenber
# Last modified: Thu Jan 18 11:26:20 2018 mstenber
# Edit time:     72 min
#

STORAGEDIR=/tmp/sanity-tfhfs-storage
MOUNTDIR=/tmp/x
STORAGEDIR2=/tmp/sanity-tfhfs-storage2
MOUNTDIR2=/tmp/y
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
    echo "mount2 $*"
    mountdir=$1
    shift
    storagedir=$1
    shift
    if [ $# == 1 ]; then
        if [ "$1" = "d" ]; then
            # debug
            MLOG=$MLOG echocmd ./tfhfs `echo $ARGS` $mountdir $storagedir >& ,log2 &
            # profile
            return
        fi
        if [ "$1" = "p" ]; then
            # profiling
            echocmd ./tfhfs `echo $ARGS` -memprofile mem.prof -cpuprofile cpu.prof $mountdir $storagedir >& ,log2 &
            return
        fi
        if [ "$1" = "r" ]; then
            # race detector
            echocmd go run `echo $ARGS` -race ./tfhfs.go $mountdir $storagedir >& ,log2 &
            return
        fi
    fi
    # fast
    echocmd ./tfhfs `echo $ARGS` $mountdir $storagedir >& ,log2 &
    waitmount $mountdir
}

echocmd () {
    echo "# $*" 
    $*
}

waitmount () {
    mountdir=$1

    mount | grep -q $mountdir && return || (echo "Waiting for mount.."; sleep 1 ; waitmount $mountdir)
}

mount1 () {
    echo "mount1 $*"
    mountdir=$1
    shift
    storagedir=$1
    shift
    logname=$1
    shift

    mount | grep -q $mountdir && umount $mountdir || true
    rm -rf $mountdir
    mkdir -p $mountdir
    rm -rf $storagedir
    mkdir -p $storagedir
    echocmd ./tfhfs $* `echo $ARGS`  $mountdir $storagedir >& $logname &
    waitmount $mountdir
}

make tfhfs tfhfs-connector
ADDRESS=localhost:12345
ADDRESS2=localhost:12346
MLOG=$MLOG mount1 $MOUNTDIR $STORAGEDIR ,log -address $ADDRESS
MLOG=$MLOG mount1 $MOUNTDIR2 $STORAGEDIR2 ,log1 -address $ADDRESS2
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

echocmd ./tfhfs-connector -interval 0s $ADDRESS root s2 $ADDRESS2 root s1  >& ,logc
[ -f $MOUNTDIR2/ls  ] || out "sync mount: copied ls not present"
ls -l $MOUNTDIR2/ls
umount $MOUNTDIR2

umount $MOUNTDIR
sleep 1

mount2 $MOUNTDIR $STORAGEDIR $*
[ -f $MOUNTDIR/ls  ] || out "second mount: copied ls not present"
ls -l $MOUNTDIR/ls
cmp -s /bin/ls $MOUNTDIR/ls || out "second mount:  ls differs"
