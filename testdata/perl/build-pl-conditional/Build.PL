use Module::Build;

my %requires = (
  'HTTP::Tiny' => '0.080',
);

if ($^O eq 'MSWin32') {
  $requires{'Win32'} = 0;
}

Module::Build->new(
  module_name => 'Example::Conditional',
  requires    => \%requires,
)->create_build_script;
