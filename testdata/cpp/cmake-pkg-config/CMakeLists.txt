cmake_minimum_required(VERSION 3.20)
project(pkg_config_dependencies LANGUAGES CXX)

find_package(PkgConfig REQUIRED)
pkg_check_modules(GLIB REQUIRED IMPORTED_TARGET GLOBAL glib-2.0>=2.76 gio-2.0)
pkg_search_module(XML REQUIRED IMPORTED_TARGET libxml-2.0 libxml2)
