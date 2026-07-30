cmake_minimum_required(VERSION 3.20)
project(find_package_versions LANGUAGES CXX)

find_package(Boost 1.82 REQUIRED COMPONENTS filesystem program_options)
find_package(fmt 10.2 EXACT CONFIG REQUIRED)
find_package(Catch2 3.5 QUIET OPTIONAL_COMPONENTS Catch2WithMain)
