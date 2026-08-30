use inc::Module::Install;

build_requires
    'HTML::Lint'       => 0,
    'Test::HTML::Lint' => '2.06',
    'Test::More'       => '0.42';

requires
    'B::Hooks::EndOfScope' => 0.05,
    'Devel::Declare'       => 0,
    'String::BufferStack'  => 1.15,
    'HTML::Entities'       => 0,
    'Exporter::Lite'       => 0,
    'Sub::Install'         => 0;

WriteAll;
