project('optional_static', 'cpp', meson_version : '>=0.63.0')

optional_dep = dependency('libunwind', required : false, static : true)
disabled_dep = dependency('backtrace', required : false, disabler : true)
internal_dep = dependency('project-internal', required : false, allow_fallback : false)
