# E09-003 - Define Local Deploy And Migration Runner

Status: complete

Epic: E09 - Delivery And Local Automation

## Summary

Define one repeatable local deploy flow that starts the stack, applies PostgreSQL migrations in a
predictable way, and leaves the project in a usable state for manual testing and smoke checks.

## Scope

- choose and automate the canonical local deploy command for the current Docker Compose stack
- make migration execution an explicit part of local deploy instead of an implied manual step
- reconcile docs with the actual compose/runtime behavior around migrations
- add smoke-level verification for the deployed stack becoming healthy
- exclude production-grade migration safety rules from E13
- exclude Kubernetes deployment flow from E12

## Acceptance Criteria

- one documented command brings up the local stack and applies pending shortener migrations in the
  intended order
- contributors do not need to remember a separate undocumented migration step during normal local
  deploys
- `docs/migrations.md`, compose config, and automation commands agree on how migrations run
- the deploy flow includes a minimal post-deploy smoke check for service health
- rollback expectations for failed local deploys are clear enough to support `E09-004`

## Assumptions

- Docker Compose remains the local deploy substrate for this phase
- running migrations before traffic hits the app is sufficient for the current bootstrap-stage
  schema
- it is acceptable for the first version to target only the shortener PostgreSQL migrations

## Implementation Notes

- start by fixing the current mismatch: `docs/migrations.md` mentions a compose `migrate` service,
  but the compose file does not currently define one
- acceptable shapes include a one-shot compose migration service, a Task/Make wrapper, or a
  deploy script, as long as the contract is explicit
- prefer deterministic sequencing over magic retries; healthchecks and dependency ordering should
  make startup behavior obvious
- keep migration failure handling visible so local recovery docs can point to concrete commands

## Progress Checklist

- [x] confirm scope and assumptions
- [x] choose the canonical local deploy and migration execution shape
- [x] implement the automation and config changes
- [x] align migration and deploy docs with the implemented flow
- [x] add minimal smoke verification for healthy local startup
- [x] run validation
- [x] pass review or resolve review findings

## Validation

- `docker compose -f deploy/docker-compose/docker-compose.yml config`
- `task deploy`
- `task smoke`
- validation output shows the one-shot `migrate` service exits successfully before `shortener` becomes healthy

## Review Notes

- `deploy/docker-compose/docker-compose.yml` now includes a one-shot `migrate` service
- `shortener` waits for the migration runner to complete successfully before startup
- `task deploy` now runs compose up plus reusable smoke checks for `shortener`, `analytics`, and `web`
- [docs/migrations.md](docs/migrations.md) now matches the implemented compose flow instead of referencing a missing service

## Review Findings

### P0 Critical

- [x] no open P0 findings

### P1 High

- [x] no open P1 findings

### P2 Medium

- [x] no open P2 findings

### P3 Low

- [x] no open P3 findings
