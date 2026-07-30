load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_jar")

def third_party_dependencies():
    http_jar(
        name = "checker_qual",
        urls = ["https://repo1.maven.org/maven2/org/checkerframework/checker-qual/3.47.0/checker-qual-3.47.0.jar"],
        sha256 = "70ef35f6027286d8b0a5c9e9da23a1d9f9e4c2b3a8d7e6f5c4b3a29181726354",
    )
