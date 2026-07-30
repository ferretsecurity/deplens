cmake_minimum_required(VERSION 3.20)
project(subdirectory_dependencies LANGUAGES CXX)

add_subdirectory(third_party/spdlog EXCLUDE_FROM_ALL)
add_subdirectory(vendor/catch2 "${CMAKE_BINARY_DIR}/vendor/catch2")
add_subdirectory("${CMAKE_CURRENT_SOURCE_DIR}/plugins/local-library")
