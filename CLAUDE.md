# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this project does

`deplens` is a Go CLI that walks a directory tree, identifies dependency manifest files (60+ formats across 23+ ecosystems), extracts structured dependency data from them, and prints results. It makes no network calls and does no vulnerability scanning.

## Commands

```bash
go build ./cmd/deplens          # build binary
go run ./cmd/deplens <path>     # run against a directory
go run ./cmd/deplens --json <path>    # JSON output
go run ./cmd/deplens --rules custom.yaml <path>  # override default ruleset
go test ./...                   # all tests
go test ./internal/analyze/     # analyze package only
go test -run TestGoMod ./internal/analyze/       # single test
go vet ./...                    # lint
```

For architecture, data flow, core types, parser patterns, and testing conventions: see [ARCHITECTURE.md](ARCHITECTURE.md).

## Adding a new detector

1. If the format is TOML/YAML/JSON/XML/INI: add a rule to `default_rules.yaml` only (no Go needed)
2. For custom extraction logic:
   a. Create `internal/analyze/<name>.go` implementing `manifestParser`
   b. Add `<name>MatcherConfig`, parser struct, `new<Name>` constructor
   c. Register in `parser_factory.go` (nil-check + constructor call; increment `parserCount`)
   d. Add config field to `ruleConfig` in `rules.go`
   e. Add rule to `default_rules.yaml`
3. Add fixture files under `testdata/<ecosystem>/`
4. Add unit tests in `<name>_test.go` and scan integration tests in `<name>_scan_test.go`
5. Update `README.md` with detector info (required per `AGENTS.md`)

## PR requirements (from AGENTS.md)

- New detector: update `README.md` with detector info
- New default rule: add example in `testdata/`
- User-visible behavior change: include before/after CLI output examples in the PR description
