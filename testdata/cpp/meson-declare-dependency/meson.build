project('declare_dependency', 'cpp', meson_version : '>=0.63.0')

zlib_dep = dependency('zlib')
core_library = static_library('core', 'core.cpp', dependencies : zlib_dep)
core_dep = declare_dependency(
  link_with : core_library,
  dependencies : [zlib_dep],
  compile_args : ['-DCORE_API=1'],
  variables : {'api_level' : '1'})
