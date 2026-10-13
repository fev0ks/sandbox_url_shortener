# E21-001 - Add MCP Logs Investigation Server

Status: complete

Epic: E21 - MCP Agent Investigation Tools

## Summary

Expose incident-investigation tools over the Model Context Protocol so an agent can find a failing
service, group its errors, follow one request chain and inspect the matching trace, without a human
writing Elasticsearch queries and without the raw index landing in the model's context window.

## Scope

- MCP server over stdio (newline-delimited JSON-RPC 2.0) in `services/mcp-logs`
- five tools: `list_services`, `search_errors`, `get_request_logs`, `get_trace`, `search_logs`
- projection, grouping, redaction and result caps between the upstream index and the model
- read paths to Elasticsearch (logs) and Tempo (traces)
- excludes any write or mutating tool
- excludes a network transport and its authorization layer

## Acceptance Criteria

- an agent can go service -> error group -> request chain -> trace using only tool results
- no tool can be called in a way that scans the index without a service and a time bound
- every capped result reports what was withheld
- credential-shaped values never leave the service
- `stdout` carries only protocol frames; logs go to `stderr`

## Assumptions

- the log index follows the E07-002 field contract (`service`, `level`, `msg`, `operation`,
  `error_kind`, `request_id`, `trace_id`, `short_code`, `status_code`, `method`, `path`)
- `@timestamp` is added by the Fluent Bit ES output (`Logstash_Format On`)
- stdio deployment only: the server runs with the permissions of whoever started it
- upstream endpoints are unauthenticated on the local stack

## Implementation Notes

- layering follows the repo convention: `cmd` -> `app` -> `transport/mcp` -> `service` -> `repository`
- `internal/domain` holds narrow projections of much wider upstream documents; this is where the
  context budget is defended
- `service` and `window` are JSON Schema enums so the model cannot invent a service name or an
  unbounded range; `search_logs` additionally refuses a request with no narrowing filter
- `service/redact.go` splits two concerns: `redact` removes credential-shaped substrings before data
  leaves the process, `normalizeMessage` collapses ids, hex, durations and numbers so repeated
  failures group together
- a failed tool call is returned as `toolCallResult{IsError: true}`, not as a JSON-RPC error, so the
  model can read the failure text and correct its arguments
- requests are handled sequentially, which keeps stdout writes ordered without a mutex

## Progress Checklist

- [x] confirm scope and assumptions
- [x] implement main code path
- [x] add or update tests
- [x] run validation
- [x] pass review or resolve review findings

## Validation

- `go build ./services/mcp-logs/...` - pass
- `go vet ./services/mcp-logs/...` - pass
- `go test ./services/mcp-logs/...` - pass, all five packages with test files
- `go test -race -count=1 ./services/mcp-logs/...` - pass, added after the read loop moved to its
  own goroutine
- static review of every file in `services/mcp-logs` against the guarantees stated in its `README.md`
- log pipeline cross-check against `deploy/docker-compose/docker-compose.yml` and
  `deploy/docker-compose/fluent-bit/fluent-bit.conf`
- protocol smoke test over stdio: `initialize` (client handshake logged),
  `notifications/initialized` (no response), `tools/list` (five tools, all `readOnlyHint`),
  `tools/call get_trace` with `trace_id=../../api/search?q=x` - refused as malformed
- shutdown smoke test: SIGTERM with an idle open stdin - process exits in ~200ms, where it
  previously needed SIGKILL
- not run: end-to-end query against a live Elasticsearch and Tempo

## Review Notes

- Structure is sound: layers are separated honestly, domain types are real projections rather than
  pass-through documents, and comments explain why a decision was made rather than restating the code.
- The blocking problems were not architectural. They were places where a guarantee stated in
  `README.md` did not hold in the code, which is worse than not stating it: the agent was told the
  result was complete when it was a slice.
- Approved. Every P0, P1 and P2 finding is resolved and verified; the one remaining P3 was closed as
  a deliberate deferral rather than a fix, with the reason recorded on the item and the work carried
  into `E21-002-scale-tool-surface-to-dozens-of-services.md`.
- Two findings turned out to be worse than first written, and the fixes reflect that:
  - `request_id` is not merely unvalidated, it is caller-supplied. It arrives from the inbound
    `X-Request-ID` header (`internal/platform/middleware/request_id.go`), is echoed into the result
    and into a next-step instruction, and was neither shape-checked nor length-capped. Both
    identifiers now have to match an expected shape, and a group whose sample identifier does not
    match offers no sample rather than a value the next call would reject.
  - The `search_errors` count problem could not be fixed by moving the grouping into an
    Elasticsearch aggregation, because the group key includes the normalized message pattern, which
    is computed in this process. The counts are still over a sample; the fix is that the result now
    says so, in a `coverage` block with prose rather than only a flag.
