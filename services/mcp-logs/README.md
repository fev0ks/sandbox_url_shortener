# mcp-logs

Incident-investigation tools exposed over stdio or authenticated Streamable HTTP using the Model Context Protocol (MCP).

An agent connected to this server can find failing services, group their errors,
follow one request chain end to end and inspect the matching trace — without a
human writing Elasticsearch queries, and without the raw index landing in the
model's context window.

## Why it exists

The naive integration is one tool that forwards a query to Elasticsearch and
returns the response verbatim. It works in a demo and collapses on the first
real incident: the model asks for the whole index, gets thousands of documents
with dozens of fields each, and fills its context before the second call.

This server is designed around the opposite question — what should reach the
model — and everything below follows from it.

## Tools

Intended order of use. Each tool returns just enough to pick the next one.

| Tool | Purpose | Cap |
| --- | --- | --- |
| `list_services` | Error and warning counts per service for a window, optionally narrowed to one domain. Answers "where do I look" with an aggregation, not documents. | 12 services |
| `describe_service` | Fields one service can be filtered by, with types, example values, exact-filter eligibility and distinct-value counts. | 40 fields |
| `search_errors` | Failures of ONE service, grouped by operation, error kind and message pattern, with counts and a sample `request_id`. | 20 groups, counted over 500 records |
| `get_request_logs` | One request chain from its start, oldest first, plus the `trace_id` if present. Last 24h. | 50 records |
| `get_trace` | Spans of one trace with durations and the slowest span highlighted. | 100 spans |
| `search_logs` | Filtered search, for cases the other tools did not cover. Requires a narrowing filter. | 30 records |
| `list_databases` | Configured database aliases without credentials or a connectivity check. | Operator registry |
| `describe_database` | Approved diagnostic views and their columns. | Approved view only |
| `query_database` | One parameterized SELECT over approved data. | 30 rows by default, maximum 100; 64 columns; 32 KiB response |

## How the context is protected

1. **Field projection.** Upstream documents carry dozens of fields; only the
   ones an investigator reads cross this boundary.
2. **Grouping with normalization.** Identifiers, hex strings, durations and
   numbers are replaced with placeholders before grouping. Grouping on the raw
   text does nothing, because real messages embed a unique id in every line.
3. **Visible truncation.** Every capped result reports `returned`, `total` and
   `capped`, counting the same kind of thing the result holds — records for a
   record list, groups for a group list. Trimming silently would make the agent
   treat a slice as the whole picture.
4. **Visible sampling.** Grouping happens here, not in the index, so it only
   sees the records that were fetched. When the window holds more matches than
   the scan limit, `search_errors` returns a `coverage` block saying how many
   records the counts are actually over. A count over a sample presented as a
   total is a number the agent repeats as fact.
5. **Closed vocabularies where they stay closed.** `window` and `domain` are
   JSON Schema enums: both are few and change rarely. `service` is not — a list
   inlined in the schema is re-sent on every turn and grows with the fleet — so
   it is validated on the server instead, against a registry discovered from the
   index. Bounds are enforced server-side either way: a schema is a statement to
   the client, not a check, and undeclared arguments are refused rather than
   silently dropped.
6. **Progressive disclosure.** Only the core field contract is typed and always
   returned. Anything specific to one service is discovered through
   `describe_service` and filtered through the open `filters` map, and comes
   back in the result only when it was filtered on. Otherwise the schema grows
   with every service and every result pays for fields nobody asked about.
7. **Refusal on broad requests.** A search without a service, without a window,
   or without any narrowing filter is rejected with a message naming the
   allowed values and the tool to use instead.
8. **Next-step hints.** Each result states what to call next and with which
   value, and carries a Kibana link — over the same levels the tool queried, so
   a human checking the claim sees the same records.

## Security notes

- **Log content is untrusted input.** Anyone able to write text into the logs can
  place instructions there. Do not pair this server with tools that have outgoing
  side effects in the same session.
- **Credentials are redacted before the data leaves the service.** Everything
  returned here is forwarded to a model provider. Redaction covers `msg`, `error`,
  `path`, descriptor examples/notes, and requested service-specific fields, including quoted credential
  assignments and URL userinfo. Free-text values are capped at 600 bytes without
  splitting UTF-8 characters, followed by `...(truncated)` when capped; other
  projected fields are length-capped. See
  `internal/service/redact.go`.

- **Identifiers are validated, not trusted.** `request_id` comes from the
  inbound `X-Request-ID` header, so it is whatever a caller sent, and `trace_id`
  is read back out of log content. Both are checked against an expected shape
  before they are echoed into a result or an outbound URL.
- **Read-only by design.** No tool mutates anything, and every tool says so in
  its `readOnlyHint` annotation. Give the upstream credentials read access only.
- **Issued HTTP keys.** Every `/mcp` method checks `Authorization: Bearer <issued-key>`
  before tool execution. `internal/app/keys.go` constructs the local development
  registry; the middleware copies it and resolves each key to a stable client ID.
  The committed key is an intentionally public fixture for local educational/demo
  use, not a production credential. For production, replace this registry with
  private keys loaded from protected configuration or a secret store.
  Remove or replace an entry, then rebuild/restart MCP to revoke or rotate access.
  Keys are never logged or returned by tools. There is no anonymous fallback.
