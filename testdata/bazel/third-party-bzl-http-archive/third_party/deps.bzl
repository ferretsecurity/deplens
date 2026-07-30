load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive")

def third_party_dependencies():
    http_archive(
        name = "rules_cc",
        urls = ["https://github.com/bazelbuild/rules_cc/archive/refs/tags/0.1.1.tar.gz"],
        sha256 = "c9526390a7cd420fdcec2988b4f3626fe9c5b51e2959f685e8f4d170d1a9bd96",
        strip_prefix = "rules_cc-0.1.1",
    )
