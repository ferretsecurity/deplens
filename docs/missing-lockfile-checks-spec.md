# Specification: Go, Composer, and Bundler missing-lockfile checks

## Purpose

Add three offline policy checks that report a dependency-bearing application when
its project-owned lock or checksum file is absent:

| Check ID | Evaluator type | Manifest | Required file |
| --- | --- | --- | --- |
| `go-sum-missing` | `go-sum-missing` | `go.mod` | `go.sum` |
| `php-composer-lockfile-missing-for-application` | `composer-application-lockfile-missing` | `composer.json` | `composer.lock` |
| `ruby-gemfile-lockfile-missing-for-application` | `gemfile-application-lockfile-missing` | `Gemfile` | `Gemfile.lock` |

The checks are advisory. They do not change the default CLI exit status and make
no network or package-manager calls.

## Shared behavior

- A lock or checksum file satisfies only the manifest in the same directory.
  An ancestor, descendant, or sibling file does not satisfy a project.
- Findings use the existing missing-lockfile shape:
  - `subject.project_root` is `.` for the scan root, otherwise the normalized
    root-relative project directory.
  - `locations` contains exactly the manifest path.
  - `evidence.manager` identifies the ecosystem manager (`go`, `composer`, or
    `bundler`) and `evidence.expected_lockfile` contains the required filename.
  - The fingerprint is generated through `newMissingLockfileFinding`; it must
    remain independent of summary, severity, remediation, and location wording.
- Checks run only when their manifest is a recognized, dependency-bearing
  candidate. A manifest with no relevant dependencies emits neither a finding
  nor a completed check run.
- A malformed policy manifest produces one `failed` check run with
  `reason_code: source-analysis-failed`, no finding, and the parser diagnostic
  in `detail`.
- A check may emit `skipped` runs only for the explicit reason codes in this
  document. Skips never emit findings.
- Results retain the existing stable ordering: project root, then check ID.

## 1. `go-sum-missing`

### Eligibility

The evaluator considers each recognized `go-mod` source independently. `go.work`
does not create an ownership relationship for this check: every Go module owns
its own `go.sum`.

A module is eligible when its parsed `go.mod` contains at least one `require`
entry that is not replaced by a local filesystem module. Both direct and
`// indirect` requirements count.

A replacement is local when the right-hand side is an absolute filesystem path,
starts with `./` or `../`, or is `.` or `..`. A local replacement suppresses only
the matching required module/version; other external requirements keep the
module eligible. Replacements to another module version or to a module path are
external.

`replace`, `exclude`, `retract`, toolchain, and Go-version directives alone do
not make a module dependency-bearing.

### Evaluation

For an eligible `dir/go.mod`, check for `dir/go.sum` in `sourceByPath`.

- Present: emit one `completed` run; no finding.
- Absent: emit one `completed` run and a finding.
- All requirements are locally replaced: emit one `skipped` run with
  `reason_code: local-dependencies-only` and detail
  `all required Go modules are replaced by local filesystem paths`.

The implementation must retain `go.mod` bytes for policy parsing, add a Go
module policy model to `evaluationContext`, and use `golang.org/x/mod/modfile`
for the policy parse. It must not infer checksum-file presence from a
`go.work.sum` file.

### Finding

```yaml
id: go-sum-missing
summary: Go module has external dependencies but no go.sum
severity: medium
evidence:
  manager: go
  expected_lockfile: go.sum
remediation: Run `go mod tidy` (or `go mod download`) and commit go.sum.
```

## 2. `php-composer-lockfile-missing-for-application`

### Application classification

Composer libraries commonly and legitimately omit `composer.lock`. To avoid
flagging them, this evaluator considers a project an application only when
`composer.json` has a root-level string field exactly equal to `"project"` in
`type` (case-insensitive). Missing, non-string, or other values are not
applications.

For every recognized dependency-bearing `php-composer` source:

- malformed JSON: failed run;
- `config.lock: false`: skipped run with `reason_code: lockfile-disabled` and
  detail `composer.json disables lockfile generation with config.lock=false`;
- `type` missing or non-string: skipped run with
  `reason_code: project-role-unknown` and detail
  `Composer application role requires type: project`;
- `type` other than `project`: skipped run with `reason_code: not-application`
  and detail `Composer package type is not project`.

### Dependency classification

An application is dependency-bearing when either `require` or `require-dev`
contains at least one installable package. Ignore Composer platform requirements:

- `php`, `php-64bit`, `php-ipv6`, and `php-zts`;
- package names beginning `ext-` or `lib-` (case-insensitive).

All other non-empty package keys count, including path, VCS, and private
repository packages. An application with only platform requirements does not
run the check.

### Evaluation

For an eligible `dir/composer.json`, require `dir/composer.lock`.

- Present: one completed run; no finding.
- Absent: one completed run and a finding.

Composer has no standard workspace ownership model for this feature; each
`composer.json` is evaluated in its own directory.

The implementation must retain `php-composer` bytes and decode only the policy
fields needed (`type`, `config.lock`, `require`, and `require-dev`). It must not
use the generic JSON presence result as the final dependency decision because a
platform-only `require` section is not installable package evidence.

