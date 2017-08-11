# go-tfhfs #

This is the next-generation rewrite of my earlier
[tfhfs project](https://github.com/fingon/tfhfs) that was written in
Python.

This is very preliminary still, but notable planned changes include:

* using inodes instead of paths as bases for the btree forest => semantics
more closely match UNIX

* implement it in Go, instead of Python (mostly because FUSE performance at
  least on OS X seems rather horrid within Python compared to Go; see my
  [fuse bindings benchmark](https://github.com/fingon/fuse-binding-test) for more details.

Design is mostly done, I wish I had just few weeks of coding time to
actually implement this next iteration :) We shall see how long this will
take. 
