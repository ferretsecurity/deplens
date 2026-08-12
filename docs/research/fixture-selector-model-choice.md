# Fixture selector model choice

Date: 2026-08-12

## Scope

The fixture collector sends an isolated `codex exec` invocation up to about
50,000 tokens of candidates that Go has already qualified. The model cannot do
useful tool calls. It must return exactly three stable candidate IDs and short
rationales. The set should include common usage and add useful structural or
edge-case variety. These choices form a long-lived implementation corpus, so a
wrong but valid-looking choice matters more than a modest latency increase.

Only models available through Codex CLI with the existing ChatGPT sign-in are
in scope. OpenAI documents ChatGPT sign-in as a supported Codex CLI
authentication method, and `codex exec` reuses the saved CLI authentication
([Codex authentication](https://learn.chatgpt.com/docs/auth),
[non-interactive mode](https://learn.chatgpt.com/docs/non-interactive-mode)).

## Verified model names

The current names are:

- `gpt-5.6-sol`
- `gpt-5.6-terra`
- `gpt-5.6-luna`

There is no model named “Solter.” This was likely a reference to **Sol, Terra,
or Luna**. All three are listed as available in Codex CLI. The unsuffixed
`gpt-5.6` alias selects Sol, but the collector should use the explicit slug so
its decision fingerprint is unambiguous
([Codex models](https://learn.chatgpt.com/docs/models),
[GPT-5.6 guide](https://developers.openai.com/api/docs/guides/latest-model)).

The installed environment was also checked locally on 2026-08-12: it has
`codex-cli 0.147.0-alpha.6.6` and is logged in using ChatGPT. This is an
environment observation, not a claim that every account has the same model
entitlements.

## Fit for this selector

| Model | Official positioning | Fit here |
|---|---|---|
| `gpt-5.6-sol` | Flagship for ambiguous, difficult, or high-value work needing judgment and polish | Best evidence-backed default. Selecting one complementary set is a semantic judgment, and the corpus has long-lived value. |
| `gpt-5.6-terra` | Everyday workhorse balancing intelligence, speed, and cost | Best efficiency challenger. It may be sufficient because Go has already filtered and validated the candidates. |
| `gpt-5.6-luna` | Fast, high-volume model for clear, repeatable extraction, classification, transformation, and structured summaries | Attractive for throughput, but this task is not only extraction. It must infer “common” and optimize diversity across a set. Do not make it the default without an eval. |

These descriptions and the relative capability/speed positioning come from
OpenAI's current [Codex model guidance](https://learn.chatgpt.com/docs/models).
The API model pages also confirm that Sol, Terra, and Luna all support
Structured Outputs and reasoning effort, so the strict three-item response
schema does not distinguish them
([Sol](https://developers.openai.com/api/docs/models/gpt-5.6-sol),
[Terra](https://developers.openai.com/api/docs/models/gpt-5.6-terra),
[Luna](https://developers.openai.com/api/docs/models/gpt-5.6-luna)).

## Reasoning effort

Use **medium** initially. OpenAI describes medium as the balanced starting
point and recommends using the lowest effort that produces the required result.
Higher effort takes longer and uses more tokens; high or extra-high should be
used when measured quality improves on a difficult workload
([Codex models](https://learn.chatgpt.com/docs/models),
[GPT-5.6 guide](https://developers.openai.com/api/docs/guides/latest-model)).

This selector has one bounded comparison, no planning or tools, and a tiny
structured output. That argues against starting at high, extra-high, or max.
Its semantic tradeoff—common usage versus complementary edge coverage—still
argues against low or no reasoning until those settings have passed an eval.

## Recommendation

Use **`gpt-5.6-sol` with medium reasoning** as the production default.

Use **`gpt-5.6-sol` with high reasoning** only as an optional
higher-assurance mode for unusually ambiguous packets, or if an evaluation
shows a meaningful selection-quality gain. Do not use max or multi-agent/Ultra:
OpenAI reserves max for the hardest single tasks and Ultra for work that can be
split among subagents; this selector is one small, indivisible decision
([Codex models](https://learn.chatgpt.com/docs/models)).

`gpt-5.6-terra` with medium reasoning is the first alternative to test if Sol's
latency or Codex-credit consumption limits collection throughput. Luna with
medium reasoning is the second, throughput-first alternative.

## What the documentation cannot establish

OpenAI does not publish accuracy, consistency, latency, or Codex-credit results
for this exact task. The model descriptions support the default above, but they
cannot prove that Sol selects better fixture sets than Terra or Luna, nor that
high reasoning improves over medium. Those claims require a workload-specific
evaluation. OpenAI's own guidance says to compare configurations on
representative tasks rather than assume that more reasoning is always better
([GPT-5.6 guide](https://developers.openai.com/api/docs/guides/latest-model)).

## Smallest useful evaluation

Before collecting many more detectors, retain 12 representative selector
packets: four straightforward/common, four with meaningful structural
variation, and four ambiguous or edge-heavy. Keep candidate order and prompt
identical.

1. Run `sol/medium`, `terra/medium`, and `luna/medium` once on all 12 packets.
2. Blind the model name and have one reviewer score each result on three binary
   requirements: credible common case, complementary coverage, and rationales
   supported by packet content. Schema/ID validity remains a Go-owned gate.
3. Repeat only packets where configurations disagree or a result fails. This
   tests consistency without paying for duplicate runs everywhere.
4. Record wall time and available Codex usage data. Reject a cheaper/faster
   configuration if it has more substantive selection failures.
5. Compare `sol/high` with `sol/medium` only on the ambiguous subset. Keep high
   only if it fixes failures or materially improves reviewer agreement.

Adopt Terra or Luna only if it matches Sol's reviewed success and consistency.
This eval is intentionally small: it tests the actual decision boundary without
building a general model-benchmark framework.
