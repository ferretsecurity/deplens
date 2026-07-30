use ExtUtils::MakeMaker;

my %prereqs = (
  'HTTP::Tiny' => '0.080',
);

if ($^O eq 'MSWin32') {
  $prereqs{'Win32'} = 0;
}

WriteMakefile(
  NAME      => 'Example::Conditional',
  PREREQ_PM => \%prereqs,
);
