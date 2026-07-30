from conan import ConanFile


class Overrides(ConanFile):
    name = "overrides"
    version = "1.0"

    def requirements(self):
        self.requires("zlib/1.3.1", force=True)
        self.requires("openssl/3.3.1", override=True)
