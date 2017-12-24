# Pending early todo #


* persistent (= immutable) btree with peds-based sub-vectors (or maybe not,
  TBD - go get github.com/tobgu/peds/cmd/peds)

* implement things from Python prototype - done: storage, codec; pending:
  forest, inode, ops.

* immutable-btree (iteration 2); arbitrary string key+value with fancy ids
  derived from data using sha256 of plaintext data + encryption key. Keep
  track of both mutated and non-mutated nodes in each subtree so when
  merging in stuff, can choose the larger tree as needed even if both are
  not mutated.

* tree-merger (new design); input: 2 trees, output: new tree => flush to
  storage, use as base for further w operations

* inode (or equivalent)

* glue to fuse binding

# Pending later TODO #

* define some way to BlockBackend to index them by Status => can easily get
  blocks with statuses that are awkward

* while Storage is holy, synchronous sacred animal, sub-part of it (notably
  where it calls codec's EncodeBytes during Flush) should be
  parallelized. Decoding part is probably easier to make occur in each
  fs-using goroutine by default.
