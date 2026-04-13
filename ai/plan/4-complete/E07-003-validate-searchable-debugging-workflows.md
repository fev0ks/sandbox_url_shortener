# E07-003 - Validate Searchable Debugging Workflows

Status: complete

Epic: E07 - Log Observability

## Summary

Prove that the log stack and field conventions support common debugging workflows in Kibana without requiring code-level reproduction first.

## Scope

- validation scenarios for request tracing and failure investigation
- Kibana query examples or short operator guidance
- E07 close-out evidence that the log stack is usable for debugging
- excludes retention tuning and alerting rules

## Acceptance Criteria

- one request can be found end-to-end by `request_id`
- redirect failures for a `short_code` can be found quickly
- Kafka publish degradation can be found by operation-oriented log queries
- analytics insert or consumer failures can be found by `event_id`, `operation`, or `error_kind`

## Assumptions

- local smoke validation is enough before adding more formal runbooks
- a short checked-in query guide is sufficient instead of full incident documentation

## Implementation Notes

- validate against realistic scenarios rather than only checking that documents exist in Elasticsearch
- keep the outcome operator-facing: show that the stack is searchable, not just deployed
- if a search workflow fails, prefer fixing fields in code or shipping config before adding documentation workarounds

## Progress Checklist

- [x] confirm scope and assumptions
- [x] implement main code path
- [x] add or update tests
- [x] run validation
- [x] pass review or resolve review findings

## Validation

- local smoke: request trace lookup by `request_id`
- local smoke: redirect failure lookup by `short_code`
- local smoke: publish failure lookup by `operation=publish_link_visited` and `error_kind=publish_failed`
- local smoke: analytics invalid payload lookup by `service=analytics`, `operation=consumer_fetch`, `error_kind=invalid_payload`
- checked-in operator guide: `docs/log_debugging_workflows.md`
- supporting unit coverage: `go test ./internal/platform/middleware ./services/shortener/internal/app`

## Review Notes

- validated one request end-to-end by `request_id`
- validated redirect failure search by `short_code`
- validated async publish degradation search by `operation` and `error_kind`
- validated analytics invalid payload search by `service`, `operation`, and `error_kind`
- checked-in query examples now live in `docs/log_debugging_workflows.md`

## Review Findings

### P0 Critical

- [x] no open P0 findings

### P1 High

- [x] no open P1 findings

### P2 Medium

- [x] no open P2 findings

### P3 Low

- [x] no open P3 findings
