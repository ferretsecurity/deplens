from conan import ConanFile


class AttributeScopes(ConanFile):
    name = "attribute-scopes"
    version = "1.0"
    requires = "fmt/10.2.1", "boost/1.84.0"
    tool_requires = "cmake/3.29.0", "ninja/1.12.1"
    test_requires = "gtest/1.14.0"
