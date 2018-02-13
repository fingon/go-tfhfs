# In-memory dict
## Write 5078 megabytes
Command: dd "if=/tmp/perf/size/install-highsierra-app.tgz" of=/tmp/x/foo.dat bs=1048576

Took 13.77706503868103 seconds
368 megabytes per second

## Write 60162 files
Command: rsync -a /tmp/perf/amount /tmp/x/

Took 126.6615538597107 seconds
474 files per second

# Badger
## Write 5078 megabytes
Command: dd "if=/tmp/perf/size/install-highsierra-app.tgz" of=/tmp/x/foo.dat bs=1048576

Took 52.104098081588745 seconds
97 megabytes per second

## Read it back
Command: find /tmp/x -type f | xargs cat > /dev/null

Took 20.546954870224 seconds
247.0 megabytes per second

## Write 60162 files
Command: rsync -a /tmp/perf/amount /tmp/x/

Took 539.0603127479553 seconds
111 files per second

## Read it back
Command: find /tmp/x -type f | xargs cat > /dev/null

Took 34.039758920669556 seconds
1767.0 files per second

# File
## Write 5078 megabytes
Command: dd "if=/tmp/perf/size/install-highsierra-app.tgz" of=/tmp/x/foo.dat bs=1048576

Took 37.444523096084595 seconds
135 megabytes per second

## Read it back
Command: find /tmp/x -type f | xargs cat > /dev/null

Took 24.949825048446655 seconds
203.0 megabytes per second

## Write 60162 files
Command: rsync -a /tmp/perf/amount /tmp/x/

Took 487.79161500930786 seconds
123 files per second

## Read it back
Command: find /tmp/x -type f | xargs cat > /dev/null

Took 39.1469190120697 seconds
1536.0 files per second

