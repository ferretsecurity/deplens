include(CMakeFindDependencyMacro)
find_dependency(ZLIB 1.3 REQUIRED)
find_dependency(OpenSSL 3.0 COMPONENTS SSL Crypto)
find_dependency(fmt CONFIG)
