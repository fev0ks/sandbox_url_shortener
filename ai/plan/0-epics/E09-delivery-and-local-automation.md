# E09 - Delivery And Local Automation

## Summary

Delivery epic for making the project reproducible through local verification, build, and deployment
automation.

## Goal

Automate build, test, image creation, local deploy commands, and migration execution so the
project can be verified and operated locally in a repeatable way.

## Scope

- local verification flow for lint, tests, build, and quality gates
- container image build
- local deploy commands and migration execution
- documented local runbook for verify, deploy, rollback, and recovery
- excludes external cloud deployment and complex multi-environment release management

## Key Decisions

- keep delivery automation aligned with the Docker-first repo shape
- local deploy must include migration handling, not rely on manual database drift fixes
- optimize for repeatable local workflows before introducing external CI providers

## Task Links

- `E09-001-build-local-verify-flow-for-lint-test-and-build.md`
- `E09-002-add-image-build-and-quality-gates.md`
- `E09-003-define-local-deploy-and-migration-runner.md`
- `E09-004-document-local-rollback-and-recovery-procedure.md`

## Open Questions

- whether local image distribution should stay Docker-native or use a small local registry later
- how much rollback automation should be scripted versus documented for the local workflow
