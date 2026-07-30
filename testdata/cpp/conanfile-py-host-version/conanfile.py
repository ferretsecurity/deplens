from conan import ConanFile


class HostVersion(ConanFile):
    name = "host-version"
    version = "1.0"

    def requirements(self):
        self.requires("protobuf/5.27.0")
        self.requires("gettext/0.22.5")

    def build_requirements(self):
        self.tool_requires("protobuf/<host_version>")
        self.tool_requires("libgettext/<host_version:gettext>")
