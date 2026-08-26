use Module::Build;

Module::Build->new(
    requires => { "Runtime::Core" => "1.2" },
    recommends => { "Unicode::ICU::Collator" => 0 },
    build_requires => { "Builder::Tool" => "0.3601" },
    configure_requires => { "Module::Build" => "0.3601" },
    test_requires => { "Test::More" => "0.88" },
)->create_build_script;
