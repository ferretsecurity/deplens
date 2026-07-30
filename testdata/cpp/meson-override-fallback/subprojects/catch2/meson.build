project('catch2', 'cpp')

catch2_library = static_library('catch2', 'catch2.cpp')
catch2_dep = declare_dependency(link_with : catch2_library)
meson.override_dependency('catch2', catch2_dep)
