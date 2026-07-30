cmake_minimum_required(VERSION 3.24)
project(find_package_search LANGUAGES CXX)

find_package(ZLIB MODULE REQUIRED)
find_package(MyLibrary CONFIG REQUIRED
  NAMES MyLibrary my-library
  HINTS "${CMAKE_CURRENT_LIST_DIR}/third_party/install"
  PATH_SUFFIXES lib/cmake/MyLibrary
  NO_DEFAULT_PATH)
find_package(nlohmann_json 3.11...<4 CONFIG QUIET GLOBAL)
