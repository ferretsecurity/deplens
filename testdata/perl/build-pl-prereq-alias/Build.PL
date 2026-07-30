use Module::Build;

Module::Build->new(
  module_name => 'Example::Alias',
  prereq      => {
    'File::Spec' => 0,
    'URI'        => '5.29',
  },
)->create_build_script;