- Behaviour changes worth knowing when reading the diff:
  - `LogRepo.Search` returns an `elasticsearch.Page` instead of `(entries, total, error)`, so a
    lower-bound total can travel with the page.
  - `track_total_hits` is bounded at 10000; a total that hits the bound is reported as
    `total_is_lower_bound`.
  - `web` was removed from the default service vocabulary rather than given a log shipper. Shipping
    Next.js logs means a new Fluent Bit parser and a `logging: driver: fluentd` block in compose,
    which is deploy work outside this task. Recorded as an open question below.
- The review itself was read-only; the fixes were applied afterwards in the same session.

### Open Question

- `web` is currently unobservable through this server. Either ship its logs and put it back in
  `MCP_LOGS_SERVICES`, or accept that the frontend is investigated elsewhere. Worth a small E07
  follow-up task if the former.

## Review Findings

### P0 Critical

- [x] `search_errors` groups a 500-document slice and reports it as complete.
  `internal/service/investigation.go:133` requests `elasticsearch.MaxScanHint` (500,
  `internal/repository/elasticsearch/logs_repo.go:37`) sorted `@timestamp desc`, then groups those
  documents. `capped` is set only when `len(groups) > limit`
  (`internal/service/investigation.go:143`), so with 9000 errors in the window the model receives
  `capped: false` and group counts summing to 500. This is exactly the failure mode `README.md`
  names as a correctness bug. `FirstSeen` is affected too: it is the oldest of the newest 500
  records, not the start of the incident, and the model will report it as such.
- [x] `get_request_logs` returns the tail of a chain labelled oldest-first.
  `internal/service/investigation.go:184` issues the query with `size = limit` against a
  `@timestamp desc` sort, then reverses the returned slice locally
  (`internal/service/investigation.go:192`). For a 200-record chain the model gets records 151-200
  ordered ascending and presented as the beginning of the request. For investigation this is the
  worst possible slice, because the cause usually sits at the start of the chain. Sort ascending in
  Elasticsearch instead of reversing a descending page.

### P1 High

- [x] `Truncation` mixes units in `search_errors`. `internal/service/investigation.go:160` sets
  `Returned` to a group count and `Total` to a document count, so `returned: 20, total: 900` reads
  as "20 of 900 groups". The same field names mean records in `search_logs` and `get_request_logs`.
- [x] `trace_id` is interpolated into the Tempo URL without validation.
  `internal/repository/tempo/trace_repo.go:39` builds `baseURL + "/api/traces/" + traceID`. Verified
  behaviour: `"../../api/search?q=x"` yields `Path="/api/traces/../../api/search"` with
  `RawQuery="q=x"`, and `"abc?limit=1"` attaches an attacker-chosen query string. `trace_id` comes
  from log content, which `README.md` itself declares untrusted input. Validate as a fixed-length
  hex string and use `url.PathEscape`.
- [x] Redaction and the length cap apply only to `Message`.
  `internal/service/investigation.go:198` and `internal/service/investigation.go:321` call `redact`
  on the message alone. `Path`, `Operation`, `ShortCode`, `RequestID` and `TraceID` are returned raw
  and unbounded. `path` is the most common place a credential reaches a log
  (`/reset?token=...`), and the existing `secretAssignment` pattern already handles that shape - it
  is simply not called for the field. An oversized `path` also bypasses `maxMessageLen` entirely,
  which defeats the purpose of the service.
- [x] Graceful shutdown does not work. `cmd/mcp-logs/main.go:15` installs `signal.NotifyContext`,
  but `internal/transport/mcp/server.go:47` checks `ctx.Err()` only at the top of the loop while
  `reader.ReadBytes` blocks on stdin indefinitely. SIGTERM does not end the process until the client
  sends a line or closes stdin. Because `stop()` is deferred until `main` returns, the handler stays
  registered and a second interrupt does not help either, leaving SIGKILL as the only exit.
- [x] `get_trace` truncates before computing its aggregates.
  `internal/service/investigation.go:244` slices to `MaxTraceSpans` before the loop that finds the
  slowest span and the root, so the span the tool promises to highlight can be the one dropped. The
  root assignment at `internal/service/investigation.go:257` is unconditional, so with several root
  spans - normal in Tempo when spans are orphaned - the last one wins rather than the longest, and
  `TotalMS` is `0` when no root survives the cut.
- [x] `web` is in the closed vocabulary but its logs are never shipped.
  `internal/config/config.go:27` lists `web` by default, yet the `web` service in
  `deploy/docker-compose/docker-compose.yml` has no `logging: driver: fluentd` (only `shortener`,
  `identity` and `analytics` do), and `fluent-bit.conf` matches only those three tags.
  `list_services` will therefore always report `web: 0 errors`, and the model will conclude the
  service is healthy when it is merely unobserved. Either drop it from the default vocabulary or
  ship its logs.

