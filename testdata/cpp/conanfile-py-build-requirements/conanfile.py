from conan import ConanFile


class BuildRequirements(ConanFile):
    name = "build-requirements"
    version = "1.0"
    settings = "os", "arch", "compiler", "build_type"

    def build_requirements(self):
        self.tool_requires("cmake/3.29.0", options={"shared": False})
        self.tool_requires("ninja/1.12.1", package_id_mode="minor_mode")
        self.test_requires("gtest/1.14.0", force=True)
        if self.settings.os == "Windows":
            self.tool_requires("nasm/2.16.01", override=True)
