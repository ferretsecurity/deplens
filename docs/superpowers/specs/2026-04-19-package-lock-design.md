# package-lock.json detector design

## Summary

This change upgrades the existing `js-npm-lock` detector for `package-lock.json` from a filename-only match to a parser-backed extractor.

Version 1 will extract direct root dependency names from `package-lock.json` and report them as normalized dependency entries in existing `deplens` output. The detector will include root `optionalDependencies` alongside normal direct dependencies when the lockfile schema exposes them at the root package entry.

The goal is to improve typical user value across mixed-language repositories by turning a very common `[matched]` result into concrete extracted dependency output without adding transitive graph noise.

## Motivation

`package-lock.json` is one of the most common dependency artifacts in real repositories. In the current ruleset, `deplens` identifies it only by filename and reports it as `[matched]`, which gives low user value compared with detectors that extract concrete dependency names.

Upgrading this detector provides a high-impact improvement with moderate implementation cost because:

- npm lockfiles are common across repositories.
- The file format is JSON, which keeps parser complexity lower than custom text lockfiles.
- The existing codebase already has parser-backed detectors and JSON handling patterns that this work can follow.

## Goals

- Keep the existing manifest type name `js-npm-lock`.
- Upgrade `package-lock.json` to a level 3 detector that extracts dependency names.
- Extract direct root dependency names only.
- Include root `optionalDependencies` alongside root normal dependencies for lockfile formats that expose both.
- Treat empty supported root dependency sets as a conclusive empty manifest.
- Keep output compatible with the existing JSON and human-readable renderer.

## Non-goals

- Extracting the full transitive lock graph.
- Modeling npm dependency classes beyond the supported root dependency maps in v1.
- Emitting versions, sources, aliases, or workspace metadata.
- Changing CLI flags or output schema.
- Expanding `npm-shrinkwrap.json` in the same change unless it is explicitly scoped later.

## Supported behavior

### Lockfile v2 and v3

For lockfile versions that use the `packages` object, the detector will read the root package entry at `packages[""]`.

The extractor will:

- read keys from `packages[""].dependencies`
- read keys from `packages[""].optionalDependencies`
- union and deduplicate those keys
- sort the final dependency names for stable output

If both maps are absent or present but empty, the detector will report the manifest as conclusively empty with `has_dependencies=false`.

### Lockfile v1

For lockfile version 1, the detector will read dependency keys from the top-level `dependencies` object only.

The extractor will:

- read keys from the top-level `dependencies` object
- sort the final dependency names for stable output

If the `dependencies` object is absent or empty, the detector will report the manifest as conclusively empty with `has_dependencies=false`.

### Output shape

Dependencies will be emitted as standard `Dependency{Name: ...}` entries with names only.

Version strings will not be emitted in v1. This keeps the detector aligned with the current output model and avoids introducing lockfile-specific formatting decisions before the project defines a broader normalization strategy.

The detector will not split dependencies into output sections in v1. Standard and optional root dependencies will be merged into a single flat list.

## Error handling

- Malformed JSON remains an error for the scan, consistent with existing parser-backed detectors.
- Structurally valid JSON with missing supported root dependency maps is not an error; it is treated as a conclusive empty match.
- If the same dependency name appears in both supported maps, the output will contain it once.

## Design choices

### Direct dependencies only

The detector will extract only direct root dependencies rather than the full lockfile graph.

This is the recommended tradeoff because it keeps the output high-signal and comparable to existing detectors such as `go.mod`, which prefer direct dependency reporting over full transitive expansion.

### Dedicated parser

The change will add a dedicated `package-lock` parser instead of extending the generic JSON presence checker.

That keeps responsibilities clear:

- generic JSON presence rules remain level 2 presence checks
- `package-lock.json` gets lockfile-specific extraction logic
- future lockfile-specific JSON extractors can follow the same pattern

## Implementation outline

1. Add a new parser configuration type for `package-lock`.
2. Register the parser in [`internal/analyze/parser_factory.go`](/home/jekos/ghq/github.com/ferretsecurity/deplens/internal/analyze/parser_factory.go).
3. Add a dedicated analyzer implementation for `package-lock.json` extraction under [`internal/analyze`](/home/jekos/ghq/github.com/ferretsecurity/deplens/internal/analyze).
4. Update the `js-npm-lock` rule in [`internal/analyze/default_rules.yaml`](/home/jekos/ghq/github.com/ferretsecurity/deplens/internal/analyze/default_rules.yaml) to use the new parser.
5. Add unit tests for lockfile v1 and v2/v3 extraction behavior.
6. Add a CLI-level regression test showing that `package-lock.json` now reports extracted dependencies instead of `[matched]`.
7. Update [`README.md`](/home/jekos/ghq/github.com/ferretsecurity/deplens/README.md) because detector behavior is user-visible and the supported-detector table will change.

## Testing plan

The change should include unit tests for at least these cases:

- v1 lockfile with top-level `dependencies`
- v2 or v3 lockfile with `packages[""].dependencies`
- v2 or v3 lockfile with both `dependencies` and `optionalDependencies`
- empty supported root dependency maps
- deduplication when the same package appears in both supported maps
- malformed JSON

The change should also include one CLI-level assertion that confirms a detected `package-lock.json` renders with extracted dependencies in human-readable output.

## Risks and mitigations

### Risk: schema variance across npm lockfile versions

Mitigation:

- explicitly support only the root fields described in this design
- keep unsupported structures out of scope for v1
- cover v1 and v2/v3 with dedicated tests

### Risk: over-reporting transitive packages

Mitigation:

- extract only root direct dependencies
- avoid traversing nested dependency objects in v1

### Risk: ambiguous handling of optional dependencies

Mitigation:

- define the rule explicitly in v1: root `optionalDependencies` are included alongside root normal dependencies for lockfile formats that expose them at the root package entry
- merge and deduplicate the final names into one output list

## Success criteria

This design is complete when:

- `package-lock.json` remains reported as `js-npm-lock`
- direct root dependency names are extracted for supported lockfile schemas
- root optional dependencies are included for v2/v3 root package entries
- empty supported root dependency sets produce `has_dependencies=false`
- human-readable output shows extracted dependencies instead of `[matched]`
- README documentation reflects the new detector capability