### P2 Medium

- [x] Kibana links are built with `url.QueryEscape` inside a URL fragment
  (`internal/service/investigation.go:367`), so a space becomes a literal `+` rather than a space.
  Use `url.PathEscape` or explicit `%20`.
- [x] The Kibana link for `search_errors` filters `level:"ERROR"` only
  (`internal/service/investigation.go:162`) while the tool itself queries `ERROR` and `WARN`
  (`internal/service/investigation.go:30`). A human following the link to verify the agent's claim
  sees a different result set, which is the one thing the link exists to prevent.
- [x] `get_request_logs` hardcodes a 24-hour lookback (`internal/service/investigation.go:187`) that
  appears in neither the input schema nor the tool description. An older request returns no entries
  together with `next_step: "Chain complete. ..."`, so the model reports a complete chain it never
  saw. Surface the bound in the description and emit a distinct message on an empty result.
- [x] Schema constraints are not enforced server-side. `decodeArgs`
  (`internal/transport/mcp/tools.go:157`) is a plain `Unmarshal`, so `additionalProperties: false`,
  `minimum` and `maximum` hold only if the client validates. The window enum is genuinely checked
  through `Window.Duration`; the numeric bounds are not.
- [x] No upstream authentication in `Config`. There is no basic auth, API key or header support,
  so the `README.md` advice to give the upstream credentials read-only access cannot be followed.
- [x] Test doubles cannot catch the two P0 findings. The fake repository ignores its `limit`
  argument and returns a fixed slice (`internal/service/investigation_test.go:21`), so a test cannot
  tell "the repository returned everything" from "the repository returned the newest N".
- [x] Repository layer has no tests: `buildFilters`, `projectEntry`, `pickTime`, `projectSpans` and
  `unixNano` are all uncovered. `unixNano` silently returns the zero time on a parse failure, which
  then flows into span durations.

### P3 Low

- [x] `ServicesOverview.KibanaURL` is declared but never populated
  (`internal/service/investigation.go:102`), while `README.md` promises a Kibana link on every
  result. `domain.Span.Duration` is populated and read nowhere.
- [x] `track_total_hits: true` (`internal/repository/elasticsearch/logs_repo.go:77`) asks for an
  exact unbounded count. A numeric bound such as `10000` with a "10000+" presentation is the usual
  choice.
- [x] Level casing is asymmetric: the filter requires exact `"ERROR"`/`"WARN"`
  (`internal/repository/elasticsearch/logs_repo.go:108`) while the response parser defends itself
  with `strings.ToUpper` (`internal/repository/elasticsearch/logs_repo.go:136`).
- [x] Closed as deferred, not fixed: `search_errors` still pulls 500 full documents to group them in
  process. An Elasticsearch terms aggregation cannot group on the normalized message pattern,
  because that pattern is computed in this process, so replacing the in-process grouping is a design
  change rather than a fix. The P0 count problem it would have solved is instead solved by reporting
  coverage. Carried into `E21-002-scale-tool-surface-to-dozens-of-services.md`.
- [x] `redact` truncates by bytes (`internal/service/redact.go:47`), which splits a multi-byte rune;
  the JSON encoder substitutes U+FFFD rather than failing, so the effect is corrupted text.
- [x] Queries always sort and range on `@timestamp`
  (`internal/repository/elasticsearch/logs_repo.go:78`, `:182`) while `pickTime` provides a `time`
  fallback (`internal/repository/elasticsearch/logs_repo.go:276`). The projection tolerates a
  missing `@timestamp`; the query does not. It works today only because Fluent Bit adds the field.
- [x] Tool declarations carry no `annotations: {readOnlyHint: true}` even though read-only is a
  headline property of the service.
- [x] `initialize` ignores the client's params and always answers `2024-11-05`
  (`internal/transport/mcp/server.go:82`). Permitted by the spec, but the client's requested version
  is worth logging.
- [x] `isNotification()` is consulted only in the `default` branch
  (`internal/transport/mcp/server.go:101`), so a known method sent without an `id` - for example
  `tools/list` - is answered with `"id": null` instead of silence.
- [x] `string(rune('a'+i))` in `TestSearchErrorsReportsTruncation`
  (`internal/service/investigation_test.go:103`) runs past `'z'` into `'{'` at 25 iterations.
  Harmless, but unintentional.

## Follow-Up Scope

Not a review finding and not a blocker for this task. The current design is correct for the four
services of this repository; it stops working somewhere above ten to fifteen. The P0 and P1 findings
above are defects in a shipped design; scaling the tool surface is a different design, so it is
tracked separately and the boundary of E21-001 stays explicit.

See `E21-002-scale-tool-surface-to-dozens-of-services.md`.
