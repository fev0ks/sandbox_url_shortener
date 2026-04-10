# E19 - End-To-End API System Testing

Status: active

## Summary

Testing epic for teaching the repo how to run real end-to-end system checks against the running
stack.

## Goal

Create a Go-driven API e2e harness that validates public HTTP contracts across multiple services
in a realistic environment.

## Scope

- compose-based test environment bootstrapping
- fixture or seed strategy
- Go-driven API e2e scenarios
- auth, ownership, admin authorization, analytics, and realtime API checks
- documentation for how to add new e2e tests in this repo
- excludes browser automation and visual UI testing

## Key Decisions

- first e2e wave is API-only, not Playwright or browser-driven
- e2e tests should validate the public edge contract, ideally via gateway
- `E09` remains delivery automation; this epic owns system-level end-to-end testing
- realtime API verification should cover SSE behavior once `E18` lands

## Task Links

- `E19-001-bootstrap-compose-based-e2e-test-harness.md`
- `E19-002-add-auth-and-session-e2e-scenarios.md`
- `E19-003-add-owned-links-and-analytics-e2e-scenarios.md`
- `E19-004-add-admin-capability-e2e-scenarios.md`
- `E19-005-document-how-to-write-new-api-e2e-tests.md`

## Open Questions

- whether e2e runs are opt-in local only at first or part of `task verify`
- whether seeded fixtures should be SQL-first or API-driven
