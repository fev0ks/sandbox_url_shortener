# E21-002 - Scale Tool Surface To Dozens Of Services

Status: complete

Epic: E21 - MCP Agent Investigation Tools

## Summary

The implemented `mcp-logs` tool surface supports service growth through live discovery, domain
narrowing and progressive disclosure of service-specific fields. Log-tool count and schemas remain
independent of the number of observed services, replacing the bootstrap's inline service vocabulary.

## Scope

- replace the service enum with server-side discovery and validation
- two-step narrowing so the agent never faces a flat list of dozens of internal names
- split the field taxonomy into a required core and a per-service tail
- a `describe_service` tool for progressive disclosure of one service's fields
- caching, authorization and index-routing consequences of the above
- excludes generating tools per service, which is explicitly rejected below
- excludes changes to the E07 field contract itself

## Acceptance Criteria

- the tool count stays fixed as the number of services grows
- the tool schema size does not grow with the number of services
- an unknown service is answered with the closest matching names and a pointer to `list_services`,
  not a bare rejection
- `short_code` no longer appears in the surface shared by every service
- an agent can learn one service's field list, types and cardinality without carrying every
  service's schema
- discovery results are cached; validation for individual tools does not issue cross-service level
  aggregations. Explicit `list_services` calls fetch fresh counts for their selected scope

## Assumptions

- domains or teams are few and change rarely, so they can remain a closed vocabulary
- the E07-002 core field contract holds for every service that is registered
- several dozen services implies several teams, which changes the authorization model
- the P0 and P1 findings on E21-001 are resolved before this task starts, so this builds on a
  correct design rather than carrying its defects forward

## Implementation Notes

### Decisions Taken Before Implementation

- **Discovery is live, from Elasticsearch.** Service names come from a terms aggregation over
  `service.keyword`; field names and types come from the index mapping; example values and
  cardinality come from aggregations. `MCP_LOGS_SERVICES` survives only as an operator allowlist and
  as a fallback when discovery fails, and is empty by default.
- **Domains are configured, not discovered.** `MCP_LOGS_DOMAINS` maps a domain to its services. The
  log schema has no domain field, and adding one would change the E07-002 contract across every
  service, which is outside this epic. Domains stay a closed vocabulary in the schema because they
  are few and change rarely.
- **Operator scoping bounds discovery.** A server started with an allowlist cannot reach past it.
  E21-003 subsequently added HTTP authentication and session ownership by issued-key client ID.
  Per-team data policies remain a future shared-deployment concern, outside this local scope.

### Bootstrap Constraints Addressed

- The bootstrap embedded service names in the shared tool schema. Discovery and plain service
  arguments remove that recurring cost as the fleet grows.
- Flat lists of unfamiliar internal names are difficult to navigate. Configured domains and a
  bounded `list_services` result support two-step narrowing.
- Service-specific fields such as `short_code` previously lived in the shared surface. They now
  use discovered, validated tail filters and are projected only when requested.

### Anti-Pattern To Avoid

Do not generate tools per service. Forty services times five tools is two hundred declarations in
every request; that multiplies the schema problem rather than solving it. The tool count stays
fixed. What changes is where the values come from.

### Implemented Design

- Replace the service enum with discovery. `service` is a plain string; the server checks it
  against a live registry and, on a miss, answers with the closest matching names and a pointer to
  `list_services` rather than a bare rejection. The service layer owns this check.
- Narrow in two steps. `list_services` accepts a configured domain and caps results at twelve
  services. Domains remain a small, closed vocabulary.
- Split the field taxonomy into a core and a tail. The core is the E07-002 contract every service
  may emit and is projected whenever present; typed filters cover supported core arguments.
  Everything else moves to a generic `filters` object on `search_logs`
  (`additionalProperties: {type: string}`), with the field names validated server-side per service.
  This removes `short_code` from the shared surface.
- The sixth log tool, `describe_service`, returns the field list of one service with types, example
  values and cardinality. Cardinality is the point: without it the agent will try to group by
  `request_id`. This is progressive disclosure - the model asks about the one service it needs
  instead of carrying every schema at all times.
- Return the core fields always, plus the tail fields that were filtered on. Saving schema tokens
  and then returning the whole document undoes the saving on the response side.

### Operational Consequences

- Cache discovered service registries and field sets using `MCP_LOGS_DISCOVERY_TTL`.
  Live error/warning counts are fetched only for explicit `list_services` requests.
- Retain operator service scoping alongside the issued-key HTTP identity model. Shared production
  deployments would need explicit per-team data authorization beyond this local implementation.
- Account for per-service indices. If logs are sharded by service, a query without a service is no
  longer merely broad - it is expensive for the cluster, and index routing becomes part of the
  repository layer.

### Rule Of Thumb

A closed vocabulary in the schema is for things that are few and change rarely. Anything that grows
with the system is discovered through a call and validated on the server.

## Progress Checklist

