# Model selector transport options

Date: 2026-08-11

## Decision scope

The fixture selector must reuse the installed Codex binary and its existing
ChatGPT/Codex OAuth login. A direct OpenAI API integration, API keys, and
separate API billing are out of scope.

Within that constraint, the selector needs to send about 50,000 tokens of
untrusted GitHub source from memory, receive only selected candidate IDs and
rationales, prevent model-invoked tools from reaching the network, GitHub
credentials, or the host filesystem, and avoid persisting the prompt as a Codex
session.

## Revised recommendation

Use a fresh `codex exec` process for each selection, with the packet on stdin,
`--ephemeral`, `--ignore-user-config`, a strict output schema, an empty private
working directory, and a custom deny-by-default permission profile. Disable
every relevant tool family exposed as a feature by the installed Codex version.

This is the strongest practical first-party Codex-OAuth option. It is not the
same guarantee as a Responses API request with `tools: []`:

- several tools can be **removed** from the model-visible surface, including the
  default shell tool, hosted web search, apps/plugins, subagents, browser and
  computer use, and image generation;
- any local tools that remain are made **ineffective outside an empty temporary
  root** by the filesystem and network sandbox;
- the public Codex interface does not provide one supported "no tools at all"
  switch, so Q30 must not claim that the model receives zero tools.

