project('compiler_libraries', 'c', meson_version : '>=0.63.0')

cc = meson.get_compiler('c')
math_dep = cc.find_library('m', required : true)
dl_dep = cc.find_library('dl', required : false)
vendor_dep = cc.find_library('vendor_codec', dirs : ['/opt/vendor/lib'], required : false, static : true)
iconv_dep = dependency('iconv', required : false)
