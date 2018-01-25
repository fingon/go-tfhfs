Test Summary Report
-------------------

/Users/mstenber/git/fstest/tests/open/17.t    (Wstat: 0 Tests: 3 Failed: 1)
  Failed test:  2
^ NXIO not returned for pipes without readers and E_NONBLOCK; noncritical

/Users/mstenber/git/fstest/tests/rename/00.t  (Wstat: 0 Tests: 79 Failed: 4)
  Failed tests:  49, 53, 57, 61
Files=184, Tests=1944, 156 wallclock secs ( 0.84 usr  0.34 sys + 14.49 cusr 18.54 csys = 34.21 CPU)
^ rename does not touch ctime for some reason

# Pending early todo #

* figure to describe arch

* fix above 5 testfs failures if I feel like it (rename I do not
  understand, NXIO would be trivial to return)

* test harder (?)

  https://stackoverflow.com/questions/21565865/filesystem-test-suites has
  plenty of resources;

  https://github.com/pjd/pjdfstest seems to be much more comprehensive than
  fstest (~50% more LoC); however adding fuse support for it might be big
  task and even default apfs fails some of the cases :p

* figure out the rare race condition(s)

 * sometimes directory torture test in middle has listdir with 0 entries;
 how?

 * sometimes there is nonexistent block reference under heavy load;
 related?

* improve performance

 * rewrite so that ReferOrStore is fully async (TBD exact semantics; maybe
   stick in the block to storageblock, and change it to real block at
   flush?)

# Pending later TODO #

* add some sort of reasonable caching to Storage, and get rid of gcache;
  something like e.g. CART seems sensible (
  https://www.usenix.org/legacy/events/fast04/tech/full_papers/bansal/bansal.pdf
  )

* define some way to BlockBackend to index them by Status => can easily get
  blocks with statuses that are awkward (or come up with an algorithm that
  does not require magic state and instead stores its state outside fs root
  in a different tree)

# Pending someday todo #

* report to Apple that their ls implementation crashes with dates far
  enough in future? :p

* ask hanwen about sometimes nil out in Link call - is it bug or feature?

* look at Windows support - dokany / winfsp?
