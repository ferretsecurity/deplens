from conan import ConanFile


class PythonRequires(ConanFile):
    name = "python-requires"
    version = "1.0"
    python_requires = "base_recipes/1.2@company/stable", "packaging_tools/2.0@company/stable"
    python_requires_extend = "base_recipes.BaseConanfile", "packaging_tools.CMakeLayout"
    requires = "fmt/10.2.1"
