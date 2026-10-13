# E21 - MCP Agent Investigation Tools

Status: complete

## Summary

Standalone service track for exposing logs, traces and approved PostgreSQL data to an LLM agent
over the Model Context Protocol. The implementation retains stdio and adds an independently
managed, authenticated Streamable HTTP runtime and bounded PostgreSQL SELECT queries so an agent
can connect observed failures with approved application state.

Separate from E07 and E08 because the consumer is a model with a fixed context window. Projection,
grouping, discovery and explicit caps govern what reaches the agent; database access also requires
an enforceable authorization boundary before query results leave the service.

## Goal

A read-only MCP service that lets an agent identify a failing operation, follow its trace and
inspect approved application data through SELECT queries. It runs independently of Codex, can be
updated by restarting its own container, returns bounded results and states what was withheld.

## Scope

- `services/mcp-logs` as an independent service with its own transport, tool surface and lifecycle
- MCP tool design: naming, descriptions, input schemas, refusal messages, next-step hints
- the boundary between upstream documents and the model: projection, grouping, redaction, caps
- read paths to Elasticsearch (logs), Tempo (traces) and configured PostgreSQL databases
- Streamable HTTP alongside the existing stdio transport, with explicit local authorization
- service-owned Docker/config assets, Compose integration and Task lifecycle commands
- schema and approved-view discovery, parameterized SELECT execution and database privileges
- evolution of a small tool surface as services, databases and tables grow
- excludes the log shipping pipeline and field contract, owned by E07
- excludes dashboards and alert rules, owned by E08 and E20
- excludes write tools, arbitrary database connections and unrestricted SQL administration

## Key Decisions

- the context window is the scarce resource, not the cluster; caps exist to protect the model
- tool count stays small and fixed; discovery absorbs growth rather than generating tools per
  service, database or table
- withheld data is always reported, because an agent that believes a slice is the whole picture
  reasons confidently and wrongly
- logs and database values are untrusted input, including text that resembles agent instructions
- stdio remains available; HTTP accepts only operator-issued API keys and resolves
  each key to a stable client/agent identity in a server-owned registry
- local HTTP defaults to loopback publishing with API-key authentication, never anonymous access;
  shared remote deployment and per-user data policies remain outside this local-runtime scope
- HTTP handlers adapt transport only; services own validation, limits and read-only transaction
  boundaries; repositories own PostgreSQL queries and connection details
- optional or unavailable PostgreSQL must not prevent MCP startup or logs/trace investigation
- redaction reduces accidental exposure; approved relation/column grants define database access
- values that are few and change rarely belong in the schema as enums; anything that grows with the
  system is discovered through a call and validated on the server

## Implementation Tracks

### E21-003 - Streamable HTTP Runtime And Local Lifecycle

Scope:

- add a protocol-compatible Streamable HTTP endpoint while retaining stdio; select and document
  the supported MCP revision, initialization handshake and session behavior
- clients send an issued API key as `Authorization: Bearer <issued-key>`; no client-supplied name
  or identity substitutes for the key; header names and authentication scheme use shared constants
- authenticate before every `/mcp` request: initialization, tool discovery/calls, POST requests,
  GET/SSE and session DELETE; missing, empty, malformed, unknown or revoked keys return HTTP 401
  without executing tools or calling backends; retain applicable Origin checks
- resolve keys through a server-owned map from issued key to stable client/agent identity;
  for bootstrap, an explicit development map literal in auth middleware is allowed
- construct the map in the app/middleware constructor and inject it, with no package-level mutable
  runtime state; treat the map as immutable while serving; an empty registry fails HTTP startup
- operators issue keys manually; v1 updates/revokes keys by rebuilding/restarting MCP;
  issuance APIs, database-backed key management, OAuth and future config/secret-store loading
  are deferred; local development fixtures live in service-owned code, not documentation
- session IDs are not authentication; bind session ownership to the issued key's client identity
  and reject cross-client session use, including GET/SSE and DELETE
- authenticate all HTTP tools, including PostgreSQL, with existing read-only and approved-data
  constraints unchanged; log only safe client IDs for auditing, never raw keys or bearer headers
- define recovery after MCP restarts: clients reinitialize expired/lost sessions instead of
  continuing with stale IDs; preserve supported request cancellation and tool error semantics
- keep Dockerfile and runtime config under `services/mcp-logs`, orchestration under `deploy/`;
  Compose publishes on loopback by default; local auth fixtures use the explicit development
  registry initially, with no anonymous fallback or requirement for a configuration store
- add Task commands to start, rebuild/restart, inspect status, read logs and stop MCP;
  include the standalone runtime in the normal deploy workflow
- make startup/readiness independent of optional PostgreSQL connectivity; unavailable backends
  produce bounded per-tool errors while unrelated tools remain usable
- document Codex HTTP connection setup and MCP-only updates: rebuild/restart the MCP container
  without restarting all of Codex; tool catalog/schema changes may still require client refresh
- Codex supplies the issued key through `bearer_token_env_var = "MCP_LOGS_API_KEY"`;
  setup documentation/examples show the environment variable name rather than a raw secret

Acceptance criteria:

- a real MCP client initializes over HTTP, lists tools and completes a log/trace investigation;
  existing stdio usage remains functional
- authentication, Origin rejection, protocol-version handling, cancellation and session recovery
  are exercised by automated tests, including container restart/reinitialization
- valid issued keys for multiple clients succeed; missing, malformed, wrong and removed keys
  return 401 with zero tool/backend calls across POST, GET/SSE and DELETE, including discovery
