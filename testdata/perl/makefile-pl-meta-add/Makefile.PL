use ExtUtils::MakeMaker;

WriteMakefile(
  NAME      => 'Example::MetaAdd',
  PREREQ_PM => {'File::Spec' => 0},
  META_ADD  => {
    'meta-spec' => {version => 2},
    prereqs => {
      runtime => {
        requires => {'HTTP::Tiny' => '0.080'},
        suggests => {'IO::Socket::SSL' => '2.085'},
      },
      test => {requires => {'Test::More' => '1.302190'}},
    },
  },
);
