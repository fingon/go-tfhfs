go-tfhfs
========

This is the next-generation rewrite of my earlier
[tfhfs project](https://github.com/fingon/tfhfs) that was written in
Python.

This is very preliminary still, but notable planned changes include:

* using inodes instead of paths as bases for the btree forest => semantics
more closely match UNIX

* implement it in Go, instead of Python (mostly because FUSE performance at
  least on OS X seems rather horrid within Python compared to Go; see my
  [fuse bindings benchmark](https://github.com/fingon/fuse-binding-test)
  for more details

* make the design leverage parallelism much more ( parallel sha256,
  encryption, tree merging )

Design is mostly done, I wish I had just few weeks of coding time to
actually implement this next iteration :) We shall see how long this will
take. 

![Current components](doc/overview.svg)

[Performance figures](perf.md)

Build
-----

This does not really use normal Go building style as I do not like storing
generated files in the tree. So if you want to play with it, you have to
play with it my way (I might be convinced to change this given good
arguments, so if you care, tell me why).

* git clone ..

* make

Usage
-----

After that, targets of interest are:

./sanitytest.sh which runs minimal test which consists of:

* mounting one volume at /tmp/x (with server at localhost:12345)

* mounting second volume at /tmp/y (with server at localhost:12346)

* copying thigs from root filesystem (/bin/ls mainly) and setting things up
in /tmp/x

* synchronizing the state between /tmp/x and /tmp/y using tfhfs-connector
utility

* umounting /tmp/x, /tmp/y

* remounting /tmp/x using the same fixed storage

* still ensuring /tmp/x has state we set up there

and

of course the built ./tfhfs mounting binary, and ./tfhfs-connector
synchronization utility (more documentation TBD, look at sanitytest.sh or
usage if you feel adventurous).

*NOTE*: You REALLY do not want to expose tfhfs server to non-localhost use
at the moment; it is plain HTTP/1.1 without any security
mechanisms. However, as the block content itself is not plaintext, and it
performs relatively rigorous checks on input, even exposing it to public
Internet will have only not particularly bad outcomes:

* resource exhaustion attack (store lot of blocks)

* network utilization attack (get blocks ad nauseaum)

* (limited) synchronization mischief; can attempt to merge in bit older
roots, but typically merge routine should simply ignore this.