- **Session ownership.** The official Go MCP SDK binds HTTP sessions to the verified
  client ID. A session ID alone grants no access. Idle sessions expire after 15 minutes;
  after restarting MCP, clients must initialize a new session. Host/Origin checks and
  a 1-MiB request-body ceiling apply at the HTTP boundary.
- **Database privileges.** Alias `shortener` uses `mcp_reader`, a dedicated nonowner,
  nonsuperuser role. Its audited `mcp_investigation.links` view omits original URLs,
  titles and creator/account identifiers. Credential, session and OAuth tables are
  inaccessible. Bootstrap revokes PUBLIC temporary-object privileges in the local
  database; the application owner keeps its privileges.
- **Validated SQL.** A real PostgreSQL parser compiled to WASM validates a closed
  grammar before execution. Use explicit columns, ordinary joins/subqueries and
  nonrecursive read-only CTEs. SELECT `*`, multiple statements, writes, locks,
  recursive CTEs and unapproved functions/types/operators are rejected. `count(*)`
  is supported. Parameter values are bound rather than interpolated. Explicit
  read-only transactions, a fixed search path, a 5-second query timeout and a
  500-ms lock timeout apply independently of parsing. Only audited view definitions
  should be added; READ ONLY alone does not make arbitrary functions safe.
- **Bounded database errors/results.** Errors carry a safe category and SQLSTATE
  when available, without raw SQL, parameters, DSNs or PostgreSQL diagnostic text.
  Results use ordered columns and row arrays, preserve NULL and report row, field
  and byte truncation. An unexhausted result reports a lower bound rather than an
  exact total.

`search_logs` and `get_request_logs` include an optional `error` field with the
underlying cause, such as a PostgreSQL DNS or connection failure. `search_errors`
includes the first nonempty diagnostic as `sample_error`, using the same
redaction and length limit. Different underlying causes do not split the existing
service/operation/error-kind/message groups. Diagnostics are output-only, so
`error` is not accepted as a service-specific filter. Missing or non-string source
errors are omitted.

`describe_service` returns `exact_filterable` for each field. Generic `search_logs`
filters accept only eligible tail fields: keyword strings, text with an exact keyword
subfield, numeric, boolean and date scalars. Elasticsearch paths are resolved from
mapping metadata; analyzed text without an exact subfield and unsupported mappings
are explicitly refused. Selected numeric/boolean values are returned as strings in
`fields`, matching the existing generic filter contract. Descriptor examples are
redacted and capped just like log text, while nonfilterable warnings are preserved.

## Configuration

| Variable | Default | Meaning |
| --- | --- | --- |
| `MCP_LOGS_TRANSPORT` | `stdio` | `stdio` or `http` |
| `MCP_LOGS_HTTP_ADDRESS` | `127.0.0.1:8090` | HTTP listen address; Compose binds inside the container and publishes on host loopback |
| `MCP_LOGS_HTTP_HOSTS` | `localhost:8090,127.0.0.1:8090` | Allowed hostnames (ports ignored) |
| `MCP_LOGS_HTTP_ORIGINS` | `http://localhost:8090,http://127.0.0.1:8090` | Exact allowed browser origins; clients may omit Origin |
| `MCP_LOGS_SHUTDOWN_TIMEOUT` | `10s` | Graceful HTTP shutdown deadline |
| `MCP_LOGS_POSTGRES_DSN` | — | Optional server-owned connection string for alias `shortener`; use the restricted reader |
| `MCP_LOGS_ELASTICSEARCH_URL` | `http://localhost:9200` | Log index endpoint |
| `MCP_LOGS_ELASTICSEARCH_INDEX` | `shortener-logs-*` | Index pattern to search |
| `MCP_LOGS_TEMPO_URL` | `http://localhost:3200` | Tempo query API |
| `MCP_LOGS_KIBANA_URL` | `http://localhost:5601` | Base URL used to build human links |
| `MCP_LOGS_SERVICES` | — | Operator allowlist. Empty means every service that ships logs |
| `MCP_LOGS_DOMAINS` | — | Domain grouping, `core:shortener,identity;data:analytics` |
| `MCP_LOGS_DISCOVERY_TTL` | `5m` | How long a discovered registry or field list stays usable |
| `MCP_LOGS_ELASTICSEARCH_USERNAME` | — | Basic auth user, if the index requires one |
| `MCP_LOGS_ELASTICSEARCH_PASSWORD` | — | Basic auth password |
| `MCP_LOGS_ELASTICSEARCH_API_KEY` | — | API key, used instead of basic auth when set |
| `MCP_LOGS_TEMPO_TOKEN` | — | Bearer token for the Tempo query API |
| `MCP_LOGS_REQUEST_TIMEOUT` | `15s` | Upstream request timeout |
| `LOG_LEVEL` | `INFO` | Log level, written to stderr |

`MCP_LOGS_SERVICES` is an operator boundary, not the vocabulary: the vocabulary
is discovered from the index. Leaving it empty exposes every service that
actually writes logs, which is also why a service that ships nothing simply does
not appear rather than reporting zero errors forever.

