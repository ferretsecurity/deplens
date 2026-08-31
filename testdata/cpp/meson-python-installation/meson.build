project('pyoptsparse', 'c')

py3 = import('python').find_installation(pure: false)
py3_dep = py3.dependency()

cc = meson.get_compiler('c')
m_dep = cc.find_library('m', required: false)
