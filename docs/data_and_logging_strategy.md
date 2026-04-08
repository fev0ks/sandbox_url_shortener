# Data And Logging Strategy

## Purpose

This document fixes the baseline decisions for how the project works with data stores, migrations,
and logs. It is more specific than the high-level system design and should be treated as the
default implementation direction unless a later task explicitly changes it.

## Final Decisions

- PostgreSQL is the only source of truth for link state
- Redis is a cache and control-plane store, not a durable system of record
- ClickHouse stores analytics events only
- SQL migrations use `golang-migrate` and live in `services/shortener/migrations/postgres`
- application logs use Go `log/slog` with JSON output to `stdout`
- log shipping uses `Fluent Bit -> Elasticsearch -> Kibana`
- request correlation is mandatory through `request_id`
- internal entity ids use PostgreSQL `uuid`
- public `short_code` values are random fixed-length codes, not derived from ids
- default internal id version is `UUIDv7`
- default short code shape is `base58` with length `7`
- there is no custom alias support in the current MVP
- the default create behavior does not deduplicate identical `original_url` values
- links support `max_clicks`, where `0` means unlimited

## Data Store Responsibilities

### PostgreSQL

Use PostgreSQL for all transactional link data:

- link creation and updates
- redirect eligibility checks
- administrative reads
- future user ownership metadata

Implementation defaults:

- driver: `pgx/v5` with `pgxpool`
- ids: PostgreSQL native `uuid`
- id generation: application-generated `UUIDv7`
- short code generation: random fixed-length code from an explicit alphabet
- write model: explicit repository methods, no ORM

PostgreSQL rules:

- every durable business fact starts in PostgreSQL
- redirect fallback always reads from PostgreSQL when Redis misses
- analytics must never require a PostgreSQL write on the redirect path beyond the link lookup

### Redis

Use Redis only for low-latency and operational concerns:

- redirect cache
- create-endpoint rate limiting
- optional idempotency keys

Redis rules:

- no business-critical state lives only in Redis
- cache lookup order is `Redis -> PostgreSQL -> Redis warm-up`
- cache failures degrade gracefully to PostgreSQL

Key conventions:

- redirect cache: `link:code:{short_code}`
- create rate limit: `rl:create:{client_key}`
- idempotency: `idem:create:{client_key}:{idempotency_key}`

Cache TTL policy:

- default redirect cache TTL: 24 hours
- if a link has `expires_at`, use the smaller of `24h` and time until expiration
- disable, enable, delete, or future update flows must invalidate the cache entry immediately

Cache and max_clicks interaction:

- the redirect cache stores the resolved target URL along with `max_clicks` and `redirect_count`
- if `max_clicks = 0`, a cache hit is sufficient to serve the redirect without hitting PostgreSQL
- if `max_clicks > 0`, the cached `redirect_count` is treated as a hint; the redirect path must
  still read and atomically increment `redirect_count` in PostgreSQL to enforce the limit correctly
- after the PostgreSQL increment, the cache entry is updated with the new `redirect_count`
- this means links with an active click limit are always confirmed against PostgreSQL, even on cache
  hit, to avoid over-serving redirects due to stale cached counts

### ClickHouse

Use ClickHouse only for append-heavy analytics data:

- raw click events written by the `analytics` service
- aggregated stats queries for API and UI

ClickHouse rules:

- the user request path never writes directly to ClickHouse
- store raw events first, optimize with materialized views only if needed later
- analytics writes must be idempotency-aware enough for at-least-once delivery

Initial table direction:

- table: `click_events_raw`
- partition: by month on `occurred_at`
- order key: `(short_code, occurred_at, event_id)`

## PostgreSQL Schema Strategy

### Primary Table: `links`

Initial table shape:

- `id UUID PRIMARY KEY`
- `short_code TEXT NOT NULL`
- `original_url TEXT NOT NULL`
- `title TEXT NULL`
- `created_by UUID NULL`
- `created_at TIMESTAMPTZ NOT NULL`
- `updated_at TIMESTAMPTZ NOT NULL`
- `expires_at TIMESTAMPTZ NULL`
- `max_clicks BIGINT NOT NULL DEFAULT 0`
- `redirect_count BIGINT NOT NULL DEFAULT 0`
- `disabled_at TIMESTAMPTZ NULL`
- `deleted_at TIMESTAMPTZ NULL`
- `last_accessed_at TIMESTAMPTZ NULL`

