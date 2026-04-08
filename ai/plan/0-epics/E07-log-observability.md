# E07 - Log Observability

## Summary

Observability epic focused on searchable centralized logs for troubleshooting request and event
flows.

## Goal

Make application logs available in Elasticsearch and Kibana with enough structured fields to trace
requests, redirects, and failures.

## Scope

- Elasticsearch and Kibana in local stack
- log shipping pipeline from applications
- structured fields such as `request_id`, `short_code`, `status_code`, `latency_ms`, and `error`
- search and investigation readiness for common failure scenarios
- excludes metric dashboards and alerting logic

## Key Decisions

- log entries remain JSON at the application boundary
- request correlation fields must be present across shortener and analytics flows
- observability fields should support incident debugging without logging sensitive data
- use `Fluent Bit -> Elasticsearch -> Kibana` as the default log pipeline

## Task Links

- `E07-001-add-elasticsearch-kibana-and-log-shipping.md`
- `E07-002-standardize-request-and-domain-log-fields.md`
- `E07-003-validate-searchable-debugging-workflows.md`

## Open Questions

- how much retention policy setup belongs to MVP versus deploy documentation
