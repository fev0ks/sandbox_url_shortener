# E06-004 - Expose Stats Via API And UI

Status: complete

Epic: E06 - Analytics And Stats

## Summary

Expose link stats through the analytics API and render them on the web details page.

## Scope

- HTTP stats endpoint in `analytics`
- typed stats client in `services/web`
- stats section on link details page
- excludes advanced charts or dashboard-wide analytics pages

## Acceptance Criteria

- analytics API exposes basic stats for a single link
- web details page renders stats from the API
- empty-stats state is explicit and does not break the page
- UI stays aligned with eventual consistency expectations

## Assumptions

- stats belong on the existing link details page before broader analytics dashboards
- simple textual and grid rendering is enough before dedicated charting work

## Implementation Notes

- keep the endpoint narrow and link-scoped
- UI should tolerate missing or not-yet-populated stats gracefully
- do not over-promise freshness; stats are eventually consistent

## Progress Checklist

- [x] confirm scope and assumptions
- [x] implement main code path
- [x] add or update tests
- [x] run validation
- [x] pass review or resolve review findings

## Validation

- `npm run typecheck`
- `npm run build`
- `npm run test`
- `go build ./services/analytics/cmd/analytics`
- local smoke: link details page renders API stats

## Review Notes

- API exposure is `GET /api/v1/links/{id}/stats` on `analytics`, which keeps ClickHouse ownership inside the analytics service while preserving the existing details-page route model.
- The web details page now renders a tolerant analytics section: stats failures do not break link details, and empty analytics state is explicit.
- Vitest coverage now fixes the client-side contract around the separate analytics base URL and stats error mapping.

## Review Findings

### P0 Critical

- [x] no open P0 findings

### P1 High

- [x] no open P1 findings

### P2 Medium

- [x] no open P2 findings

### P3 Low

- [x] no open P3 findings