Why `deleted_at` is included:

- the API can expose delete semantics without physically removing link history immediately
- analytics and auditability stay intact
- future recovery or admin workflows remain possible

Initial indexes:

- unique index on active short code: `UNIQUE (short_code) WHERE deleted_at IS NULL`
- index for recent links listing: `(created_at DESC)`
- optional lookup support index if needed later: `(created_by, created_at DESC)`

Redirect eligibility rules:

- `deleted_at IS NULL`
- `disabled_at IS NULL`
- `expires_at IS NULL OR expires_at > now()`
- `max_clicks = 0 OR redirect_count < max_clicks`

Why `redirect_count` is in PostgreSQL:

- click analytics in Kafka or ClickHouse are asynchronous and cannot be the enforcement source for a
  hard redirect limit
- the limit must rely on transactional state in the primary store

### Create Flow In PostgreSQL

Create link flow should run in one transaction:

1. generate internal UUID
2. generate random `short_code`
3. try to insert the row with both values
4. on `short_code` unique violation, generate a new code and retry
5. commit on success

This keeps the internal identifier decoupled from the public URL while preserving a simple insert
path. The retry loop is only for `short_code` collisions.

### Short Code Policy

Defaults:

- fixed length: `7`
- alphabet: `123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz`
- generated with `crypto/rand`
- validated against an explicit alphabet
- reusable custom aliases may be added later, but generated codes remain the default path

Rules:

- `short_code` is immutable after creation in the MVP
- deleted links do not free a code for reuse
- collisions are handled by insert-retry on the unique index
- generated-code retries are bounded to `5` attempts before returning an internal create failure
- if collision pressure becomes operationally meaningful, increase generated code length in a later
  generator version instead of changing existing stored codes

Capacity note:

- `58^7` yields roughly `2.2` trillion possible generated codes, which is enough for the current
  project scope

### Deduplication Policy

Default behavior:

- do not deduplicate links by `original_url`
- creating the same `original_url` twice can produce two different short links

Why:

- keeps create semantics simple
- avoids surprising coupling between different user sessions or future owners
- leaves room for different TTL, disabled state, and analytics histories per link

If dedup is ever added later, it should be an explicit product decision, not implicit repository
behavior.

### Redirect Status Policy

External redirect behavior:

- active link: `302 Found`
- expired link: `410 Gone`
- max-clicks-reached link: `410 Gone`
- disabled link: `404 Not Found`
- deleted link: `404 Not Found`
- unknown short code: `404 Not Found`

Why:

- `410 Gone` is a good fit for an expired link that is intentionally no longer valid
- `404` for disabled and deleted links avoids exposing internal state distinctions to anonymous
  clients

Internal observability should still distinguish these cases through logs and metrics.

### Max Clicks Policy

Defaults:

- `max_clicks = 0` means unlimited redirects
- `redirect_count` is the authoritative enforcement counter

Rules:

- `redirect_count` increases only after a successful redirect decision
- if `max_clicks > 0` and `redirect_count >= max_clicks`, the link is no longer redirectable
- reaching the limit is treated similarly to expiration at the external HTTP level, but should be
  tracked as a separate reason in logs and metrics

### Delete Semantics

Delete is a soft delete in MVP:

- set `deleted_at`
- invalidate Redis cache
- make all read and redirect queries exclude deleted links by default

Physical cleanup, if needed later, belongs to a maintenance task, not the request path.

## Migration Strategy

Chosen tool: `golang-migrate`

Why:

- simple SQL-first workflow
- works well in local development, CI, and deploy jobs
- avoids hiding schema changes behind application boot logic

Rules:

- migrations are forward-applied explicitly, not implicitly on every app start
- each schema change gets paired `up` and `down` SQL files
- migration execution is part of CI verification and deploy flow

