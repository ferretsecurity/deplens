cmake_minimum_required(VERSION 3.11)
project(fetchcontent_populate LANGUAGES CXX)

include(FetchContent)
FetchContent_Declare(
  rapidjson
  GIT_REPOSITORY https://github.com/Tencent/rapidjson.git
  GIT_TAG v1.1.0)
FetchContent_GetProperties(rapidjson)
if(NOT rapidjson_POPULATED)
  FetchContent_Populate(rapidjson)
  add_subdirectory("${rapidjson_SOURCE_DIR}" "${rapidjson_BINARY_DIR}" EXCLUDE_FROM_ALL)
endif()
