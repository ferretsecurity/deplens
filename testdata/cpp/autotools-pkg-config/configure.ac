AC_INIT([pkg_config_dependencies], [1.0])
AC_CONFIG_SRCDIR([configure.ac])
PKG_PROG_PKG_CONFIG
PKG_CHECK_MODULES([GLIB], [glib-2.0 >= 2.76 gio-2.0])
PKG_CHECK_MODULES([JSON], [json-c >= 0.17], [], [AC_MSG_ERROR([json-c is required])])
PKG_CHECK_MODULES_STATIC([ARCHIVE], [libarchive >= 3.7])
PKG_CHECK_EXISTS([libcurl >= 8.0], [have_curl=yes], [have_curl=no])
AC_OUTPUT
