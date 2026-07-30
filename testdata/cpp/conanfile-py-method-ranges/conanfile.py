from conan import ConanFile


class MethodRanges(ConanFile):
    name = "method-ranges"
    version = "1.0"

    def requirements(self):
        self.requires("zlib/1.3.1")
        self.requires("fmt/[>=10.0 <11]")
        self.requires("nlohmann_json/[>=3.11 <4]")
        self.requires("spdlog/[~1.14]")
        self.requires("experimental/[>=2.0 <3.0, include_prerelease]")
