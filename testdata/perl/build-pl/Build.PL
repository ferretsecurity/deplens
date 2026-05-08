use Module::Build;
Module::Build->new(
    module_name => "Demo",
    requires    => { "List::Util" => 0 },
)->create_build_script;
