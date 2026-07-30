use Module::Build;

Module::Build->new(
  module_name => 'Example::Metadata',
  meta_merge  => {
    resources => {repository => {type => 'git', url => 'https://example.test/project.git'}},
    prereqs   => {
      runtime => {requires => {'JSON::MaybeXS' => '1.004008'}},
      develop => {requires => {'Test::Pod' => '1.52'}},
    },
  },
)->create_build_script;
