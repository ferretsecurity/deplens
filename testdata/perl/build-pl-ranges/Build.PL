use Module::Build;

Module::Build->new(
  module_name => 'Example::Ranges',
  requires    => {
    perl         => '>= 5.020, < 5.041',
    'Type::Tiny' => '>= 2.000000, != 2.001000',
    'Path::Tiny' => '== 0.146',
  },
)->create_build_script;
