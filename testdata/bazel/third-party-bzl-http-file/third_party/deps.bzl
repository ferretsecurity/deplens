load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_file")

def third_party_dependencies():
    http_file(
        name = "protoc_binary",
        urls = ["https://github.com/protocolbuffers/protobuf/releases/download/v28.3/protoc-28.3-linux-x86_64.zip"],
        sha256 = "b3c1d20e4f5a69788796a5b4c3d2e1f00112233445566778899aabbccddeeff0",
        downloaded_file_path = "protoc.zip",
        executable = False,
    )
