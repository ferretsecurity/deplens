# Glossary Design

## Summary

Add a project glossary as descriptive documentation so contributors, users, and coding agents use the same terms when discussing `deplens`. The glossary will live in `docs/glossary.md` and will be linked from the main `README.md`.

The glossary is reference material, not process policy. It defines the canonical user-facing terms used in documentation and CLI-facing explanations, while also clarifying how internal concepts map onto that language.

## Goals

- Reduce terminology drift across the README, future docs, tests, PR descriptions, and implementation discussions.
- Give users a stable reference for terms that appear in CLI output and JSON output.
- Give contributors and coding agents a shared vocabulary for internal concepts without forcing them to read code to infer meaning.

## Non-Goals

- The glossary will not enforce contributor behavior or introduce writing rules.
- The glossary will not duplicate detailed detector documentation already covered in the README.
- The glossary will not attempt to define generic language ecosystem terms unless `deplens` gives them a project-specific meaning.

## Canonical Language Model

User-facing language is the source of truth. When code, tests, or implementation conversations use shorthand or internal terminology, the glossary should map those terms back to the canonical user-visible wording instead of the other way around.

This keeps the README, CLI behavior descriptions, and future examples aligned around the same vocabulary.

## Scope

The glossary should have an average scope: roughly 15 to 20 entries. That is large enough to cover the main concepts and output semantics without turning the glossary into a second README.

The intended coverage is:

- Core project concepts such as manifest, detector, and rule.
- Detection and extraction terms such as match, extracted dependency, and section.
- Output semantics that affect interpretation, such as dependency status unknown, no dependencies, and dependencies present, not extracted.
- Selected JSON terms where field meaning is not obvious from the name alone, such as `has_dependencies` and `root`.

## Information Architecture

The glossary should be organized into short sections that let readers scan by topic:

- Core Concepts
- Detection and Extraction
- Output Semantics
- Configuration Terms

Each glossary entry should use:

- One canonical term as the heading
- A concise 2 to 4 sentence definition
- Optional mention of likely synonyms or internal shorthand when that helps prevent drift
- Clarification of both user-facing meaning and contributor-facing nuance when needed

## Initial Candidate Terms

The first version should likely include these terms:

- manifest
- detector
- rule
- built-in rule
- custom rule
- match
- extracted dependency
- dependency status
- dependency status unknown
- no dependencies
- dependencies present, not extracted
- empty manifest
- section
- human-readable output
- JSON output
- `has_dependencies`
- root
- path-first output

The exact final list can change slightly during authoring if a term proves redundant or another term is clearly more important, but the glossary should stay within the target scope.

## README Integration

The README should stay the main entry point for the project. It should gain a short terminology-oriented note near the top that links to `docs/glossary.md`.

The README should not mirror the glossary contents. It should only make the glossary discoverable and explain that the glossary defines project-specific terminology used in the tool's documentation and output descriptions.

## Writing Guidance

Definitions should be concrete and stable. They should describe how `deplens` uses a term today, not provide abstract dictionary-style explanations.

Where a term appears in output or structured data, the definition should explain what a user can infer from seeing that term. Where a term is mostly relevant to implementation, the definition should explain it in language that still makes sense to non-maintainers.

## Risks And Mitigations

Risk: the glossary becomes a dumping ground for every repeated noun in the README.
Mitigation: only include terms that are project-specific, user-visible, or likely to cause ambiguity.

Risk: glossary definitions drift from user-visible wording over time.
Mitigation: treat README and CLI-facing wording as canonical and update glossary entries to match those terms.

Risk: glossary duplicates detector reference material.
Mitigation: keep detector-specific mechanics in the README and use the glossary only for shared terminology.

## Open Decisions Resolved

- Location: `docs/glossary.md`, linked from `README.md`
- Scope: average, approximately 15 to 20 entries
- Audience: both users and contributors, including coding agents
- Tone: purely descriptive documentation
- Canonical wording: user-facing language
