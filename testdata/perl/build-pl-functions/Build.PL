use Module::Build::Functions;

module_name 'Example::Functions';
requires 'Moo' => '2.005005';
requires 'File::Spec';
configure_requires 'Module::Build' => '0.4234';
build_requires 'ExtUtils::CBuilder' => '0.280236';
test_requires 'Test::More' => '1.302190';
recommends 'JSON::MaybeXS' => '1.004008';
conflicts 'JSON' => '< 4.00';
create_build_script;
