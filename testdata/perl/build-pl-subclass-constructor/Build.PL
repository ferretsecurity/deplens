use Module::Build;

my %arguments = (
    configure_requires => { "Module::Build" => 0.38 },
);
Module::Build->subclass(class => "Local::Builder")->new(%arguments)->create_build_script;
