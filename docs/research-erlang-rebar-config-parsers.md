# Research: parsing `rebar.config` in Go

## Conclusion

Do **not** add a Go parsing dependency for `erlang-rebar-config`. The only
purpose-built package found is itself a small handwritten parser with a
narrower grammar and a much weaker maintenance story than a dependency in
DepLens should have. The local parser is the simpler choice.

`rebar.config` is not a JSON- or YAML-like schema. Rebar3 reads it as a list
of Erlang terms, and `file:consult/1` defines those as terms separated by a
dot. [Rebar3 source](https://github.com/erlang/rebar3/blob/main/apps/rebar/src/rebar_config.erl#L240-L267)
and [the Erlang `file:consult/1` documentation](https://www.erlang.org/docs/25/man/file.html#consult-1)
are the authoritative references. The relevant dependency and profile shapes
are documented by [Rebar3 dependencies](https://rebar3.org/docs/configuration/dependencies/)
and [profiles](https://rebar3.org/docs/configuration/profiles/).

For DepLens, a small scanner/parser for the static dependency shapes is more
appropriate than either regex or a complete Erlang implementation. It can
recognize `deps`, `project_plugins`, and nested profile configuration while
returning an incomplete result for a dependency form it cannot interpret.

## Go package found

| Package | API and syntax | Maintenance and cost | Decision |
| --- | --- | --- | --- |
| [`github.com/scagogogo/erlang-rebar-config-parser`](https://github.com/scagogogo/erlang-rebar-config-parser) | [`Parse`, `ParseReader`, and `ParseFile`](https://github.com/scagogogo/erlang-rebar-config-parser/blob/main/pkg/parser/parser.go) produce an AST of [`Atom`, `String`, `Integer`, `Float`, `Tuple`, and `List`](https://github.com/scagogogo/erlang-rebar-config-parser/blob/main/pkg/parser/types.go). It handles whitespace, comments at top level, quoted atoms, nested lists/tuples, strings, and numbers. | MIT-licensed ([license](https://github.com/scagogogo/erlang-rebar-config-parser/blob/main/LICENSE)); its `go.mod` has no required modules ([source](https://github.com/scagogogo/erlang-rebar-config-parser/blob/main/go.mod)). It has no release tags, only 13 commits, and the latest main-branch commit was 2025-07-17 ([history](https://github.com/scagogogo/erlang-rebar-config-parser/commits/main/)). pkg.go.dev reports a pseudo-version rather than a tagged stable release ([package page](https://pkg.go.dev/github.com/scagogogo/erlang-rebar-config-parser/pkg/parser)). | **Do not adopt.** |

The package is a reasonable small project, but it does not provide a complete
Erlang term parser. Its source only dispatches tuples, lists, strings, quoted
atoms, ordinary atoms, and numbers. In particular, it does not parse maps,
binaries/bit syntax, character literals, variables, macros, records,
operators or expressions, and it does not support list tails. It is therefore
not a way to delegate general Erlang syntax maintenance. Replacing our local
parser would also drop support that matters to the current extraction scope,
such as comments within nested collections and list tails.

## Non-candidates

Packages described as Erlang *term format* implementations are usually for
the binary External Term Format used by Erlang distribution/RPC, not the
source text in `rebar.config`. Erlang documents that binary representation
separately in its [External Term Format specification](https://www.erlang.org/docs/23/apps/erts/erl_ext_dist.html).
For example, [`gitlab.com/andrenathan/go/ei`](https://pkg.go.dev/gitlab.com/andrenathan/go/ei)
is in that category and cannot parse a config file.

Invoking `erl` and `file:consult/1` would parse the full static term language,
but would add an Erlang runtime requirement to a standalone Go CLI and require
cross-process term serialization. It is not a Go-module solution and is not
justified for this analyzer.

## Recommended boundary

Keep the local term parser deliberately limited to syntax needed to locate and
interpret dependency declarations. It should:

* accept atoms (including quoted atoms), strings, tuples, lists, comments, and
  the dependency forms verified in fixtures;
* treat a supported dependency container with an unsupported dependency source
  as incomplete, rather than guessing;
* extend only when a real `rebar.config` fixture needs an additional static
  term form.

Do not try to evaluate `rebar.config.script`. Rebar3 documents that file as
[evaluated Erlang code](https://rebar3.org/docs/configuration/config_script/),
which is intentionally outside the safe, offline static analysis boundary of
DepLens.
