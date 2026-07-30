cmake_minimum_required(VERSION 3.20)
project(external_project LANGUAGES CXX)

include(ExternalProject)
ExternalProject_Add(zlib_external
  GIT_REPOSITORY https://github.com/madler/zlib.git
  GIT_TAG v1.3.1
  GIT_SHALLOW TRUE
  CMAKE_ARGS -DCMAKE_POSITION_INDEPENDENT_CODE=ON
  INSTALL_COMMAND "")
ExternalProject_Add(cxxopts_external
  URL https://github.com/jarro2783/cxxopts/archive/v2.2.0.tar.gz
  URL_HASH SHA256=447dbfc2361fce9742c5d1c9cfb25731c977b405f9085a738fbd608626da8a4d
  CONFIGURE_COMMAND ""
  BUILD_COMMAND ""
  INSTALL_COMMAND "")
ExternalProject_Add(apr_svn_external
  SVN_REPOSITORY https://svn.apache.org/repos/asf/apr/apr/tags/1.7.5
  CONFIGURE_COMMAND ""
  BUILD_COMMAND ""
  INSTALL_COMMAND "")
