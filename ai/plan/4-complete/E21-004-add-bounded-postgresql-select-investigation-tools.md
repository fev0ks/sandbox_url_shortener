# Bounded PostgreSQL SELECT Investigation Tools

Status: complete

Epic: E21 - MCP Agent Investigation Tools

## Summary

Expose approved PostgreSQL diagnostic views through bounded, parameterized read-only SELECT tools.

## Scope

- Fixed tools list_databases, describe_database and query_database.
- Alias shortener, dedicated reader role, audited mcp_investigation.links view.
- Real PostgreSQL AST validation through libpg_query compiled to WASM (CGO disabled).
- SELECTs, ordinary joins, subqueries and nonrecursive read-only CTEs within a closed grammar.
- Explicit read-only transactions, deterministic search path, timeouts and result bounds.

## Acceptance Criteria

- Queries bind typed parameters and return useful diagnostic columns without credentials or PII.
- Deny sensitive relations, SQL writes, locks, unapproved functions/types/operators and aliases.
- Preserve NULLs and duplicate column names with ordered metadata and row arrays.
- Report row/field/byte truncation and avoid exact totals when results are not exhausted.
- Database failures expose safe categories/SQLSTATE; other tools and runtime remain usable.
- Actual PostgreSQL checks verify reader grants and read-only transactions independently of parsing.

## Assumptions

- Initial view excludes original URLs, titles and creator/account identifiers.
- Local bootstrap credentials are development fixtures and remain server-owned configuration.

## Implementation Notes

- Parser github.com/wasilibs/go-pgquery pinned to v0.0.0-20260915022521-81f99195012b.
- Pools are constructed lazily without startup ping.
- Defaults: 30 rows, ceiling 100, 600-byte fields, 32-KiB response, 5s query and 500ms lock timeout.
- Bootstrap is idempotent and can run against existing Compose volumes.

## Progress Checklist

- [x] confirm scope and assumptions
- [x] implement main code path
- [x] add or update tests
- [x] run validation
- [x] pass review or resolve review findings

## Validation

- `task test`, `task build`, and `task lint`: passed with `GOTOOLCHAIN=go1.25.0`; lint reports zero issues.
- Focused tests with CGO disabled and actual restricted-reader integration passed.
- Real HTTP discovery returned ten approved columns; bound SELECTs, JOINs and nonrecursive CTEs passed.
- Reader independently denied writes, sensitive relations and TEMP; privileged DSNs are rejected. Timeout cleanup preserved pool reuse.
- AST policy, typed int64/NULL handling, scoped transaction deadlines, UTF-8/redaction, row/field/byte budgets and safe SQLSTATE regressions passed.
- Actual 64-column oversized numeric query withheld cells and returned a bounded response with explicit field/byte truncation and an unknown total.
- `task mcp:db:prepare` succeeded twice against the existing local volume.

## Review Notes

- Final changed-files review: APPROVED; all blocking findings resolved with regressions.

## Review Findings

### P0 Critical

- [x] no open P0 findings

### P1 High

- [x] no open P1 findings

### P2 Medium

- [x] partial initialization cannot hold shutdown indefinitely; active calls are cancelled before closing sessions
- [x] transaction-scoped deadlines reach query and result reads
- [x] column/field/byte limits apply while scanning, before oversized values can accumulate

### P3 Low

- [x] no open P3 findings
