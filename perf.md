# In-memory dict
## Write 5078 megabytes
Command: dd "if=/tmp/perf/size/install-highsierra-app.tgz" of=/tmp/x/foo.dat bs=1048576

Took 13.427963733673096 seconds
378 megabytes per second

## Write 60162 files
Command: rsync -a /tmp/perf/amount /tmp/x/

Took 136.44552612304688 seconds
440 files per second

# Tree (custom nested btree in one file with superblocks)
## Write 5078 megabytes
Command: dd "if=/tmp/perf/size/install-highsierra-app.tgz" of=/tmp/x/foo.dat bs=1048576

Took 25.206897258758545 seconds
201 megabytes per second

## Read it back
Command: find /tmp/x -type f | xargs cat > /dev/null

Took 17.086349964141846 seconds
297 megabytes per second

## Write 60162 files
Command: rsync -a /tmp/perf/amount /tmp/x/

Took 211.086168050766 seconds
285 files per second

## Read it back
Command: find /tmp/x -type f | xargs cat > /dev/null

Took 26.752897024154663 seconds
2248 files per second

# Badger
## Write 5078 megabytes
Command: dd "if=/tmp/perf/size/install-highsierra-app.tgz" of=/tmp/x/foo.dat bs=1048576

Took 49.97873115539551 seconds
101 megabytes per second

## Read it back
Command: find /tmp/x -type f | xargs cat > /dev/null

Took 17.274454593658447 seconds
293 megabytes per second

## Write 60162 files
Command: rsync -a /tmp/perf/amount /tmp/x/

Took 541.6776812076569 seconds
111 files per second

## Read it back
Command: find /tmp/x -type f | xargs cat > /dev/null

Took 27.569681882858276 seconds
2182 files per second

# File (raw 64kb blocks on filesystem)
## Write 5078 megabytes
Command: dd "if=/tmp/perf/size/install-highsierra-app.tgz" of=/tmp/x/foo.dat bs=1048576

Took 39.061781883239746 seconds
129 megabytes per second

## Read it back
Command: find /tmp/x -type f | xargs cat > /dev/null

Took 27.635469675064087 seconds
183 megabytes per second

## Write 60162 files
Command: rsync -a /tmp/perf/amount /tmp/x/

Took 540.6193330287933 seconds
111 files per second

## Read it back
Command: find /tmp/x -type f | xargs cat > /dev/null

Took 62.51984786987305 seconds
962 files per second

