load("@bazel_tools//tools/build_defs/repo:git.bzl", "git_repository")

def third_party_dependencies():
    git_repository(
        name = "private_rules",
        remote = "git@github.com:example/private-rules.git",
        commit = "0123456789abcdef0123456789abcdef01234567",
        shallow_since = "2025-01-01T00:00:00Z",
    )
    git_repository(
        name = "tagged_rules",
        remote = "https://github.com/example/tagged-rules.git",
        tag = "v2.0.0",
    )
    native.local_repository(
        name = "company_sdk",
        path = "../company-sdk",
    )
    native.new_local_repository(
        name = "legacy_sdk",
        path = "../legacy-sdk",
        build_file_content = "cc_library(name = \"legacy_sdk\")",
    )
