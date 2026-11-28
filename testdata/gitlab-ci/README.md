# GitLab CI container fixtures

These small fixtures preserve the declarations from the sweep's pinned examples:

- `.gitlab/doxygen.yml`: [KiCad job image and daemon service](https://gitlab.com/kicad/code/kicad/-/blob/54914b100d8e0cafc77de10f0fbe3203373bcfdd/.gitlab/doxygen.yml#L2).
- `.gitlab/ci/jobs.gitlab-ci.yml`: [Dependabot hidden templates, mapping-form services, and dynamic images](https://gitlab.com/dependabot-gitlab/dependabot/-/blob/0df965bb19376ba5e0ae1a78507e37b13f15da21/.gitlab/ci/jobs.gitlab-ci.yml#L43).
- `build/build.gitlab-ci.yml`: [Versionlens top-level image and service with the same value](https://gitlab.com/versionlens/vscode-versionlens/-/blob/2153292334c6affbaa6489531b77d3a826b7ec73/build/build.gitlab-ci.yml#L1).

`.gitlab-ci.yml` adds synthetic default, mapping-image, and local-include declarations. The scan test asserts each source path, declaring field, literal image, extraction state, package type, relationship, and scope. The analyzer tests cover unsupported declarations and image normalization.

Run `go test ./internal/analyze -run TestGitlabCI` or `go run ./cmd/deplens --json testdata/gitlab-ci`.
