use Module::Build 0.28;

my %arguments = (
    build_requires => { "Module::Build" => "0.28" },
    configure_requires => { "Module::Build" => "0.28" },
    requires => { "perl" => "5.008005" },
    test_requires => { "Test::More" => 0 },
);
Module::Build->new(%arguments)->create_build_script;
