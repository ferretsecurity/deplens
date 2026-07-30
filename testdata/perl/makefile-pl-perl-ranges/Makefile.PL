use ExtUtils::MakeMaker;

WriteMakefile(
  NAME             => 'Example::Ranges',
  MIN_PERL_VERSION => '5.020',
  PREREQ_PM        => {
    perl            => '>= 5.020, < 5.041',
    'Type::Tiny'    => '>= 2.000000, != 2.001000',
    'Path::Tiny'    => '== 0.146',
    'Version::Next' => v1.2.3,
  },
);
