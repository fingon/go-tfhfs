# In-memory dict
## Write 5720 megabytes
Command: dd "if=/tmp/perf/size/install-mojave.tgz" of=/tmp/x/foo.dat bs=1048576

Took 16.99096179008484 seconds
336 megabytes per second

## Write 64481 files
Command: rsync -a /tmp/perf/amount /tmp/x/

Took 76.07438802719116 seconds
847 files per second

# Badger (unsafe)
## Write 5720 megabytes
Command: dd "if=/tmp/perf/size/install-mojave.tgz" of=/tmp/x/foo.dat bs=1048576

Took 31.641685247421265 seconds
180 megabytes per second

## Read it back
Command: find /tmp/x -type f | xargs cat > /dev/null

Took 35.88524794578552 seconds
159 megabytes per second

## Write 64481 files
Command: rsync -a /tmp/perf/amount /tmp/x/

Took 141.39858889579773 seconds
456 files per second

## Read it back
Command: find /tmp/x -type f | xargs cat > /dev/null

Took 24.599828958511353 seconds
2621 files per second

# Badger
## Write 5720 megabytes
Command: dd "if=/tmp/perf/size/install-mojave.tgz" of=/tmp/x/foo.dat bs=1048576

Took 37.487074851989746 seconds
152 megabytes per second

## Read it back
Command: find /tmp/x -type f | xargs cat > /dev/null

Took 41.891806840896606 seconds
136 megabytes per second

## Write 64481 files
Command: rsync -a /tmp/perf/amount /tmp/x/

Took 280.03475308418274 seconds
230 files per second

## Read it back
Command: find /tmp/x -type f | xargs cat > /dev/null

Took 21.74087405204773 seconds
2965 files per second

# Tree (custom nested btree in one file with superblocks)
## Write 5720 megabytes
Command: dd "if=/tmp/perf/size/install-mojave.tgz" of=/tmp/x/foo.dat bs=1048576

Took 51.1850311756134 seconds
111 megabytes per second

## Read it back
Command: find /tmp/x -type f | xargs cat > /dev/null

Took 45.37874412536621 seconds
126 megabytes per second

## Write 64481 files
Command: rsync -a /tmp/perf/amount /tmp/x/

Took 123.71357679367065 seconds
521 files per second

## Read it back
Command: find /tmp/x -type f | xargs cat > /dev/null

Took 20.650289058685303 seconds
3122 files per second

# File (raw 64kb blocks on filesystem)
## Write 5720 megabytes
Command: dd "if=/tmp/perf/size/install-mojave.tgz" of=/tmp/x/foo.dat bs=1048576

Took 28.519735097885132 seconds
200 megabytes per second

## Read it back
Command: find /tmp/x -type f | xargs cat > /dev/null

Took 47.583134174346924 seconds
120 megabytes per second

## Write 64481 files
Command: rsync -a /tmp/perf/amount /tmp/x/

Took 310.05190110206604 seconds
207 files per second

## Read it back
Command: find /tmp/x -type f | xargs cat > /dev/null

Took 25.16872811317444 seconds
2561 files per second

