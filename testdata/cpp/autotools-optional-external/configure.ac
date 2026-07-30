AC_INIT([optional_external], [1.0])
AC_CONFIG_SRCDIR([configure.ac])
AC_PROG_CC
AC_ARG_WITH([readline],
  [AS_HELP_STRING([--with-readline], [enable readline support])],
  [],
  [with_readline=yes])
AS_IF([test "x$with_readline" != xno], [
  AC_CHECK_HEADERS([readline/readline.h])
  AC_CHECK_LIB([readline], [readline],
    [AC_DEFINE([HAVE_READLINE], [1], [Define if readline is available])],
    [AC_MSG_FAILURE([--with-readline was given, but readline is unavailable])],
    [-lncurses])
])
AC_ARG_ENABLE([tracing],
  [AS_HELP_STRING([--enable-tracing], [enable optional tracing support])],
  [enable_tracing=$enableval],
  [enable_tracing=no])
AC_ARG_WITH([foo],
  [AS_HELP_STRING([--with-foo=DIR], [use Foo installed under DIR])],
  [],
  [with_foo=no])
AS_IF([test "x$with_foo" != xno && test "x$with_foo" != xyes], [
  CPPFLAGS="-I$with_foo/include $CPPFLAGS"
  LDFLAGS="-L$with_foo/lib $LDFLAGS"
])
AC_CHECK_HEADER([foo/foo.h], [have_foo_header=yes], [have_foo_header=no])
AC_CHECK_LIB([foo], [foo_init], [have_foo_library=yes], [have_foo_library=no])
AC_OUTPUT
