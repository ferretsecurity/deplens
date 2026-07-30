project('dependency_fallbacks', 'cpp', meson_version : '>=0.63.0')

fmt_dep = dependency(
  'fmt',
  version : '>=10.0',
  fallback : ['fmt', 'fmt_dep'],
  default_options : ['default_library=static'])
catch2_dep = dependency('catch2', fallback : 'catch2')
