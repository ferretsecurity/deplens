AC_INIT([search_libraries], [1.0])
AC_CONFIG_SRCDIR([configure.ac])
AC_PROG_CC
AC_SEARCH_LIBS([dlopen], [dl dld], [], [AC_MSG_ERROR([dynamic loader library is required])])
AC_SEARCH_LIBS([gethostbyname], [nsl socket], [], [], [-lresolv])
AC_CHECK_FUNCS([clock_gettime pthread_create])
AC_OUTPUT
