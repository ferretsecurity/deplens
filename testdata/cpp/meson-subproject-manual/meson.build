project('subproject_manual', 'cpp', meson_version : '>=0.63.0')

libsimple_project = subproject(
  'libsimple',
  default_options : ['default_library=static'],
  required : false,
  version : '>=1.0')
if libsimple_project.found()
  libsimple_dep = libsimple_project.get_variable('libsimple_dep')
endif
