#!/usr/bin/env python3
# -*- coding: utf-8 -*-
# -*- Python -*-
#
# $Id: perf_fs.py $
#
# Author: Markus Stenberg <fingon@iki.fi>
#
# Copyright (c) 2016 Markus Stenberg
#
# Created:       Sun Dec 25 08:04:44 2016 mstenber
# Last modified: Wed Jan 24 15:46:21 2018 mstenber
# Edit time:     88 min
#
"""This is 'whole'-system benchmark used to gather data for populating
the 'official' performance figures with.

( Grabbed from the original tfhfs project, refactored to use sanitytest.sh for mounting drive )

"""

import argparse
import os
import sys

from mounter import Mounter


def _system(cmd):
    rc = os.system(cmd)
    if rc:
        sys.exit(42)


if __name__ == '__main__':

    global __package__
    if __package__ is None:
        import python3fuckup
        __package__ = python3fuckup.get_package(__file__, 1)

    import time

    read_cmd = 'find /tmp/x -type f | xargs cat > /dev/null'
    tests = [
        ('In-memory dict', dict(backend='inmemory')),
        ('Badger', dict()),
        ('Bolt', dict(backend='bolt')),
        ('File', dict(backend='file')),
    ]

    p = argparse.ArgumentParser(
        formatter_class=argparse.ArgumentDefaultsHelpFormatter)
    p.add_argument('--test', '-t', type=int, default=-1,
                   help='Run single test (-1 = all)')
    args = p.parse_args()
    if args.test >= 0:
        tests = [tests[args.test]]
    for desc, opts in tests:
        print(f'# {desc}')
        for write_cmd, units, unit_type in [
                (  # 'dd "if=/tmp/perf/size/install-highsierra-app.tgz" of=/tmp/x/foo.dat bs=1024000',
                    'rsync /tmp/perf/size/install-highsierra-app.tgz /tmp/x/foo.dat',
                    5078, 'megabyte'),  # 1 file :p
                ('rsync -a /tmp/perf/amount /tmp/x/',
                 60162, 'file'),  # 1194MB
        ]:
            print(f'## Write {units} {unit_type}s')

            print(f'Command: {write_cmd}')
            args = []
            mountpoint = '/tmp/x'
            storagedir = '/tmp/sanity-tfhfs-storage'
            m = Mounter(mountpoint, storagedir, clean=True, **opts)
            start_time = time.time()
            failed = True
            if m.mounted():
                _system(write_cmd)
                if m.mounted():
                    failed = False
                    m.close()
                    write_time = time.time() - start_time
                    cnt = units // write_time
                    print()
                    print(f'Took {write_time} seconds')
                    print(f'{cnt} {unit_type}s per second')
                    print()

            if opts.get('backend') != 'inmemory':
                print(f'## Read it back')
                print(f'Command: {read_cmd}')
                m = Mounter(mountpoint, storagedir, **opts)
                if m.mounted():
                    start_time = time.time()
                    _system(read_cmd)
                    if m.mounted():
                        m.close()
                        read_time = time.time() - start_time
                        cnt = units // read_time
                        print()
                        print(f'Took {read_time} seconds')
                        print(f'{cnt} {unit_type}s per second')
                        print()
