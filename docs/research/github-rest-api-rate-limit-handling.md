# GitHub REST API rate-limit handling for the fixture collector

Date: 2026-08-12

## Scope

The fixture collector discovers public dependency-source files through
`GET /search/code`, then qualifies them with repository, Git-ref, and contents
`GET` requests. This note covers the GitHub.com REST API limits that apply to
that workload and recommends a small retry, pacing, and diagnostic policy.

Only first-party GitHub documentation and the collector's source are used.

## Limits that matter

The collector authenticates with `GH_TOKEN`, `GITHUB_TOKEN`, or the token from
`gh auth token`. A personal access token and requests made on a user's behalf by
OAuth or GitHub Apps normally share that user's **5,000-request-per-hour**
primary limit. Some GitHub Enterprise Cloud app cases receive 15,000 per hour.
Unauthenticated public requests receive only 60 per hour
([GitHub: primary REST API limits](https://docs.github.com/en/rest/using-the-rest-api/rate-limits-for-the-rest-api#about-primary-rate-limits)).

Code search has a separate, much smaller bucket. It requires authentication and
allows **10 requests per minute**. Other authenticated search endpoints allow
30 per minute, but that does not increase the code-search allowance
([GitHub: search rate limit](https://docs.github.com/en/rest/search/search#rate-limit),
[GitHub: Search code](https://docs.github.com/en/rest/search/search#search-code)).
The `GET /rate_limit` response exposes code search as `code_search`, ordinary
REST requests as `core`, and other search endpoints as `search`
([GitHub: rate-limit resources](https://docs.github.com/en/rest/rate-limit/rate-limit#about-rate-limits)).

GitHub also applies secondary limits. Published ceilings include 100 concurrent
REST and GraphQL requests, 900 REST points per minute, and 90 seconds of API CPU
time per 60 seconds of wall time. Most REST `GET` requests cost one point, but
some endpoint costs and some secondary-limit triggers are not public, and the
limits can change without notice
([GitHub: secondary REST API limits](https://docs.github.com/en/rest/using-the-rest-api/rate-limits-for-the-rest-api#about-secondary-rate-limits)).
There is no endpoint that reports remaining secondary-limit capacity
([GitHub: checking rate-limit status](https://docs.github.com/en/rest/using-the-rest-api/rate-limits-for-the-rest-api#checking-the-status-of-your-rate-limit)).

The limits relevant to the current collector are therefore:

| Resource | Requests made by the collector | Published primary limit |
|---|---|---|
| `code_search` | `GET /search/code` pages | 10/minute, authenticated |
| `core` | repository, ref, source, and license requests | normally 5,000/hour for the authenticated user |
| Secondary | all requests, including the above | partly published, partly undisclosed |

The user-level `core` budget can also be consumed by other tools using the same
identity. A limiter inside this process cannot reserve capacity against another
collector, `gh`, or a separate application.

## What a `403` means

A `403` alone does **not** prove that a rate limit was reached. Both primary and
secondary rate limits can return `403` or `429`, but GitHub also uses `403` for
other problems, including an invalid `User-Agent`. The response headers and
bounded JSON error body are needed for classification
([GitHub: exceeding REST API limits](https://docs.github.com/en/rest/using-the-rest-api/rate-limits-for-the-rest-api#exceeding-the-rate-limit),
[GitHub: request headers](https://docs.github.com/en/rest/using-the-rest-api/getting-started-with-the-rest-api#headers)).

GitHub defines these response signals:

- `X-RateLimit-Resource`: the charged bucket, such as `core` or `code_search`;
- `X-RateLimit-Limit`, `X-RateLimit-Used`, and `X-RateLimit-Remaining`: the
  primary-window counters;
- `X-RateLimit-Reset`: the reset time as UTC epoch seconds;
- `Retry-After`: the minimum delay in seconds when GitHub supplies it;
- the JSON `message`: identifies a secondary-rate-limit response when the
  remaining primary count is not zero;
- `X-GitHub-Request-Id`: a useful support and correlation identifier.

The rate-limit header meanings are documented by GitHub, and the general REST
response documentation exposes `Retry-After` and `X-GitHub-Request-Id`
([GitHub: rate-limit response headers](https://docs.github.com/en/rest/using-the-rest-api/rate-limits-for-the-rest-api#checking-the-status-of-your-rate-limit),
[GitHub: REST response headers](https://docs.github.com/en/rest/using-the-rest-api/getting-started-with-the-rest-api#about-the-response-code-and-headers)).

This means the recent collector error, `GitHub HTTP status 403`, cannot be
diagnosed after the fact. The current HTTP adapter discards all error-response
headers and body fields and returns only the status code
([current request adapter](../../cmd/fixturecollectorloop/github_acquisition.go)).
It may have been a secondary limit, but that is only a hypothesis.

## GitHub's required waiting behavior

GitHub specifies the following order for handling a rate-limit response:

1. If `Retry-After` is present, wait at least that many seconds.
2. Otherwise, if `X-RateLimit-Remaining` is zero, wait until
   `X-RateLimit-Reset`.
3. Otherwise, wait at least one minute. If a secondary-limit response repeats,
   use exponentially increasing waits and stop after a bounded number of
   retries.

Continuing to send requests while limited can result in the integration being
banned
([GitHub: handling rate-limit errors](https://docs.github.com/en/rest/using-the-rest-api/best-practices-for-using-the-rest-api#handle-rate-limit-errors-appropriately)).

GitHub also recommends serial requests to reduce secondary-limit risk. The
collector is already serial; it should remain so. GitHub recommends a fixed
one-second pause specifically for large numbers of mutating requests, not for
read-only `GET` workloads
([GitHub: REST API best practices](https://docs.github.com/en/rest/using-the-rest-api/best-practices-for-using-the-rest-api)).

## Recommended collector policy

### 1. Preserve response metadata before deciding what to do

For every GitHub response, parse a small `rateLimitSnapshot` containing status,
resource, limit, used, remaining, reset time, `Retry-After`, request ID, and a
bounded GitHub JSON `message`. Never include the authorization header or token
in errors or logs. Limit the error body independently (for example, 16 KiB) and
only retain the parsed message and documentation URL.

All bytes read across failed attempts and retries should still count against
the collector's decoded-response byte budget. Retrying must not create a route
around an existing safety limit.

### 2. Pace the two primary resources separately

Use one process-wide, context-aware gate per `X-RateLimit-Resource`:

- For `code_search`, start requests no faster than one every six seconds. This
  directly respects the documented 10-per-minute limit and avoids consuming a
  ten-request burst at the start of every detector.
- For `core`, use the latest response headers. If a successful response leaves
  zero remaining, close the gate until the reset time before issuing the next
  core request. Optionally spread requests over the remaining window using
  `timeUntilReset / remaining`; this produces steadier throughput, but it is an
  optimization rather than a GitHub requirement.
- Keep requests serial. Do not add concurrency in an attempt to compensate for
  the waits.

Prefer response headers over repeatedly polling `GET /rate_limit`. GitHub says
that the endpoint does not consume the primary limit but can consume secondary
capacity, and recommends response headers when possible
([GitHub: checking rate-limit status](https://docs.github.com/en/rest/using-the-rest-api/rate-limits-for-the-rest-api#checking-the-status-of-your-rate-limit)).

The smallest first implementation should use the six-second code-search gate
and the zero-remaining core gate. Header-driven spreading can be added only if
real runs still hit limits; this keeps the first change simple.

### 3. Retry only errors that are expected to become successful

Use a bounded retry loop around the shared `GET` request method:

| Response | Action |
|---|---|
| `403`/`429` with `Retry-After` | Wait that duration, then retry. |
| `403`/`429` with remaining `0` | Wait until reset, plus a small clock-skew margin, then retry. |
| `403`/`429` whose message identifies a secondary limit | Wait 60 seconds, then exponential delays such as 120 and 240 seconds. |
| Other `403` | Fail immediately with diagnostics; it may be authorization, `User-Agent`, or another forbidden condition. |
| `404` | Keep the existing candidate-not-found behavior; do not retry. |
| `422` | Fail without a generic retry; code search uses this for validation errors or spam detection, which needs message-specific diagnosis. |
| `500`, `502`, `503`, `504`, or a temporary transport error | Retry briefly with bounded exponential backoff and jitter. |
| Other `4xx` | Fail immediately. |

A suitable default is four total attempts for rate-limit responses and three
total attempts for transient transport/server failures. Every wait must select
on `ctx.Done()` so `--duration`, Ctrl-C, and tests interrupt it immediately.
If the next permitted retry is beyond the run deadline, stop cleanly and report
the scheduled reset rather than sleeping past the requested duration.

Every collector request is a `GET`, so replaying it cannot mutate GitHub. That
makes bounded transport retries safe for this adapter. This is a property of
the current collector interface, not permission to apply the same retry policy
to future mutating requests. GitHub's timeout guidance says to try a request
later or simplify it when the API terminates a long request
([GitHub: REST API timeouts](https://docs.github.com/en/rest/using-the-rest-api/troubleshooting-the-rest-api#timeouts)).

### 4. Add diagnosis and wait logs

Write a nested progress line whenever the collector must wait or retry, and a
complete bounded diagnosis if it finally fails. Examples:

```text
│    GitHub wait: resource=code_search reason=primary-rate-limit retry=1/4 delay=42s reset=2026-08-12T15:02:00Z remaining=0/10 request-id=ABCD:1234
│    GitHub retry: resource=core reason=secondary-rate-limit retry=2/4 delay=2m0s remaining=4812/5000 request-id=EFGH:5678
│    GitHub failure: status=403 class=forbidden resource=core remaining=4811/5000 request-id=IJKL:9012 message="Resource not accessible by personal access token"
```

For successful requests, avoid one log line per API call. Report rate status
only at meaningful thresholds, such as the first observation of each resource,
10% remaining, zero remaining, and after a reset. This keeps the existing
detector hierarchy readable.

### 5. Make requests identifiable and versioned

Add these headers to every GitHub request:

```text
Accept: application/vnd.github+json
User-Agent: deplens-fixture-collector
X-GitHub-Api-Version: 2026-03-10
```

GitHub requires a valid `User-Agent` and recommends the application name. It
also recommends specifying an API version
([GitHub: REST request headers](https://docs.github.com/en/rest/using-the-rest-api/getting-started-with-the-rest-api#headers)).
The `2026-03-10` version is currently supported; explicitly pinning it avoids
unannounced behavior changes from the default version
([GitHub: API versions](https://docs.github.com/en/rest/about-the-rest-api/api-versions)).

## What not to implement yet

- Do not switch authentication type solely for this failure. The collector is
  already authenticated, and 5,000 core requests per hour is normally adequate
  when handled correctly.
- Do not add parallel candidate qualification. GitHub explicitly recommends
  serial API requests to reduce secondary-limit risk.
- Do not retry every `403`. A non-rate-limit permission or request problem will
  not improve with waiting.
- Do not call `GET /rate_limit` before every request. Response headers provide
  the same primary-limit control signal without extra secondary-limit traffic.
- Do not add persistent HTTP caching yet. Candidate evidence is pinned by
  commit, but an in-memory acquisition cache already avoids some repeat
  repository and ref calls; retry and diagnostics solve the observed failure
  more directly.

## Recommended implementation order

1. Capture and classify the bounded GitHub error response and rate headers.
2. Add context-aware waits and bounded retries to the shared request method.
3. Add the six-second `code_search` gate and header-driven `core` reset gate.
4. Expose wait, retry, and terminal diagnostics through the existing nested
   progress printer.
5. Add `User-Agent` and `X-GitHub-Api-Version` headers.
6. Test `Retry-After`, primary reset, secondary backoff, context cancellation,
   terminal non-rate-limit `403`, transient `5xx`, byte accounting, and separate
   `core`/`code_search` gates with a fake clock and fake sleeper.

This policy lets a long collection run pause and recover when GitHub explicitly
asks it to slow down, while still failing promptly on real configuration or
authorization errors.
