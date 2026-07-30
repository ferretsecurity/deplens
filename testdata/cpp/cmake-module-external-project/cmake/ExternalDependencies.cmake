include(ExternalProject)

ExternalProject_Add(private_codec
  GIT_REPOSITORY https://github.com/example/private-codec.git
  GIT_TAG 0123456789abcdef0123456789abcdef01234567
  GIT_SHALLOW TRUE
  UPDATE_DISCONNECTED TRUE
)

ExternalProject_Add(header_only_tools
  URL https://downloads.example.test/header-only-tools-2.0.0.tar.xz
  URL_HASH SHA256=632ed2f6f78c47659c0ba6b85f15b96f9c147d0d4db7620d3810cb4a74a9e4f4
)