- cross-client session access is rejected; revocation takes effect after MCP rebuild/restart;
  an empty registry fails startup and secrets are absent from logs, docs and examples
- Task lifecycle commands and deploy integration work with documented local configuration;
  a restart serves updated code and a missing/down PostgreSQL does not block MCP readiness
- documentation distinguishes runtime reconnection from a client tool-catalog refresh and does
  not promise that every connected client hot-reloads changed tool schemas

### E21-004 - Bounded PostgreSQL SELECT Investigation Tools

Scope:

- expose configured database aliases only; credentials and DSNs are server-owned configuration,
  never tool arguments; keep a fixed tool set for schema discovery and SELECT execution
- discover only approved schemas, relations, columns and views visible to the investigation role;
  support one validated SELECT statement with typed bind parameters, including useful joins and
  read-only CTEs within the supported grammar
- use a dedicated nonowner, nonsuperuser role without BYPASSRLS or inherited privileged roles;
  grant access only to approved relations/columns/views and retain applicable row-level security
- begin an explicit READ ONLY transaction before execution and always close it on completion,
  failure or cancellation; do not treat a SELECT string-prefix check as an authorization boundary
- parse and validate the supported grammar: reject multiple statements, data-modifying CTEs,
  SELECT INTO, COPY, CALL, DDL/DML, row locks and unsupported syntax before execution
- allow only audited functions needed for investigation; account for implicit/cast/operator and
  view function calls, PUBLIC execution privileges, SECURITY DEFINER behavior and temporary-object
  exceptions to read-only restrictions; restrict privileges and reject unsafe query forms
- define a deterministic search path and privilege policy; READ ONLY alone does not guarantee
  the absence of every function or external side effect
- deny `password_credentials`, `sessions` and `oauth_states` by default, including indirect view
  access; evaluate account PII and original URLs through explicitly approved views/columns
- enforce configurable server ceilings for execution time, cancellation, returned rows, total
  response bytes and individual fields; caller arguments can narrow but cannot widen ceilings
- fetch at most the row cap plus one to detect additional rows; report row/byte/field truncation
  explicitly, preserve UTF-8, and distinguish an exhausted result from an unknown total
- sanitize PostgreSQL errors into useful categories/SQLSTATE without leaking DSNs, credentials,
  raw sensitive SQL, parameter values or unapproved data; keep structured logs low-noise

Acceptance criteria:

- a parameterized SELECT over approved views returns useful bounded data; schema discovery works
  across configured aliases without introducing per-table tools or revealing denied objects
- authorization tests deny sensitive relations, unapproved columns and unknown aliases, including
  attempts to bypass restrictions through views, functions or role/search-path manipulation
- query-validation tests reject all prohibited statement forms and side-effect functions;
  integration tests verify the restricted role and READ ONLY transaction independently of parsing
- empty results, NULLs, supported types, exact-limit and over-limit rows, oversized fields and
  byte budgets produce accurate truncation metadata without inventing an exact total
- syntax errors, parameter/type failures, permission denial, timeout, cancellation and database
  outages return sanitized per-tool errors; log/trace tools remain usable during database failure
- tests verify cleanup and bounded responses on success and failure, and confirm parameters,
  secrets and raw sensitive SQL are absent from logs and error responses

## Task Links

- `E21-001-add-mcp-logs-investigation-server.md`
- `E21-002-scale-tool-surface-to-dozens-of-services.md`
- `E21-003-add-streamable-http-runtime-and-local-lifecycle.md`
- `E21-004-add-bounded-postgresql-select-investigation-tools.md`

E21-003 and E21-004 have concrete task cards recording implementation and validation evidence.

## Implemented Decisions

- Official Go MCP SDK v1.6.1, stateful HTTP with JSON responses and 15-minute idle sessions;
  client identity comes from verified bearer keys and SDK sessions enforce ownership.
- Alias `shortener` exposes `mcp_investigation.links` through a dedicated reader, excluding URLs,
  titles, creator identifiers and identity/authentication tables. Local bootstrap removes PUBLIC
  TEMP privileges; the app owner retains its privileges.
- PostgreSQL's libpg_query AST runs through WASM via pinned `wasilibs/go-pgquery`, preserving
  CGO-disabled builds. Supported SELECT grammar fails closed; canonical SQL is executed.
- Explicit columns, ordinary joins/subqueries/nonrecursive CTEs and audited pg_catalog
  functions/casts are supported. READ ONLY transactions and a deterministic search path provide
  independent database enforcement.
- Defaults: 30 rows, maximum 100; a 600-byte field ceiling; 32-KiB response budget;
  5-second statements and 500-ms locks. Actual total is omitted when exhaustion was not observed.

## Optional Follow-Up

- whether trace search deserves additional tools beyond the current single-trace lookup
- private key-registry loading and per-team data policies for a future shared deployment

These are future extensions, not gaps in the delivered local investigation scope.

## Completion Note

All four implementation tracks are delivered: bounded log/trace tools, service and field discovery,
authenticated independent HTTP lifecycle, and approved PostgreSQL SELECT investigation. Existing
stdio remains available. The completed HTTP/database task cards record runtime smoke, outage,
security and quality-gate evidence; E21-002 is complete following final review. Discovery reads field names/types from Elasticsearch mappings and exact-match statistics
from aggregations, resolving the earlier field-taxonomy question.

## Completion Evidence

- All four implementation tasks are in `ai/plan/4-complete`.
- Final tests, builds, lint and MCP race checks passed on 2026-10-13 with Go 1.25.
- Final review approved descriptor redaction, mapping-aware filters and the public
  educational-key explanation. No open blocking findings remain.
