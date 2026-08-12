#
# Copyright (c) 2026 Mike Aldred
# All rights reserved.
#
# Permission to use, copy, modify, and distribute this software for any
# purpose with or without fee is hereby granted, provided that the above
# copyright notice and this permission notice appear in all copies.
#
# THE SOFTWARE IS PROVIDED "AS IS" AND THE AUTHOR DISCLAIMS ALL WARRANTIES
# WITH REGARD TO THIS SOFTWARE INCLUDING ALL IMPLIED WARRANTIES OF
# MERCHANTABILITY AND FITNESS. IN NO EVENT SHALL THE AUTHOR BE LIABLE FOR
# ANY SPECIAL, DIRECT, INDIRECT, OR CONSEQUENTIAL DAMAGES OR ANY DAMAGES
# WHATSOEVER RESULTING FROM LOSS OF USE, DATA OR PROFITS, WHETHER IN AN
# ACTION OF CONTRACT, NEGLIGENCE OR OTHER TORTIOUS ACTION, ARISING OUT OF
# OR IN CONNECTION WITH THE USE OR PERFORMANCE OF THIS SOFTWARE.
#

AC_PREREQ([2.60])
AC_INIT([xf86-video-illumosfb],
        [0.1.0],
        [https://github.com/aldredmr/xf86-video-illumosfb/issues],
        [xf86-video-illumosfb])

AC_CONFIG_SRCDIR([Makefile.am])
AC_CONFIG_HEADERS([config.h])
AC_CONFIG_AUX_DIR(.)

AM_INIT_AUTOMAKE([foreign dist-xz])
AM_MAINTAINER_MODE([enable])

# Require xorg-macros 1.8 or later: sets up CWARNFLAGS and man page macros
m4_ifndef([XORG_MACROS_VERSION],
          [m4_fatal([must install xorg-macros 1.8 or later before running autoreconf])])
XORG_MACROS_VERSION(1.8)
XORG_DEFAULT_OPTIONS

AC_PROG_CC
LT_INIT([disable-static])

AH_TOP([#include "xorg-server.h"])

# Determine Xorg module install directory
AC_ARG_WITH(xorg-module-dir,
            AS_HELP_STRING([--with-xorg-module-dir=DIR],
                           [Default xorg module directory [[default=$libdir/xorg/modules]]]),
            [moduledir="$withval"],
            [moduledir="$libdir/xorg/modules"])
AC_SUBST([moduledir])

# Check for required Xorg packages
PKG_CHECK_MODULES(XORG, [xorg-server >= 1.18 xproto fontsproto])

# Substitutions for the man page
DRIVER_NAME=illumosfb
AC_SUBST([DRIVER_NAME])

AC_CONFIG_FILES([
    Makefile
    src/Makefile
    man/Makefile
])
AC_OUTPUT