`MCP_LOGS_ELASTICSEARCH_INDEX` may contain `{service}`. That marks logs sharded
per service, and the placeholder is resolved per query — to `*` when a query
names no service, so the fan-out is explicit rather than accidental.

## Running over HTTP

```bash
task mcp:start       # rebuild/start MCP only; upstream outages do not block it
task mcp:db:prepare  # start PostgreSQL, apply shortener migrations, prepare reader/views
task mcp:status
task mcp:restart     # rebuild/recreate MCP only after code or key changes
task mcp:logs
task mcp:stop
```

The local endpoint is `http://localhost:8090/mcp`. `task deploy` includes MCP and
reader preparation. `/health/live` and `/health/ready` confirm transport availability;
they intentionally do not require Elasticsearch, Tempo or PostgreSQL. A down backend
fails its corresponding tool, while other tools remain available. The optional
PostgreSQL pool is created without pinging the database at startup.

Obtain your manually issued key from the operator/development registry, set
`MCP_LOGS_API_KEY` in the environment inherited by Codex, and register the HTTP server:

```bash
codex mcp add mcp-logs --url http://localhost:8090/mcp --bearer-token-env-var MCP_LOGS_API_KEY
```

Equivalent Codex configuration:

```toml
[mcp_servers.mcp-logs]
url = "http://localhost:8090/mcp"
bearer_token_env_var = "MCP_LOGS_API_KEY"
```

For the local bootstrap, an issued key can also be stored in the client's
`http_headers` configuration. The configured local Codex connection uses this form;
no raw key is included in this document. `bearer_token_env_var` is preferable when the
client already inherits a managed environment variable.

Initial connection/configuration changes may require a client reconnect. Subsequent
MCP code updates use `task mcp:restart`; clients recover a lost session by initializing
again at the same URL. A changed tool schema/catalog may still need a client refresh.
HTTP uses the SDK's MCP protocol negotiation (including revision 2025-11-25); stdio
retains its existing 2024-11-05 revision. Both transports share the tool dispatcher.

## PostgreSQL query example

After `describe_database`, call `query_database` with:

```json
{
  "database": "shortener",
  "sql": "SELECT short_code, redirect_count, expires_at FROM mcp_investigation.links WHERE redirect_count >= $1 ORDER BY redirect_count DESC",
  "parameters": [{"type": "int", "value": 5}],
  "limit": 10
}
```

Parameter types are `string`, `int`, `float`, `bool` and `null`. Use explicit columns
reported by discovery. Default ceilings are 30 returned rows (up to 100), a 600-byte
field ceiling and a 32-KiB response budget, with truncation markers and metadata. The repository withholds cells larger than 600 bytes
before retaining rows; it also caps columns at 64 and retained scan data at 32 KiB.
Discovery and query calls use only configured aliases; the caller cannot supply a DSN.

Bootstrap credentials in Compose and the reader SQL are local development fixtures.
For another deployment, supply a dedicated restricted DSN and issue separate API keys.
Network publication beyond loopback requires an explicitly configured host/origin policy
and an appropriate HTTPS boundary.

## Running over stdio

The server speaks newline-delimited JSON-RPC 2.0 on stdio. `stdout` carries the
protocol; logs go to `stderr` so they cannot corrupt the stream.

```bash
go build -o bin/mcp-logs ./services/mcp-logs/cmd/mcp-logs
```

Smoke test without an MCP client:

```bash
printf '%s\n' \
  '{"jsonrpc":"2.0","id":1,"method":"initialize"}' \
  '{"jsonrpc":"2.0","id":2,"method":"tools/list"}' \
  | ./bin/mcp-logs
```

Registering it with an MCP-capable client:

```json
{
  "mcpServers": {
    "mcp-logs": {
      "command": "/absolute/path/to/bin/mcp-logs",
      "env": {
        "MCP_LOGS_ELASTICSEARCH_URL": "http://localhost:9200",
        "MCP_LOGS_TEMPO_URL": "http://localhost:3200"
      }
    }
  }
}
```

The local stack must be running — see `deploy/docker-compose/docker-compose.yml`.

## Layout

```text
services/mcp-logs/
  cmd/mcp-logs/           entrypoint
  internal/config/        environment configuration
  internal/domain/        projected types and closed vocabularies
  internal/repository/    elasticsearch (logs), tempo (traces), postgresql (approved reads)
  internal/service/       validation, grouping, redaction, limits
  internal/transport/mcp/ stdio + authenticated Streamable HTTP, shared tools
```

## Tests

```bash
go test ./services/mcp-logs/...
```

Coverage focuses on the parts that carry policy: redaction, message
normalization, service and field validation including the near-match
suggestions, registry caching and its allowlist fallback, truncation and
sampling reporting, query shape, index routing and projection in both
repositories, domain configuration, and the protocol round trip including
notification handling, cancellation and tool-failure reporting.

## Related

- `docs/log_debugging_workflows.md` — the validated queries these tools automate
- `docs/metrics_and_tracing_workflows.md` — the tracing stack behind `get_trace`
