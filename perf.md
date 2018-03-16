# In-memory dict
## Write 5078 megabytes
Command: dd "if=/tmp/perf/size/install-highsierra-app.tgz" of=/tmp/x/foo.dat bs=1048576

Took 14.746984004974365 seconds
344 megabytes per second

## Write 60162 files
Command: rsync -a /tmp/perf/amount /tmp/x/

Took 141.46033692359924 seconds
425 files per second

# Tree (custom nested btree in one file with superblocks)
## Write 5078 megabytes
Command: dd "if=/tmp/perf/size/install-highsierra-app.tgz" of=/tmp/x/foo.dat bs=1048576

Took 28.10564422607422 seconds
180 megabytes per second

## Read it back
Command: find /tmp/x -type f | xargs cat > /dev/null

Took 22.570555210113525 seconds
224.0 megabytes per second

## Write 60162 files
Command: rsync -a /tmp/perf/amount /tmp/x/

Took 232.61201214790344 seconds
258 files per second

## Read it back
Command: find /tmp/x -type f | xargs cat > /dev/null

Took 33.57448077201843 seconds
1791.0 files per second

# Badger
## Write 5078 megabytes
Command: dd "if=/tmp/perf/size/install-highsierra-app.tgz" of=/tmp/x/foo.dat bs=1048576

Took 48.45631814002991 seconds
104 megabytes per second

## Read it back
Command: find /tmp/x -type f | xargs cat > /dev/null

Took 17.043668031692505 seconds
297.0 megabytes per second

## Write 60162 files
Command: rsync -a /tmp/perf/amount /tmp/x/

Took 546.862368106842 seconds
110 files per second

## Read it back
Command: find /tmp/x -type f | xargs cat > /dev/null

Took 27.841084003448486 seconds
2160.0 files per second

# File (raw 64kb blocks on filesystem)
## Write 5078 megabytes
Command: dd "if=/tmp/perf/size/install-highsierra-app.tgz" of=/tmp/x/foo.dat bs=1048576

Took 35.19376611709595 seconds
144 megabytes per second

## Read it back
Command: find /tmp/x -type f | xargs cat > /dev/null

Took 33.84479093551636 seconds
150.0 megabytes per second

## Write 60162 files
Command: rsync -a /tmp/perf/amount /tmp/x/

Took 504.9437429904938 seconds
119 files per second

## Read it back
Command: find /tmp/x -type f | xargs cat > /dev/null

Took 33.60341715812683 seconds
1790.0 files per second

