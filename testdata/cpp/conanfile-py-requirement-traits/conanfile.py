from conan import ConanFile


class RequirementTraits(ConanFile):
    name = "requirement-traits"
    version = "1.0"

    def requirements(self):
        self.requires("rapidjson/1.1.0", headers=True, libs=False, transitive_headers=True)
        self.requires("openssl/3.3.1", run=True, transitive_libs=True)
        self.requires("protobuf/5.27.0", options={"shared": False})
        self.requires("private-implementation/1.0", visible=False, no_skip=True)
