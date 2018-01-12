# Pending early todo #

* figure 

* test harder

 fstest: mkdir seems to fail; successful sequence seems to be LOOKUP(name),
 ACCESS(w), MKDIR

# Pending later TODO #

* define some way to BlockBackend to index them by Status => can easily get
  blocks with statuses that are awkward (or come up with an algorithm that
  does not require magic state and instead stores its state outside fs root
  in a different tree)

# Pending someday todo #

* report to Apple that their ls implementation crashes with dates far
  enough in future? :p

* look at Windows support - dokany / winfsp?
