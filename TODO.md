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

# Pending later TODO #

* define some way to BlockBackend to index them by Status => can easily get
  blocks with statuses that are awkward (or come up with an algorithm that
  does not require magic state and instead stores its state outside fs root
  in a different tree)

# Pending someday todo #

* report to Apple that their ls implementation crashes with dates far
  enough in future? :p

* ask hanwen about sometimes nil out in Link call - is it bug or feature?

* look at Windows support - dokany / winfsp?
