name "edge"

cookbook "local", path: "."
cookbook "helpers", ">= 1.2.0", git: "https://example.test/helpers.git", tag: "v1.2.3"