Repository layout:

- `services/shortener/migrations/postgres`
- future analytics DDL may live under `services/analytics/migrations/clickhouse` if needed
- ClickHouse DDL management approach (manual SQL scripts, golang-migrate, or another tool) should be
  decided when E06 work begins; until then, initial table creation can be handled by a setup script

Operational pattern:

- local: run via `make migrate-up` or a dedicated compose task
- CI: run migration validation before integration tests
- deploy: run migrations before switching traffic to the new version

## Data Access Rules In Code

- services orchestrate behavior; repositories own store-specific queries
- handlers never issue SQL, Redis, or broker commands directly
- no cross-store distributed transactions
- cache invalidation is part of the service workflow, not ad hoc handler logic
- read models for stats can use separate repository methods from write models

## Logging Strategy

### Logging Library And Output

Chosen baseline:

- Go `log/slog`
- JSON handler
- output to `stdout`
- no file-based application logs

Why:

- standard library support is enough for structured logging
- stdout is the simplest path for Docker, local development, and centralized collection
- file logs add rotation and lifecycle complexity with little value for this project

### Log Shipping

Chosen pipeline:

`shortener/analytics stdout -> Fluent Bit -> Elasticsearch -> Kibana`

Why Fluent Bit:

- lightweight and common for container log forwarding
- fits Docker Compose and VM deploy shape
- can enrich and forward JSON logs without requiring application changes

Indexing direction:

- Elasticsearch index pattern: `shortener-logs-*`
- distinguish services with a `service` field instead of separate schemas per binary

### Required Log Fields

Every structured log entry should carry a stable baseline where relevant:

- `ts`
- `level`
- `msg`
- `service`
- `env`
- `version`
- `component`
- `operation`
- `request_id`

HTTP request logs should additionally include:

- `method`
- `path`
- `route`
- `status_code`
- `latency_ms`
- `client_ip`
- `user_agent`

Domain or business logs should include when relevant:

- `short_code`
- `link_id`
- `event_id`
- `error`
- `error_kind`

### Correlation Rules

- incoming `X-Request-ID` is reused when present
- otherwise middleware generates a request id
- the request id is returned in the response headers
- redirect-published Kafka events include the same `request_id`
- analytics logs copy `request_id` from the event payload when available

This gives one correlation key across HTTP, Kafka, analytics, and Kibana searches.

### Log Level Rules

- `DEBUG`: disabled by default outside local troubleshooting
- `INFO`: service lifecycle, request summaries, link create or state-change events
- `WARN`: degraded but non-fatal conditions such as Kafka publish failure after a successful redirect
- `ERROR`: request failure, storage failure, consumer failure, or invariant break

Normal cache misses are not warnings. They are expected behavior and belong in metrics, not noisy
logs.

### What We Log And What We Avoid

Log:

- startup and shutdown events
- one summary log per request
- explicit business state changes such as link created, disabled, enabled, deleted
- async pipeline failures and retries

Do not log:

- secrets, tokens, passwords, raw config values
- full request or response bodies by default
- full `original_url` at info level if it can contain sensitive query params

If URL context is needed in logs, prefer:

- `original_url_host`
- `original_url_scheme`
- a redacted or hashed form in debug-only investigation paths

## Operational Search Patterns

Kibana queries should support:

- finding one request by `request_id`
- finding all redirect failures for one `short_code`
- finding Kafka publish degradation by `operation=publish_click_event`
- finding analytics insert failures by `event_id`

This is why `request_id`, `short_code`, `status_code`, `operation`, and `error_kind` are mandatory
design fields, not optional nice-to-haves.

## Rollout Order

- `E01`: adopt `slog`, stdout JSON logs, migration tooling choice, PostgreSQL baseline
- `E02`: implement `links` schema and repository access
- `E03`: add Redis key conventions and invalidation rules
- `E05`: propagate `request_id` into Kafka events
- `E06`: add ClickHouse raw events storage
- `E07`: add Fluent Bit, Elasticsearch, and Kibana wiring
- `E08`: add Prometheus metrics collection and Grafana dashboards
