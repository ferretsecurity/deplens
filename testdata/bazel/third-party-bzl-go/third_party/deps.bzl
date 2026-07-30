load("@bazel_gazelle//:deps.bzl", "go_repository")

def third_party_dependencies():
    go_repository(
        name = "com_github_google_uuid",
        importpath = "github.com/google/uuid",
        version = "v1.6.0",
        sum = "h1:NIvaJDMOsjVaqfq5e0bE6Qf6B/zmwbT7d4lY/1Y7jEw=",
    )
    go_repository(
        name = "com_github_test_fixture",
        importpath = "github.com/example/test-fixture",
        commit = "0123456789abcdef0123456789abcdef01234567",
        remote = "https://github.com/example/test-fixture.git",
    )
