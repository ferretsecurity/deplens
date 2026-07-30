use ExtUtils::MakeMaker;

WriteMakefile(
  NAME               => 'Example::Phases',
  VERSION            => '0.001',
  CONFIGURE_REQUIRES => {'ExtUtils::MakeMaker' => '>= 7.64'},
  BUILD_REQUIRES     => {'ExtUtils::CBuilder' => '0.280236'},
  TEST_REQUIRES      => {'Test::More' => '>= 1.302190'},
  PREREQ_PM          => {'Try::Tiny' => '0.31'},
);
