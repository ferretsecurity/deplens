AC_INIT([headers_libraries], [1.0])
AC_CONFIG_SRCDIR([configure.ac])
AC_PROG_CC
AC_CHECK_HEADERS([zlib.h openssl/ssl.h sqlite3.h], [], [AC_MSG_ERROR([required header missing])])
AC_CHECK_HEADER([curl/curl.h], [have_curl_header=yes], [have_curl_header=no], [[#include <stddef.h>]])
AC_CHECK_LIB([z], [inflate])
AC_CHECK_LIB([ssl], [SSL_new], [], [AC_MSG_ERROR([OpenSSL is required])], [-lcrypto])
AC_CHECK_FUNC([strlcpy], [have_strlcpy=yes], [have_strlcpy=no])
AC_OUTPUT
