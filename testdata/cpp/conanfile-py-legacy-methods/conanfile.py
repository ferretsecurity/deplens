from conans import ConanFile


class LegacyMethods(ConanFile):
    name = "legacy-methods"
    version = "1.0"
    settings = "os", "arch", "compiler", "build_type"

    def requirements(self):
        self.requires("boost/1.82.0@conan/stable")
        if self.settings.os == "Windows":
            self.requires("winpthreads/1.0@conan/stable")

    def build_requirements(self):
        self.build_requires("cmake/3.22.6@conan/stable")
        self.build_requires("gtest/1.11.0@conan/stable", force_host_context=True)
