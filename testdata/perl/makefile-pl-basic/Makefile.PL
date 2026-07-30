use strict;
use warnings;
use ExtUtils::MakeMaker;

WriteMakefile(
  NAME      => 'Example::Basic',
  VERSION   => '0.001',
  PREREQ_PM => {
    'List::Util' => 0,
    'Moo'        => '>= 2.005000',
  },
);
