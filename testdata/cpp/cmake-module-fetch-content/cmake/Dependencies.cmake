include(FetchContent)
FetchContent_Declare(
  fmt
  GIT_REPOSITORY https://github.com/fmtlib/fmt.git
  GIT_TAG 10.2.1
  GIT_SHALLOW TRUE
)
FetchContent_Declare(
  doctest
  URL https://github.com/doctest/doctest/archive/refs/tags/v2.4.11.tar.gz
  URL_HASH SHA256=632ed2f6f78c47659c0ba6b85f15b96f9c147d0d4db7620d3810cb4a74a9e4f4
)
FetchContent_MakeAvailable(fmt doctest)
