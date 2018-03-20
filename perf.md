# In-memory dict
## Write 5078 megabytes
Command: dd "if=/tmp/perf/size/install-highsierra-app.tgz" of=/tmp/x/foo.dat bs=1048576

Took 13.475411176681519 seconds
376 megabytes per second

## Write 60162 files
Command: rsync -a /tmp/perf/amount /tmp/x/

Took 139.53556299209595 seconds
431 files per second

# Badger
## Write 5078 megabytes
Command: dd "if=/tmp/perf/size/install-highsierra-app.tgz" of=/tmp/x/foo.dat bs=1048576

Took 50.83405804634094 seconds
99 megabytes per second

## Read it back
Command: find /tmp/x -type f | xargs cat > /dev/null

Took 17.049104928970337 seconds
297 megabytes per second

## Write 60162 files
Command: rsync -a /tmp/perf/amount /tmp/x/

Took 546.1091620922089 seconds
110 files per second

## Read it back
Command: find /tmp/x -type f | xargs cat > /dev/null

Took 27.30282974243164 seconds
2203 files per second

# Tree (custom nested btree in one file with superblocks)
## Write 5078 megabytes
Command: dd "if=/tmp/perf/size/install-highsierra-app.tgz" of=/tmp/x/foo.dat bs=1048576

Took 35.6706600189209 seconds
142 megabytes per second

## Read it back
Command: find /tmp/x -type f | xargs cat > /dev/null

Took 24.561628103256226 seconds
206 megabytes per second

## Write 60162 files
Command: rsync -a /tmp/perf/amount /tmp/x/

Took 218.62881302833557 seconds
275 files per second

## Read it back
Command: find /tmp/x -type f | xargs cat > /dev/null

Took 29.043033123016357 seconds
2071 files per second

# File (raw 64kb blocks on filesystem)
## Write 5078 megabytes
Command: dd "if=/tmp/perf/size/install-highsierra-app.tgz" of=/tmp/x/foo.dat bs=1048576

Took 35.00848698616028 seconds
145 megabytes per second

## Read it back
Command: find /tmp/x -type f | xargs cat > /dev/null

Took 22.370288848876953 seconds
226 megabytes per second

## Write 60162 files
Command: rsync -a /tmp/perf/amount /tmp/x/

Took 507.20417189598083 seconds
118 files per second

## Read it back
Command: find /tmp/x -type f | xargs cat > /dev/null

Took 33.76305794715881 seconds
1781 files per second

