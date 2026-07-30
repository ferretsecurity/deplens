from conans import ConanFile


class LegacyBuildRequires(ConanFile):
    name = "legacy-build-requires"
    version = "1.0"
    requires = "boost/1.82.0@conan/stable"
    build_requires = "cmake/3.22.6@conan/stable", "ninja/1.11.1@conan/stable"
