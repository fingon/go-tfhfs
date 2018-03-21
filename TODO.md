# Pending early todo #

* figure to describe arch

* fix fstest failures if I feel like it (rename I do not understand, NXIO
  would be trivial to return)

  sudo prove -f -o -r ~/git/fstest/tests/rename

  => no errors; only when all tests are run it does error out. This implies
  that there is some timing constraint (possibly flush? parallel write from
  previous tests? that interferes)

* fix pjdfstest failures?

* test harder (?)

  https://stackoverflow.com/questions/21565865/filesystem-test-suites has
  plenty of resources, maybe some are still applicable

# Pending later TODO #

* define some way to BlockBackend to index them by Status => can easily get
  blocks with statuses that are awkward (or come up with an algorithm that
  does not require magic state and instead stores its state outside fs root
  in a different tree)

* could make storage/tree actually do multiple parallel read/writes; given
  SSDs and kernel buffer cache, it might still be faster than what it is
  now

# Pending someday todo #

* report to Apple that their ls implementation crashes with dates far
  enough in future? :p

* ask hanwen about sometimes nil out in Link call - is it bug or feature?

* look at Windows support - dokany / winfsp?

* figure how to get supplementary groups on OS X, provide the info to
  osefuxe project, and eventually use the ops' Access method

Test Summary Report (fstest on OS X)
------------------------------------

/Users/mstenber/git/fstest/tests/open/17.t    (Wstat: 0 Tests: 3 Failed: 1)
  Failed test:  2
^ NXIO not returned for pipes without readers and E_NONBLOCK; noncritical

/Users/mstenber/git/fstest/tests/rename/00.t  (Wstat: 0 Tests: 79 Failed: 4)
  Failed tests:  49, 53, 57, 61
Files=184, Tests=1944, 156 wallclock secs ( 0.84 usr  0.34 sys + 14.49 cusr 18.54 csys = 34.21 CPU)
^ rename does not touch ctime for some reason; possibly some sort of
caching issue?

Test Summary Report (pjdfstest on OS X)
---------------------------------------
/Users/mstenber/git/pjdfstest/tests/chown/07.t          (Wstat: 0 Tests: 132 Failed: 19)
  Failed tests:  7, 12, 20, 27, 32, 40, 47, 52, 60, 67, 72
  80, 87, 92, 100, 107, 112, 120, 127
/Users/mstenber/git/pjdfstest/tests/rename/09.t         (Wstat: 0 Tests: 2353 Failed: 16)
  Failed tests:  2269-2272, 2279-2281, 2284, 2289-2292, 2299-2301
                2304
/Users/mstenber/git/pjdfstest/tests/rename/10.t         (Wstat: 0 Tests: 2099 Failed: 8)
  Failed tests:  2056-2058, 2061, 2063-2065, 2068
^ apfs has these same :p

/Users/mstenber/git/pjdfstest/tests/open/17.t           (Wstat: 0 Tests: 3 Failed: 1)
Failed test:  2
^ NXIO

/Users/mstenber/git/pjdfstest/tests/open/06.t           (Wstat: 0 Tests: 144 Failed: 9)
  Failed tests:  67-68, 70-71, 73-74, 76, 80, 84
^ some unexpected EPERMs; kernel-side checks too strident?

/Users/mstenber/git/pjdfstest/tests/rename/00.t         (Wstat: 0 Tests: 150 Failed: 7)
  Failed tests:  96, 100, 104, 108, 112, 116, 120
^ ctime not being updated (deja vu)

/Users/mstenber/git/pjdfstest/tests/rename/21.t         (Wstat: 0 Tests: 16 Failed: 1)
  Failed test:  5
^ peculiar test; it has conditions for both success and failure of rename

Files=232, Tests=8677, 246 wallclock secs ( 1.82 usr  0.49 sys + 39.95 cusr 48.49 csys = 90.75 CPU)
Result: FAIL
