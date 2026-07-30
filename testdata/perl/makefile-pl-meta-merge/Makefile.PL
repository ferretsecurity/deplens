use ExtUtils::MakeMaker;

WriteMakefile(
  NAME => 'Example::MetaMerge',
  META_MERGE => {
    'meta-spec' => {version => 2},
    prereqs => {
      runtime => {
        requires   => {'JSON::MaybeXS' => '>= 1.004000'},
        recommends => {'Cpanel::JSON::XS' => '4.37'},
        suggests   => {'JSON::XS' => '4.03'},
        conflicts  => {'JSON' => '< 4.00'},
      },
      test => {requires => {'Test::More' => '1.302190'}},
      develop => {requires => {'Test::Pod' => '1.52'}},
    },
  },
);
