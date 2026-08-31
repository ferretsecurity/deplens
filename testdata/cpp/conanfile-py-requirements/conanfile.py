from conan import ConanFile


class LibqasmConan(ConanFile):
    def build_requirements(self):
        self.tool_requires("tree-gen/1.0.9")
        self.tool_requires("zulu-openjdk/21.0.1")
        self.tool_requires("emsdk/3.1.50")
        self.test_requires("gtest/1.15.0")

    def requirements(self):
        self.requires("fmt/11.0.2", transitive_headers=True)
        self.requires("range-v3/0.12.0", transitive_headers=True)
        self.requires("tree-gen/1.0.9", transitive_headers=True, transitive_libs=True)
        self.requires("antlr4-cppruntime/4.13.1", transitive_headers=True)
