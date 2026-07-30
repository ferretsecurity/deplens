project('cross_provider_controls', 'c', meson_version : '>=0.63.0')

build_tool_dep = dependency(
  'pkgconf',
  native : true,
  language : 'c',
  include_type : 'system',
  required : false,
  not_found_message : 'pkgconf is optional for this build')
fallback_dep = dependency(
  'embedded-library',
  required : false,
  allow_fallback : true,
  fallback : ['embedded-library', 'embedded_dep'])
