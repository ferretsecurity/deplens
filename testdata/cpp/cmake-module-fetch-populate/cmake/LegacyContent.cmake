include(FetchContent)

FetchContent_Declare(legacy_fmt
  GIT_REPOSITORY https://github.com/fmtlib/fmt.git
  GIT_TAG 0123456789abcdef0123456789abcdef01234567
)
FetchContent_GetProperties(legacy_fmt)
if(NOT legacy_fmt_POPULATED)
  FetchContent_Populate(legacy_fmt)
  add_subdirectory(${legacy_fmt_SOURCE_DIR} ${legacy_fmt_BINARY_DIR})
endif()
