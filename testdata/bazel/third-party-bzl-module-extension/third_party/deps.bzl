load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive")

def _third_party_impl(module_ctx):
    http_archive(
        name = "tooling_bundle",
        urls = ["https://downloads.example.test/tooling-bundle-1.0.0.tar.gz"],
        sha256 = "d4e5f60718293a4b5c6d7e8f90112233445566778899aabbccddeeff00112233",
    )

third_party = module_extension(
    implementation = _third_party_impl,
)
