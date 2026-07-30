use Module::Build;

Module::Build->new(
  module_name => 'Example::Basic',
  license     => 'perl',
  requires    => {
    'List::Util' => 0,
    'Moo'        => '>= 2.005000',
  },
)->create_build_script;
