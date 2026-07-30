cmake_minimum_required(VERSION 3.24)
project(fetchcontent_git LANGUAGES CXX)

include(FetchContent)
FetchContent_Declare(
  fmt
  GIT_REPOSITORY https://github.com/fmtlib/fmt.git
  GIT_TAG 10.2.1
  FIND_PACKAGE_ARGS CONFIG)
FetchContent_Declare(
  googletest
  GIT_REPOSITORY https://github.com/google/googletest.git
  GIT_TAG v1.14.0
  OVERRIDE_FIND_PACKAGE)
FetchContent_MakeAvailable(fmt googletest)