- [x] confirm scope and assumptions
- [x] replace the service enum with a live registry and a near-match refusal path
- [x] add domain or team narrowing to `list_services`
- [x] split the field taxonomy into core and tail, moving `short_code` out of the shared surface
- [x] add the generic `filters` object to `search_logs` with per-service server-side validation
- [x] add the `describe_service` tool including field cardinality
- [x] return core fields plus the tail fields that were filtered on
- [x] cache discovery results with a short TTL
- [x] revisit authorization for the multi-team case - implemented as operator scoping; real
      multi-team authorization deferred, see Review Notes
- [x] account for per-service index routing in the repository layer
- [x] add or update tests
- [x] run validation
- [x] pass review or resolve review findings

## Validation

- Final pre-publication checks on 2026-10-13: `task test`, `task build` and `task lint`
  passed with `GOTOOLCHAIN=go1.25.0`; lint reported zero issues.
- `GOTOOLCHAIN=go1.25.0 go test -race -count=1 ./services/mcp-logs/...`: passed.
- Regression tests verify descriptor redaction/UTF-8 limits and preserved warnings;
  cached field discovery with fresh explicit level counts; exact-filter eligibility;
  actual keyword/numeric/boolean/date paths; unsupported mappings refused before search;
  scalar examples and selected fields with exact large-integer handling.
- The following original implementation evidence is retained for historical context:

Recorded implementation validation below predates this documentation closure; this edit does not
claim a new test run. Repository-wide completion gates are handled at final handoff.

- `go build ./services/mcp-logs/...` - pass
- `go vet ./services/mcp-logs/...` - pass
- `go test ./services/mcp-logs/...` - pass, six packages with tests (`config` gained coverage)
- `go test -race -count=1 ./services/mcp-logs/...` - pass; the registry cache is shared state
- end-to-end smoke against a stub Elasticsearch serving mapping, discovery, aggregation and search
  responses, driving the real binary over stdio. Verified in one run:
  - `tools/list` returns six tools and no service name appears anywhere in the schema
  - `list_services` discovers `shortener`, `analytics`, `identity` from the index; with
    `domain=core` it returns only `shortener` and `identity`
  - `describe_service` reports per-field types, example values and distinct-value counts
  - `search_errors` with `service=shortner` is refused with `Closest known: shortener`
  - `search_errors` reports `partial: true` with the sampling note
  - `search_logs` with `filters={"shortcode":...}` is refused with `Closest: short_code`
  - `search_logs` with the correct key returns `fields: {"short_code": "abc0"}` and a path
    redacted to `/r/abc0?token=[redacted]`
- unit test asserts the tool schema contains no service name, which is the property that keeps the
  per-turn cost flat as the fleet grows

## Review Notes

- Final changed-files review on 2026-10-13: APPROVED. Both pre-publication findings
  were fixed with regression tests: descriptor examples now use copied/redacted projections;
  generic filters resolve actual mappings and refuse fields without exact-match support.
- `exact_filterable` is explicit metadata; existing nonfilterable notes survive cardinality hints.

- Implemented as designed. The three decisions recorded above held up; nothing needed reopening.
- One correctness problem surfaced only in the end-to-end smoke, not in the unit tests:
  `describe_service` aggregated on `<field>.keyword` for every field it found in the mapping. That
  works under dynamic mapping, which gives every string a `keyword` subfield, and fails for a field
  explicitly mapped as analyzed `text` - and one such field fails the entire request, not one row of
  the answer. The repository now reads the subfield information out of the mapping and aggregates
  only where an exact-match form exists; a field without one is still described, and says it is not
  filterable by exact value.
- The `web` open question left over from E21-001 is answered structurally rather than by a decision:
  the registry is discovered from the index, so a service that ships no logs never appears. There is
  no longer a way to configure a service into existence and have it report zero errors forever.
- `MCP_LOGS_SERVICES` bounds what one process can reach, and discovery is intersected with it.
  HTTP authentication/session ownership is delivered by E21-003; per-team data policies are an
  optional future extension rather than unfinished acceptance work in E21-002.
- Behaviour changes worth knowing when reading the diff:
  - `MCP_LOGS_SERVICES` changed meaning: it was the vocabulary, it is now an allowlist, and its
    default is empty rather than a list of service names.
  - `short_code` is gone from `domain.LogEntry` and from the `search_logs` schema. It is reached
    through `filters` and comes back under `fields`.
  - `LogRepo` grew the discovery half (`DiscoverServices`, `DescribeFields`), and
    `NewInvestigation` takes an `Options` struct rather than a service list and a URL.

## Review Findings

### P0 Critical

- [x] no open P0 findings

### P1 High

- [x] descriptor examples are redacted and capped before model output; no open P1 findings

### P2 Medium

- [x] generic filters respect discovered mapping types and preserve warnings; no open P2 findings

### P3 Low

- [x] no open P3 findings
