#!/usr/bin/env python
# -*- coding: utf-8 -*-
# -*- Python -*-
#
# Author: Markus Stenberg <fingon@iki.fi>
#
# Copyright (c) 2018 Markus Stenberg
#
# Created:       Tue Jan 23 12:36:27 2018 mstenber
# Last modified: Wed Jan 24 15:47:00 2018 mstenber
# Edit time:     16 min
#
"""

"""

import os
import os.path
import shutil
import subprocess
import time

_support_dir = os.path.dirname(__file__)
_tfhfs = os.path.join(_support_dir, '..', 'tfhfs')


class Mounter:
    """This is convenience class that can be used to mount tfhfs (with
    arbitrary options) somewhere. It also provides 'close' method for
    getting rid of it cleanly.
    """

    def __init__(self, mountpoint, storagedir, clean=False, **kwargs):
        self.mountpoint = mountpoint
        self.storagedir = storagedir
        self.kwargs = kwargs
        while self.mounted():
            self.unmount()
        if os.path.isdir(mountpoint):
            shutil.rmtree(mountpoint)
        os.mkdir(mountpoint)
        if clean:
            if os.path.isdir(storagedir):
                shutil.rmtree(storagedir)
                os.mkdir(storagedir)
        args = []
        for k, v in kwargs.items():
            args.extend(['-%s' % k, v])
        args.extend([self.mountpoint, self.storagedir])
        self.p = subprocess.Popen([_tfhfs] + list(args), stdout=2)
        self.wait()

        # just in case, there's sometimes still bit of a race
        # condition between fuse filesystem being up and mount showing
        # it
        time.sleep(1)

    closed = False

    def close(self):
        if self.closed:
            return
        self.closed = True
        try:
            self.p.wait(timeout=10)
        except subprocess.TimeoutExpired:
            self.p.terminate()
            try:
                self.p.wait(timeout=10)
            except subprocess.TimeoutExpired:
                self.p.kill()
                self.p.wait()

    def mounted(self):
        return self.mountpoint in os.popen('mount').read()

    def wait(self):
        # Wait for the mount to go up; if mountpoint shows up in
        # 'mount' output it is good enough I guess.
        for i in range(5):
            if self.mounted():
                break
            time.sleep(1)

    def unmount(self):
        try:
            subprocess.call(['umount', self.mountpoint],
                            stderr=subprocess.DEVNULL)
        except:
            pass
