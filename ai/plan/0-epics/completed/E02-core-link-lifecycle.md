# E02 - Core Link Lifecycle

Status: completed

## Summary

Core product epic for creating, storing, resolving, and administrating short links without any async
infrastructure.

## Goal

Deliver a working URL shortener backend on PostgreSQL with create, read, enable, disable, delete,
and redirect flows covered by meaningful tests.

## Scope

- `Link` domain model and PostgreSQL schema
- short code generation and link persistence
- HTTP API for create, get, enable, disable, and delete
- redirect handler with PostgreSQL as the source of truth
- integration coverage for happy path and critical failure states
- excludes Redis cache, Kafka, ClickHouse, and browser UI

## Key Decisions

- use PostgreSQL as the primary source of truth for links
- use UUIDv7 as the internal link identifier
- generate public short codes as random base58 values with length `7`
- do not support custom aliases in the current MVP
- create does not deduplicate identical `original_url` values by default
- support `max_clicks` with default `0 = unlimited`
- use `302 Found` for active redirects, `410 Gone` for expired links, and `404 Not Found` for
  disabled, deleted, or unknown links
- use `410 Gone` when the redirect limit is reached
- keep handler, service, and repository boundaries explicit
- use soft delete semantics with `deleted_at` for the MVP delete flow
- increment `redirect_count` atomically in PostgreSQL during redirect for `max_clicks` enforcement

## Task Links

- `E02-001-design-link-model-and-postgres-migrations.md`
- `E02-002-implement-link-create-and-read-api.md`
- `E02-003-implement-enable-disable-delete-flows.md`
- `E02-004-implement-redirect-resolution-and-tests.md`

## Open Questions

- whether custom aliases should be added only after user identity and ownership exist
