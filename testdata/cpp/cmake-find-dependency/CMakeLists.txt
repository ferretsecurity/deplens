cmake_minimum_required(VERSION 3.20)
project(transitive_dependency_config LANGUAGES CXX)

include(CMakeFindDependencyMacro)
find_dependency(fmt 10.2 CONFIG REQUIRED)
find_dependency(ZLIB 1.3 REQUIRED)
