use Module::Build;

Module::Build->new(
  module_name        => 'Example::Prereqs',
  configure_requires => {'Module::Build' => '0.4234'},
  build_requires     => {'ExtUtils::CBuilder' => '0.280236'},
  test_requires      => {'Test2::V0' => '0.000162'},
  requires           => {'Try::Tiny' => '0.31'},
  recommends         => {'JSON::MaybeXS' => '1.004008'},
  conflicts          => {'JSON' => '< 4.00'},
)->create_build_script;