### Finding

```yaml
id: php-composer-lockfile-missing-for-application
summary: Composer application has dependencies but no composer.lock
severity: medium
evidence:
  manager: composer
  expected_lockfile: composer.lock
remediation: Run `composer update` and commit composer.lock.
```

## 3. `ruby-gemfile-lockfile-missing-for-application`

### Application classification

Bundler libraries often use a `Gemfile` only to develop a gem and should not be
required to commit `Gemfile.lock`. A `Gemfile` is treated as a Ruby application
only when all of the following are true:

1. It is named exactly `Gemfile` (not an Appraisal `*.gemfile`).
2. It contains at least one statically recognizable `gem` declaration.
3. It does not contain the Bundler `gemspec` directive.

Recognize a `gem` declaration only when, after leading whitespace, the line
begins with `gem` followed by whitespace or `(` and its first argument is a
literal quoted gem name. Comments and blank lines are ignored. The evaluator
does not need to extract a version constraint.

`gemspec` is recognized when, after leading whitespace, a non-comment line
begins with `gemspec` followed by whitespace, `(`, or end of line. This is the
explicit library signal. It takes precedence even when the Gemfile also declares
development gems.

Dynamic dependency declarations (`eval_gemfile`, a computed gem name, or an
unparseable `gem` call) do not establish eligibility. They do not cause a
finding by themselves.

### Evaluation

For a recognized Gemfile:

- no static gem declaration: no run;
- `gemspec` directive: one skipped run with `reason_code: not-application` and
  detail `Gemfile declares gemspec and is treated as a Ruby library`;
- otherwise require `Gemfile.lock` in the same directory:
  - present: one completed run; no finding;
  - absent: one completed run and a finding.

There is no Bundler workspace ownership in this feature. Each eligible Gemfile
owns only its sibling `Gemfile.lock`.

The implementation must retain `ruby-gemfile` bytes. It must not use the
selector-only source state (`unknown` / `unsupported`) as dependency evidence,
and it must not evaluate `ruby-appraisal` sources.

### Finding

```yaml
id: ruby-gemfile-lockfile-missing-for-application
summary: Ruby application has dependencies but no Gemfile.lock
severity: medium
evidence:
  manager: bundler
  expected_lockfile: Gemfile.lock
remediation: Run `bundle install` and commit Gemfile.lock.
```

## Configuration and implementation changes

1. Add the three evaluator types to `validEvaluatorType` and dispatch each from
   `evaluateChecks`.
2. Add the three default `checks` entries shown above. Their evaluator
   configurations are empty and remain strictly validated.
3. Extend `needsPolicyContent` for `go-mod`, `php-composer`, and `ruby-gemfile`.
4. Add policy parsing models and evaluator functions in `findings.go`; reuse
   `newMissingLockfileFinding` rather than inventing an output shape.
5. Update the README's policy-check list, evaluator-type list, and concrete
   behavior examples. Update `DEPENDENCY_COVERAGE.md` only if the generated
   inventory's check count or related wording is part of its generator output.

## Required tests and fixtures

Add focused fixtures under `testdata/findings/` and assertions in
`internal/analyze/findings_test.go` for at least the following cases.

| Ecosystem | Fixture case | Expected result |
| --- | --- | --- |
| Go | external `require`, no `go.sum` | completed + finding |
| Go | external `require`, sibling `go.sum` | completed, no finding |
| Go | no `require` | no run, no finding |
| Go | all required modules locally replaced | skipped `local-dependencies-only` |
| Go | one local replacement and one external requirement | completed + finding |
| Go | nested module with only ancestor `go.sum` | finding at nested module |
| Go | malformed `go.mod` | failed run, no finding |
| Composer | `type: project`, installable require, no lock | completed + finding |
| Composer | project with sibling lock | completed, no finding |
| Composer | project with only platform requirements | no run, no finding |
| Composer | `type: library` | skipped `not-application` |
| Composer | no `type` | skipped `project-role-unknown` |
| Composer | `config.lock: false` | skipped `lockfile-disabled` |
| Composer | malformed manifest | failed run, no finding |
| Ruby | Gemfile with static gem, no lock | completed + finding |
| Ruby | Gemfile with sibling lock | completed, no finding |
| Ruby | Gemfile with `gemspec` and gems | skipped `not-application` |
| Ruby | Gemfile with no static gems | no run, no finding |
| Ruby | Appraisal `.gemfile` | no run, no finding |

Also add rule-schema coverage confirming the three evaluator types are accepted
with empty configuration and reject unknown evaluator fields. Extend the default
rules metadata-count assertion from six checks to nine checks. Add a fingerprint
stability test for at least one new check.

## Acceptance criteria

- The default ruleset loads with 185 detectors and nine checks.
- Every positive fixture produces exactly one finding with the stated ID,
  manifest location, expected-lockfile evidence, and non-empty fingerprint.
- Satisfied, dependency-free, library, local-only, and disabled-lockfile cases
  never produce a finding.
- Malformed inputs report failed runs without preventing other projects from
  being scanned.
- No evaluator reads outside the scan root or invokes Go, Composer, Bundler, or
  the network.
