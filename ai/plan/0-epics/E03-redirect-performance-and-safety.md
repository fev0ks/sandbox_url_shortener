# E03 - Redirect Performance And Safety

## Summary

Performance and resilience epic for protecting the hot path and the create path after the core
backend works end to end.

## Goal

Reduce redirect latency and database pressure with Redis cache-aside while adding basic protective
controls on write endpoints.

## Scope

- Redis cache-aside for redirect lookups
- cache warm-up and invalidation for link state changes
- rate limiting on link creation
- optional idempotency support for create requests
- excludes event streaming and analytics storage

## Key Decisions

- keep redirect lookup order `Redis -> PostgreSQL -> Redis warm-up`
- cache invalidation must happen on disable, enable, update, and delete paths
- protective controls should not complicate the public API more than needed
- idempotency remains optional until there is a concrete consumer need

## Task Links

- `E03-001-add-redis-cache-aside-to-redirect-flow.md`
- `E03-002-implement-cache-invalidation-for-link-state-changes.md`
- `E03-003-add-create-endpoint-rate-limiting.md`
- `E03-004-evaluate-and-add-idempotency-key-support.md`

## Open Questions

- whether rate limiting should be per IP only or also support authenticated users later

## Resolved Questions

- TTL and cache key shape defaults are defined in `docs/data_and_logging_strategy.md`:
  key `link:code:{short_code}`, TTL 24h (or min of 24h and time until expiration)
