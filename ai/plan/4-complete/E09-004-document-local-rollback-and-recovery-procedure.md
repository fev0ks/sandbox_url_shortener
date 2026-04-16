# E09-004 - Document Local Rollback And Recovery Procedure

Status: complete

Epic: E09 - Delivery And Local Automation

## Summary

Document the operator runbook for backing out a broken local deploy and recovering the stack after
common failures such as bad config, failed migrations, unhealthy containers, and dirty local state.

## Scope

- write a local runbook covering verify, deploy, rollback, and recovery paths
- document the common failure modes introduced by the local automation from `E09-001` to `E09-003`
- define the minimum commands and checkpoints needed to return the stack to a known-good state
- exclude remote environment rollback and incident escalation policy
- exclude new automation unless documentation reveals a clear blocking gap

## Acceptance Criteria

- a repo doc exists with step-by-step local rollback and recovery procedures
- the runbook covers at least: failed verify run, failed image build, failed migration, unhealthy
  compose service, and full stack reset
- the doc references the canonical verify and deploy commands introduced by earlier E09 tasks
- a contributor can follow the runbook without needing chat history or tribal knowledge
- the runbook clearly distinguishes destructive recovery steps from safe routine checks

## Assumptions

- documentation is the right first move; full automated rollback is not required at this stage
- local state resets using Docker volumes are acceptable in this repo's current learning-oriented
  environment
- the runbook can link to existing migration docs rather than duplicating every low-level detail

## Implementation Notes

- prefer one operator-facing doc over scattering recovery notes across multiple files
- structure the runbook by failure mode and recovery objective, not by tool
- destructive commands such as full volume resets should be visually separated and explained
- reuse the terminology from the implemented automation so the doc reads like an operational
  companion, not a parallel universe

## Progress Checklist

- [x] confirm scope and assumptions
- [x] inventory the local failure and rollback scenarios introduced by E09 automation
- [x] write the local rollback and recovery runbook
- [x] cross-link the runbook from the relevant automation and migration docs
- [x] run validation
- [x] pass review or resolve review findings

## Validation

- doc walkthrough against [docs/local_delivery_workflows.md](docs/local_delivery_workflows.md)
- cross-check [docs/migrations.md](docs/migrations.md) links and commands against the new Taskfile / compose flow
- `task deploy` and `task smoke` confirm the documented happy-path commands are valid

## Review Notes

- [docs/local_delivery_workflows.md](docs/local_delivery_workflows.md) now serves as the operator-facing runbook
- the runbook covers failed verify, failed image build, failed migration, unhealthy service, and full stack reset
- destructive recovery is separated from safe checks via `task stop` versus `task reset`

## Review Findings

### P0 Critical

- [x] no open P0 findings

### P1 High

- [x] no open P1 findings

### P2 Medium

- [x] no open P2 findings

### P3 Low

- [x] no open P3 findings
