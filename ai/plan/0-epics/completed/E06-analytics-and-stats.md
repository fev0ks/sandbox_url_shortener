# E06 - Analytics And Stats

Status: complete

## Summary

Analytics epic for turning click events into queryable statistics owned by `analytics` and surfaced to the UI.

## Goal

Persist raw click events in ClickHouse and expose basic per-link statistics through the analytics
service for operational and user views.

## Scope

- ClickHouse in local stack
- raw click events table and ingestion path from `analytics`
- stats API in `analytics` for link detail pages
- baseline aggregates such as total clicks, time buckets, and referrer views
- excludes advanced BI modeling and complex pre-aggregation pipelines

## Key Decisions

- store raw events first and optimize aggregation later if needed
- keep the analytics ingestion contract simple and append-oriented
- expose only the minimum useful stats needed by MVP UI and API
- keep ClickHouse ownership inside `analytics`; `shortener` should not read analytics storage

## Task Links

- `E06-001-add-clickhouse-to-local-stack-and-schema.md`
- `E06-002-write-analytics-events-to-clickhouse.md`
- `E06-003-add-basic-stats-query-layer.md`
- `E06-004-expose-stats-via-api-and-ui.md`

## Open Questions

- whether geolocation enrichment belongs to MVP analytics or stays deferred
- whether stats queries should read directly from raw tables at MVP scale
- whether ClickHouse local bootstrap should become fully database-name-configurable beyond the
  current `analytics` default
- when raw event writes should switch from one-row inserts to buffered batches
