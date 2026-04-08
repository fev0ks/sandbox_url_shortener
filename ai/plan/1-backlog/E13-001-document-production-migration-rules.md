# E13-001 - Document Production Migration Rules

Status: backlog

Epic: E13 - Production Grade Migrations

## Summary

Expand `docs/migrations.md` with the production-grade operational rules: forward-only deploys,
expand/contract patterns, lock and timeout discipline, `CREATE INDEX CONCURRENTLY` rule, dirty
state recovery runbook, and schema-change checklist. Mark `migrate-down` in the Makefile as
dev-only.

## Scope

- new sections in `docs/migrations.md`: forward-only rule, expand/contract, locks/timeouts,
  index concurrency, dirty recovery, schema-change checklist
- update Makefile comment on `migrate-down` to label it dev-only
- excludes any actual CI/CD wiring
- excludes any tool change

## Acceptance Criteria

- `docs/migrations.md` has a "Production Operations" section covering all six topics above
- the doc references the senior-golang-study migration ops doc as the deeper learning source
- Makefile `migrate-down` target has a `DEV ONLY` comment and a clearly marked help string
- the schema-change checklist is reachable from a top-level heading in `docs/migrations.md`
- nothing in the codebase changes besides docs and Makefile comments

## Assumptions

- the senior-golang-study doc at `LearnGoLang/docs/senior-golang-study/06-databases/migrations-in-go.md`
  is the authoritative learning reference and may be linked from this project's docs
- Russian-language learning notes are out of scope for this repo's docs; this repo stays English

## Implementation Notes

- add the new sections to `docs/migrations.md` after the existing "Operational Pattern" section
- keep tone consistent with the rest of the docs (terse, rule-oriented, no hedging)
- the checklist should be a Markdown checkbox list so it can be referenced from PR descriptions
- do not duplicate the deep explanations from the learning doc — keep this doc operational

## Progress Checklist

- [ ] confirm scope and assumptions
- [ ] add Production Operations section to `docs/migrations.md`
- [ ] add schema-change checklist subsection
- [ ] update Makefile `migrate-down` help text and comment
- [ ] cross-link to the learning doc
- [ ] run validation (no code, just doc lint)
- [ ] pass review or resolve review findings

## Validation

- `markdownlint docs/migrations.md` if available, otherwise visual review
- `make help` (or visual Makefile diff) shows the dev-only label on migrate-down

## Review Notes

- pending

## Review Findings

### P0 Critical

- [x] no open P0 findings

### P1 High

- [x] no open P1 findings

### P2 Medium

- [x] no open P2 findings

### P3 Low

- [x] no open P3 findings
