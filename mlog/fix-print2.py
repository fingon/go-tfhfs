#!/usr/bin/env python
# -*- coding: utf-8 -*-
# -*- Python -*-
#
# Author: Markus Stenberg <fingon@iki.fi>
#
# Copyright (c) 2017 Markus Stenberg
#
# Created:       Sat Dec 30 15:17:06 2017 mstenber
# Last modified: Sat Dec 30 15:22:54 2017 mstenber
# Edit time:     5 min
#
"""

This is preprocessor for the mlog.Printf2 statements.

It rewrites the code for the given argument files, if any such stmts are included within, with the _exact_ path given.

For example, if we have foo/bar.go file, the first argument of Printf2
will be "foo/bar" (no .go, hah hah).

"""

import os
import re

p2_re = re.compile('(mlog\.Printf2\(")[^"]+(")')


def rewrite_file(filename):
    assert filename.endswith(GOSUFFIX)
    bfilename = filename[:-len(GOSUFFIX)]
    tfilename = '%s.tmp.fp2' % filename
    with open(tfilename, 'w') as f:
        for line in open(filename):
            line = p2_re.sub(lambda m: m.group(
                1) + bfilename + m.group(2), line)
            f.write(line)
    os.rename(tfilename, filename)


if __name__ == '__main__':
    import sys
    GOSUFFIX = ".go"
    for filename in sys.argv[1:]:
        rewrite_file(filename)
