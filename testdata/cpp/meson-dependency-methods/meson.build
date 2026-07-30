project('dependency_methods', 'c', meson_version : '>=0.63.0')

png_dep = dependency('libpng', method : 'pkg-config', required : true)
openssl_dep = dependency('OpenSSL', method : 'cmake', modules : ['SSL', 'Crypto'])
threads_dep = dependency('threads')