The installed `codex-cli 0.147.0-alpha.6.5` exposes `--ephemeral`,
`--ignore-user-config`, `--ignore-rules`, `--output-schema`, repeated
`--disable`, and stdin prompt input. The upstream source classifies
`shell_tool` as a stable feature whose purpose is to enable the default shell
tool ([feature registry](https://github.com/openai/codex/blob/main/codex-rs/features/src/lib.rs)).
Record the exact Codex version in the selector fingerprint and test the required
feature names at preflight. The implementation accepts any version number but
fails closed when the isolation contract is unavailable.

## Recommended invocation

Go should resolve the installed Codex executable, create two private temporary
directories, and invoke it directly without a shell:

- `work_dir`: empty and mode `0700`; this is the only caller-selected readable
  root and the Codex working directory;
- `control_dir`: mode `0700`; contains only the non-sensitive JSON output schema
  and is not granted to model tools.

The argument vector should be equivalent to the following. The Go
implementation must construct arguments directly and TOML-escape the absolute
temporary path; it must not interpolate this example through a shell.

```text
codex exec
  --ephemeral
  --ignore-user-config
  --ignore-rules
  --strict-config
  --skip-git-repo-check
  --cd <empty-work-dir>
  --output-schema <control-dir>/selection.schema.json
  --disable shell_tool
  --disable unified_exec
  --disable shell_snapshot
  --disable hooks
  --disable multi_agent
  --disable apps
  --disable plugins
  --disable remote_plugin
  --disable plugin_sharing
  --disable tool_suggest
  --disable skill_search
  --disable skill_mcp_dependency_install
  --disable in_app_browser
  --disable browser_use
  --disable browser_use_external
  --disable browser_use_full_cdp_access
  --disable computer_use
  --disable image_generation
  --disable goals
  --disable guardian_approval
  --disable workspace_dependencies
  --disable auth_elicitation
  --disable tool_call_mcp_elicitation
  --disable in_app_updates
  --config web_search="disabled"
  --config approval_policy="never"
  --config default_permissions="fixture-selector"
  --config permissions.fixture-selector.filesystem={"<empty-work-dir>"="read"}
  --config permissions.fixture-selector.network.enabled=false
  --config shell_environment_policy.inherit="none"
  --config allow_login_shell=false
  --config project_doc_max_bytes=0
  --config skills.include_instructions=false
  --config analytics.enabled=false
  -
```

The final `-` makes the prompt come from stdin. The CLI documentation says
stdin is accepted for the prompt, `--output-schema` supplies the final-response
JSON Schema, and `--ephemeral` avoids session rollout persistence
([developer command reference](https://learn.chatgpt.com/docs/developer-commands?surface=cli),
[Codex source README](https://github.com/openai/codex/blob/main/codex-rs/README.md)).
Do not use `--json` or `--output-last-message`: capture ordinary stdout in
memory, keep stderr bounded in memory for diagnostics, and never persist either
raw stream.

The disable list above is intentionally explicit for the currently installed
binary. At collector startup, run `codex features list` once, verify every
required key exists, and fail closed on a missing key. `--strict-config` catches
unknown configuration fields; an unknown `--disable` feature is also an error in
the installed binary. Do not silently drop an isolation setting after a Codex
upgrade.

## What the isolation does and does not guarantee

### Network

`permissions.fixture-selector.network.enabled=false` compiles to Codex's
restricted network policy. The permission compiler starts custom profiles from
a restricted filesystem and restricted network, and maps `enabled=false` to
`NetworkSandboxPolicy::Restricted`
([permission compiler](https://github.com/openai/codex/blob/main/codex-rs/core/src/config/permissions.rs#L327-L387)).
On Linux, the current sandbox uses a separate network namespace when direct
network is restricted; macOS uses Seatbelt policy enforcement
([Linux sandbox](https://github.com/openai/codex/blob/main/codex-rs/linux-sandbox/README.md),
[core sandbox support](https://github.com/openai/codex/blob/main/codex-rs/core/README.md)).

This policy applies to model-invoked local tool processes, not to the parent
Codex process. Codex can therefore contact OpenAI using the existing ChatGPT
login while a shell process, if one somehow remains available, cannot contact
GitHub or any other host. Hosted tools do not depend on the local process
network sandbox, which is why web search, apps, browser/computer use, plugins,
and image generation must also be disabled rather than relying on
`network.enabled=false`.

### Filesystem and credentials

The custom profile grants no `:root`, `:minimal`, `:tmpdir`, `:slash_tmp`, or
`:workspace_roots` entry. It grants read-only access to the fresh empty working
directory and no write root. Codex's profile compiler begins with
`FileSystemSandboxPolicy::restricted(Vec::new())` and adds only recognized
entries from the selected profile
([permission compiler](https://github.com/openai/codex/blob/main/codex-rs/core/src/config/permissions.rs#L327-L368)).
Use this named profile alone; do not also pass `--sandbox read-only`, because
that legacy preset means broad read-only filesystem access rather than the
split, allowlisted-read policy.

`--ignore-user-config` keeps ambient MCP, plugin, and app configuration out of
the run while still using `CODEX_HOME` for authentication. Running in a new
non-repository directory, setting `project_doc_max_bytes=0`, disabling skill
instructions, and using `--ignore-rules` prevent repository instructions,
skills, and exec-policy rules from being imported into the selector context.

The child process environment should be an explicit allowlist. Retain only what
Codex needs for OAuth and TLS, including the existing `CODEX_HOME`; do not pass
`GH_TOKEN`, `GITHUB_TOKEN`, Git credential-helper variables, cloud credentials,
or `SSH_AUTH_SOCK`. `shell_environment_policy.inherit="none"` independently
ensures that any shell-like tool process receives none of the parent's
environment. Authentication remains the normal Codex ChatGPT sign-in rather
than API-key auth ([Codex authentication](https://learn.chatgpt.com/docs/auth)).

There is one qualification: Codex source can add exact runtime helper paths,
such as the selected zsh executable or an exec wrapper, to the tool-readable
policy so its own runtime can function
([runtime-readable roots](https://github.com/openai/codex/blob/main/codex-rs/core/src/config/permissions.rs#L449-L476)).
Therefore, the enforceable statement is "no caller-selected filesystem access
outside the empty root, apart from Codex-added runtime helper paths," not
literally "the empty root is the only readable inode." Disabling `shell_tool`,
`unified_exec`, and `shell_snapshot` makes those helper paths non-useful to the
model. This residual must be re-audited when upgrading Codex.

At least one local built-in can remain model-visible in current Codex versions.
For example, `view_image` reads through the selected tool environment with the
turn's filesystem sandbox context, so it cannot bypass the allowlisted read
policy
([view-image handler](https://github.com/openai/codex/blob/main/codex-rs/core/src/tools/handlers/view_image.rs#L127-L164)).
That is a concrete example of a tool being present but ineffective against host
files, not a tool being removed.

### Approvals and fail-closed behavior

Set `approval_policy="never"`. A blocked tool call must return its sandbox
failure to the model and must not ask a human or reviewer to widen permissions.
The tool orchestrator's current source explicitly avoids unsandboxed retry under
the relevant non-escalating path and preserves the sandbox denial
([tool orchestrator](https://github.com/openai/codex/blob/main/codex-rs/core/src/tools/orchestrator.rs)).
The parent process should also enforce a wall-clock timeout and terminate the
child if it loops on denied tool calls.

### Prompt and session persistence

Write the candidate packet only to the child's stdin. Do not place it in argv,
an environment variable, a temporary file, an error, or a log. Capture stdout
and stderr only in bounded memory. The output schema file contains no candidate
content and can be deleted with both temporary directories after the process
exits.

`--ephemeral` is the documented control that prevents Codex session rollout
files. It does not promise that the Codex installation performs no local writes
at all: OAuth refresh, model caches, update state, or non-content operational
metadata may still touch `CODEX_HOME`. Disable analytics and update checks, do
not enable `RUST_LOG`, and treat the public guarantee narrowly as **no persisted
session rollout containing the prompt**. OS crash dumps and third-party process
instrumentation are outside the Codex contract and must be disabled separately
if the deployment threat model includes them.

Service-side retention remains governed by the ChatGPT/Codex account's data
controls; `--ephemeral` is a local session-storage setting, not a provider-side
zero-retention promise.

## Structured output and validation

The schema should permit only the selected list and the two requested fields:

```json
{
  "type": "object",
  "properties": {
    "selected": {
      "type": "array",
      "minItems": 0,
      "maxItems": 5,
      "items": {
        "type": "object",
        "properties": {
          "id": { "type": "string" },
          "rationale": { "type": "string" }
        },
        "required": ["id", "rationale"],
        "additionalProperties": false
      }
    }
  },
  "required": ["selected"],
  "additionalProperties": false
}
```

Go must still parse and validate the final stdout:

- every ID was present in the exact packet sent;
- IDs are unique and the count is either zero or within the remaining range
  needed to bring the detector to 3-5 accepted examples;
- rationales are non-empty, bounded, valid UTF-8, and contain no forbidden
  controls;
- no repository coordinate or file content supplied by the model is trusted;
- only validated stable IDs and rationales are persisted.

The schema constrains the final answer, but it does not stop the Codex agent
from attempting intermediate tool calls. The feature disables, sandbox, approval
policy, timeout, and post-validation are all independent controls.

## Approximately 50,000 tokens

Codex CLI, its SDK wrappers, and app-server do not expose the Responses
input-token-count endpoint under Codex OAuth. They report usage after a turn,
but there is no exact preflight count that includes Codex's hidden instructions
and tool schemas. Go should define the 50,000-token target as the **candidate
packet budget**, count or conservatively estimate that packet locally for the
configured model, and leave headroom for selector instructions and any residual
tool schemas. Record actual post-turn input usage as an operational metric and
tune the packet target from observed runs; do not describe the local estimate as
an exact full-request count.

## OAuth-compatible option comparison

| Option | OAuth reuse | Input and output | Isolation and persistence | Verdict |
|---|---|---|---|---|
| `codex exec` | Uses installed Codex ChatGPT login | Prompt on stdin; `--output-schema`; ordinary stdout | `--ignore-user-config`, explicit feature disables, custom permission profile, `--ephemeral` | **Recommended**; smallest and strongest auditable surface |
| Raw `codex app-server` | Uses the same Codex account | Text in JSON-RPC `turn/start`; per-turn `outputSchema` | Exact restricted-read `sandboxPolicy` and ephemeral thread are available, but there is no app-server `--ignore-user-config` flag; full event stream and lifecycle add risk and complexity | Viable only with a separately isolated `CODEX_HOME`; not recommended here |
| TypeScript Codex SDK | Reuses CLI auth when no API key is supplied | Wraps CLI, writes input to stdin, supports `outputSchema` | Public thread options omit ephemeral and ignore-user-config controls; adds Node and JSONL buffering | Reject |
| Python Codex SDK | Uses app-server and supports ChatGPT login | In-memory text input and `output_schema` | Supports ephemeral threads, but public sandbox API exposes broad presets rather than exact readable roots and does not solve ambient config isolation; adds Python | Reject |

### Raw app-server details

Start app-server on stdio with the same feature-disable and configuration
overrides used for CLI where its flags permit them. It reuses the OAuth account
from `CODEX_HOME`. Create the thread in memory:

```json
{
  "method": "thread/start",
  "id": 10,
  "params": {
    "model": "<configured-model>",
    "cwd": "<empty-work-dir>",
    "approvalPolicy": "never",
    "ephemeral": true
  }
}
```

App-server can then express the exact filesystem policy directly on
`turn/start`:

```json
{
  "method": "turn/start",
  "id": 30,
  "params": {
    "threadId": "thr_123",
    "input": [{ "type": "text", "text": "<selector packet>" }],
    "approvalPolicy": "never",
    "sandboxPolicy": {
      "type": "readOnly",
      "access": {
        "type": "restricted",
        "includePlatformDefaults": false,
        "readableRoots": ["<empty-work-dir>"]
      }
    },
    "outputSchema": { "type": "object" }
  }
}
```

The protocol documents restricted `readOnly.access`, readable roots, and
per-turn `outputSchema`
([app-server sandbox and turn protocol](https://learn.chatgpt.com/docs/app-server#turns)).
`thread/start` also accepts `ephemeral: true` for an in-memory thread in the
first-party protocol/source
([app-server README](https://github.com/openai/codex/blob/main/codex-rs/app-server/README.md),
[thread protocol type](https://github.com/openai/codex/blob/main/codex-rs/app-server-protocol/src/protocol/v2/thread.rs)).

However, `codex app-server` has no equivalent of `codex exec
--ignore-user-config`. Starting it against the normal OAuth `CODEX_HOME` can
load ambient MCP/plugin configuration before a thread starts. A clean temporary
`CODEX_HOME` would require securely bridging or copying the existing OAuth
credential and then handling refresh and cleanup correctly. That is a larger
credential-handling surface than the CLI solution, so app-server is not the
recommended first implementation.

### SDK details

The TypeScript SDK explicitly wraps `codex exec`, exchanges JSONL over
stdin/stdout, and supports per-turn schemas
([TypeScript SDK README](https://github.com/openai/codex/blob/main/sdk/typescript/README.md)).
Its public `ThreadOptions` expose sandbox, working directory, network, web
search, and approval controls, but not `ephemeral` or `ignoreUserConfig`
([TypeScript `ThreadOptions`](https://github.com/openai/codex/blob/main/sdk/typescript/src/threadOptions.ts)).
Its subprocess source also shows the prompt written to stdin and the inherited
or caller-supplied environment, but no corresponding ephemeral flag
([TypeScript exec wrapper](https://github.com/openai/codex/blob/main/sdk/typescript/src/exec.ts#L79-L181)).

The Python SDK uses app-server, can use ChatGPT login, accepts
`ephemeral=True`, and supports `output_schema`, but its documented sandbox
surface is `read_only`, `workspace_write`, or `full_access`; `read_only` is not
an exact-root policy
([Python SDK API reference](https://github.com/openai/codex/blob/main/sdk/python/docs/api-reference.md)).
Neither SDK improves the boundary enough to justify adding a second runtime to
the Go collector.

## Prior broader result

Without the OAuth-only constraint, a stateless Responses API request with an
empty tool list, `tool_choice: "none"`, strict Structured Outputs, and
`store: false` would provide a cleaner literal no-tool contract and exact input
token counting. It was the earlier recommendation. It is now explicitly
excluded because it requires API credentials and API billing rather than the
installed Codex OAuth entitlement. This context explains why the revised Codex
recommendation is phrased as **tools removed where supported and otherwise made
ineffective**, not as an equivalent no-tool guarantee.

## Updated Q30 decision

> **Q30 - Selector invocation and retention:** Go invokes the pinned installed
> `codex exec` binary using the existing Codex ChatGPT OAuth login. It sends the
> untrusted candidate packet only on stdin and captures the structured final
> answer only in bounded memory. The run uses `--ephemeral`,
> `--ignore-user-config`, `--ignore-rules`, a strict output schema, a fresh empty
> working directory, `approval_policy="never"`, a deny-by-default custom
> permission profile with network disabled and only that empty directory
> readable, an empty shell environment, and an explicit version-checked disable
> list for shell, web, MCP/app/plugin, subagent, browser/computer, image, hook,
> and related tool families. Go persists only validated stable IDs, rationales,
> and non-content operational metadata. This removes the high-risk tools that
> Codex exposes as configurable features and makes residual local tools
> ineffective outside the sandbox; it does not claim that the model receives an
> empty tool list. Go generates the schema's upper bound from the detector's
> remaining capacity and rejects a nonzero selection smaller than the number
> needed to reach three accepted examples. The 50,000-token target applies to
> the candidate packet and is
> estimated locally with headroom because Codex OAuth surfaces provide no exact
> preflight count for the complete hidden agent request.
