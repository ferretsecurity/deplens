load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive")

def third_party_dependencies():
    http_archive(
        name = "patched_library",
        urls = [
            "https://mirror.example.test/patched-library-2.0.0.tar.xz",
            "https://upstream.example.test/releases/patched-library-2.0.0.tar.xz",
        ],
        integrity = "sha256-Y5JjkKe6BN8s7K2yvkb+PvEEFe9qfLFUZxUCyoThqD4=",
        strip_prefix = "patched-library-2.0.0",
        patches = ["//third_party:patched-library.patch"],
        patch_args = ["-p1"],
        build_file_content = "exports_files([\"include/library.h\"])",
    )
