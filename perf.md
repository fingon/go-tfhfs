# Performance results

**NOTE**: Some results omitted currently as there are bugs that cause
crashes which give too big numbers. This is just result of one random run. 

All results (including in-memory backend) include deduplication using
SHA256, and AES256-GCM encryption/decryption.

# In-memory dict
## Write 5078 megabytes
Command: dd "if=/tmp/perf/size/install-highsierra-app.tgz" of=/tmp/x/foo.dat bs=1048576

Took 23.94075894355774 seconds
212.0 megabytes per second

## Write 60162 files
Command: rsync -a /tmp/perf/amount /tmp/x/

Took 211.01718711853027 seconds
285.0 files per second

# Badger
## Write 5078 megabytes
Command: dd "if=/tmp/perf/size/install-highsierra-app.tgz" of=/tmp/x/foo.dat bs=1048576

Took 64.77219295501709 seconds
78.0 megabytes per second

## Read it back
Command: find /tmp/x -type f | xargs cat > /dev/null

Took 19.77255892753601 seconds
256.0 megabytes per second

## Write 60162 files
Command: rsync -a /tmp/perf/amount /tmp/x/

Took 750.4327480792999 seconds
80.0 files per second

# Bolt
## Write 5078 megabytes
Command: dd "if=/tmp/perf/size/install-highsierra-app.tgz" of=/tmp/x/foo.dat bs=1048576

Took 364.39971566200256 seconds
13.0 megabytes per second

## Read it back
Command: find /tmp/x -type f | xargs cat > /dev/null

Took 35.15881609916687 seconds
144.0 megabytes per second

## Write 60162 files
Command: rsync -a /tmp/perf/amount /tmp/x/

Took 2694.8448917865753 seconds
22.0 files per second

## Read it back
Command: find /tmp/x -type f | xargs cat > /dev/null

Took 32.41966104507446 seconds
1855.0 files per second

# File
## Write 5078 megabytes
Command: dd "if=/tmp/perf/size/install-highsierra-app.tgz" of=/tmp/x/foo.dat bs=1048576

Took 63.31801390647888 seconds
80.0 megabytes per second

## Read it back
Command: find /tmp/x -type f | xargs cat > /dev/null

Took 49.989299058914185 seconds
101.0 megabytes per second

## Write 60162 files
Command: rsync -a /tmp/perf/amount /tmp/x/

Took 831.329797744751 seconds
72.0 files per second

