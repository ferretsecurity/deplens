# setup.cfg detector design

## Summary

Add support for detecting Python dependencies declared in `setup.cfg`, specifically the declarative setuptools form that mirrors `setup.py`.

The first version is intentionally narrow:

- Match `setup.cfg` files.
- Recognize dependency declarations under `[options]` keys `setup_requires` and `install_requires`.
- Recognize dependency declarations under all keys in `[options.extras_require]`.
- Extract dependencies only from straightforward static multiline values.
- Match the manifest even when targeted keys are present but extraction is not possible.

## Goals

- Report `setup.cfg` as a Python dependency manifest when declarative dependency keys are present.
- Extract dependency strings from common static multiline declarations.
- Keep behavior predictable and easy to explain.
- Fit the existing `deplens` rule model and testdata conventions.

## Non-goals

- Support `file:` indirection.
- Support `%()` interpolation.
- Support single-line comma-separated dependency lists.
- Support arbitrary INI parsing beyond what is needed for `setup.cfg`.
- Normalize or parse Python requirement syntax beyond trimming and comment removal.

## User-visible behavior

Add a built-in detector:

```yaml
- name: python-setup-cfg
  filename-regex: '^setup\.cfg$'
  ini:
    queries:
      - section: options
        key: setup_requires
      - section: options
        key: install_requires
      - section: options.extras_require
        key: '*'
```

Expected behavior:

- A `setup.cfg` file matches when any query target exists.
- Extracted dependencies may be empty.
- Supported extracted values are multiline lists written as indented continuation lines after `key =`.
- Blank lines are ignored.
- Comment-only lines are ignored.
- Inline comments are stripped before returning a dependency.
- Unsupported value forms do not prevent a match.

Examples:

```ini
[options]
install_requires =
    requests>=2.31
    urllib3<3
```

Returns:

```text
match: yes
dependencies:
- requests>=2.31
- urllib3<3
```

```ini
[options]
install_requires =
    requests>=2.31  # runtime client
    urllib3<3
```

Returns:

```text
match: yes
dependencies:
- requests>=2.31
- urllib3<3
```

```ini
[options]
install_requires = requests>=2.31, urllib3<3
```

Returns:

```text
match: yes
dependencies: none
```

```ini
[options]
install_requires =
    file: requirements.txt
```

Returns:

```text
match: yes
dependencies: none
```

## Parser design

Introduce a new parser family:

- `ini` in rule config
- `newINIQueryParser(...)` in `internal/analyze`

Suggested config types:

```go
type iniMatcherConfig struct {
    Queries []iniQueryConfig `yaml:"queries"`
}

type iniQueryConfig struct {
    Section string `yaml:"section"`
    Key     string `yaml:"key"`
}
```

Validation rules:

- `ini.queries` must contain at least one entry.
- `section` is required.
- `key` is required.
- `key: '*'` is allowed to mean every key in the named section.
- Wildcards are only allowed as the full key value, not partial patterns.

## Parsing model

The parser only needs a narrow INI feature set:

- Sections in `[name]` form.
- Key-value assignments in `key = value` form.
- Continuation lines for multiline values via indentation.
- Comment markers `#` and `;`.

Suggested internal model:

- Read file line by line.
- Track current section.
- Record whether any query target has been seen.
- For each query hit, inspect the raw value:
  - If the first line after `=` is empty and continuation lines follow, treat it as a candidate multiline list.
  - If the assignment has a non-empty inline value on the same line, treat it as unsupported for v1.
  - For wildcard section queries, inspect every key in that section.
- For candidate multiline lists:
  - Iterate continuation lines.
  - Strip inline comments.
  - Trim whitespace.
  - Ignore empty results.
  - Skip entries that begin with `file:` after trimming.
  - Skip entries that begin with `%(` after trimming.
  - Append the remaining values as dependencies.

Match semantics:

- `ok = true` if any query target key exists, even when no dependencies are extracted.
- `dependencies` contains all extracted strings from supported values.
- Return an error only for invalid rule configuration or malformed INI that prevents safe parsing.

## Architecture impact

Changes are localized:

- Extend `ruleConfig` in `internal/analyze/rules.go` with `INI *iniMatcherConfig`.
- Update parser compilation to allow `ini` as another mutually-exclusive parser family.
- Add `internal/analyze/ini.go`.
- Add tests in `internal/analyze`.
- Add a default built-in rule in `internal/analyze/default_rules.yaml`.
- Update `README.md` to list the new detector and example output.
- Add `testdata/python/setup-cfg-*` samples because a new default rule is being added.

This stays aligned with the repository’s existing pattern of dedicated lightweight parsers per manifest family.

## Edge cases to cover

Supported extraction:

- `[options] install_requires`
- `[options] setup_requires`
- multiple extras under `[options.extras_require]`
- environment markers
- requirement extras like `requests[socks]>=2.31`
- blank lines and comment lines inside multiline blocks
- inline comments after a dependency

Presence-only match with no extracted dependencies:

- empty multiline block
- comment-only multiline block
- inline single-line value such as `install_requires = requests>=2.31, urllib3<3`
- `file:` indirection
- `%()` interpolation
- mixed supported and unsupported entries in the same block

Out of scope:

- nested interpolation behavior
- preserving comment text
- full packaging-spec aware tokenization

## Testing strategy

Unit tests:

- rule validation for valid and invalid `ini` config
- parser matches on key presence
- parser extracts supported multiline values
- wildcard extraction from `[options.extras_require]`
- parser strips comments and ignores blanks
- parser returns match with empty dependencies for unsupported values
- parser errors on malformed INI only when structure is irrecoverable

Testdata samples:

- `testdata/python/setup-cfg-install-requires/setup.cfg`
- `testdata/python/setup-cfg-setup-requires/setup.cfg`
- `testdata/python/setup-cfg-extras-require/setup.cfg`
- `testdata/python/setup-cfg-mixed/setup.cfg`
- `testdata/python/setup-cfg-comments-and-blanks/setup.cfg`
- `testdata/python/setup-cfg-inline-comma-unsupported/setup.cfg`
- `testdata/python/setup-cfg-file-unsupported/setup.cfg`
- `testdata/python/setup-cfg-interpolation-unsupported/setup.cfg`

The `testdata` fixtures supplement unit tests rather than replace them.

## README updates

Update the supported detectors table to mention the `ini` parser and the built-in `python-setup-cfg` detector.

Add a short example alongside the existing `setup.py` example showing:

```text
python-setup-cfg
- setup.cfg
  - requests>=2.31
  - pytest>=8
```

## Risks and tradeoffs

Benefits of the narrow scope:

- low implementation cost
- easy to reason about
- low risk of extracting incorrect dependency strings from complex declarative features

Tradeoff:

- some real-world `setup.cfg` files will match without producing extracted dependencies

That tradeoff is acceptable for v1 because it still improves manifest discovery while keeping extraction behavior conservative.

## Open decisions resolved

- Key presence implies manifest match by default.
- No explicit `match_on_presence` config field is needed.
- Comment text should be stripped from extracted dependency lines.
- No explicit `literal` field is needed until the `ini` parser supports more than one extraction mode.
