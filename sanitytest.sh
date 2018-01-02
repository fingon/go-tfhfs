#!/bin/bash -ue
#-*-sh-*-
#
# Author: Markus Stenberg <fingon@iki.fi>
#
# Copyright (c) 2017 Markus Stenberg
#
# Created:       Tue Jan  2 15:24:47 2018 mstenber
# Last modified: Tue Jan  2 16:07:32 2018 mstenber
# Edit time:     12 min
#

STORAGEDIR=/tmp/sanity-tfhfs-storage
MOUNTDIR=/tmp/x
MLOG=.

out () {
    cd
    echo $*
    umount $MOUNTDIR
    exit 1
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

