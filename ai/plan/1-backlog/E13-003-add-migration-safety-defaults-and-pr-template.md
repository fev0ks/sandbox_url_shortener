# E13-003 - Add Migration Safety Defaults And PR Template

Status: backlog

Epic: E13 - Production Grade Migrations

## Summary

Establish two safety nets that catch unsafe migrations before they hit a real database:
(1) a migration template that bakes `lock_timeout` and `statement_timeout` into every new
migration, and (2) a schema-change pull-request template with the expand/contract checklist.
Retroactively update the existing `000001_create_links_table` migration to include the same
safety defaults.

## Scope

- new file `services/shortener/migrations/postgres/.template.up.sql` with the safety preamble
- update `migrate-create` Make target (or document a wrapper) so new migrations include the
  preamble automatically
- update `services/shortener/migrations/postgres/000001_create_links_table.up.sql` to include `lock_timeout` and
  `statement_timeout`
- new file `.github/PULL_REQUEST_TEMPLATE/schema-change.md` with the checklist
- documentation update in `docs/migrations.md` referencing both
- excludes lint tools (squawk, atlas migrate lint) — separate follow-up
- excludes CI enforcement of the PR template — relies on contributor discipline for now

## Acceptance Criteria

- new migrations created via `make migrate-create NAME=...` include the safety preamble at
  the top of the up file
- `000001_create_links_table.up.sql` starts with `SET lock_timeout = '5s'; SET statement_timeout = '60s';`
- `make migrate-up` against a fresh database still applies cleanly with the preamble in place
- `db/schema.sql` does not change after adding the preamble (SET statements are session-local
  and do not appear in pg_dump output)
- `.github/PULL_REQUEST_TEMPLATE/schema-change.md` exists with a checklist covering: forward-only,
  expand/contract for drops/renames, lock_timeout, statement_timeout, CONCURRENTLY for indexes,
  no NOT NULL without default, batched backfills, rollout plan
- `docs/migrations.md` references both the template file and the PR template

## Assumptions

- `golang-migrate create` does not natively support a template flag, so the wrapper is a Make
  target that copies the template file into the new migration after running `migrate create`
- the existing CI/PR setup (none yet) will pick up `.github/PULL_REQUEST_TEMPLATE/` once a CI
  pipeline is wired in E09
- `SET` statements in a migration apply only to the current connection and do not leak across
  migrations

## Implementation Notes

- safety preamble (target file `services/shortener/migrations/postgres/.template.up.sql`):

  ```sql
  -- Safety defaults — do not remove without understanding lock impact.
  SET lock_timeout = '5s';
  SET statement_timeout = '60s';

  -- Migration starts below.
  ```

- updated Make target sketch:

  ```makefile
  ## migrate-create: create a new migration pair with safety preamble
  migrate-create:
  	migrate create -ext sql -dir $(MIGRATE_PATH) -seq $(NAME)
  	@latest_up=$$(ls -1 $(MIGRATE_PATH)/*_$(NAME).up.sql | tail -1); \
  	cat $(MIGRATE_PATH)/.template.up.sql $$latest_up > $$latest_up.tmp && \
  	mv $$latest_up.tmp $$latest_up
  ```

- PR template skeleton:

  ```markdown
  ## Schema change

  Brief description.

  ### Pre-flight checklist
  - [ ] forward-only — no destructive change in same release as code that depends on it
  - [ ] expand/contract for drops/renames/type changes
  - [ ] `lock_timeout` set
  - [ ] `statement_timeout` set
  - [ ] `CREATE INDEX CONCURRENTLY` for any index on a growing table
  - [ ] no `NOT NULL` without `DEFAULT` on existing rows
  - [ ] backfill batched (or N/A)
  - [ ] `db/schema.sql` regenerated and committed
  - [ ] rollout plan for partial failure documented
  ```

- update `000001_create_links_table.up.sql` to prepend the preamble; verify `make migrate-up`
  still works on a fresh DB

## Progress Checklist

- [ ] confirm scope and assumptions
- [ ] add `services/shortener/migrations/postgres/.template.up.sql`
- [ ] update `migrate-create` Make target to inject the template
- [ ] update `000001_create_links_table.up.sql` with the safety preamble
- [ ] add `.github/PULL_REQUEST_TEMPLATE/schema-change.md`
- [ ] update `docs/migrations.md` with references
- [ ] verify `make migrate-up` still works on a clean DB
- [ ] verify `db/schema.sql` did not change (depends on E13-002)
- [ ] run validation
- [ ] pass review or resolve review findings

## Validation

- create a throwaway migration via `make migrate-create NAME=test_template`, verify the up
  file starts with the safety preamble, then delete it
- `docker compose down -v && make run && make migrate-up` succeeds with preamble in place
- `make schema-check` (from E13-002) still passes after the preamble update

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
