# E09-002 - Add Image Build And Quality Gates

Status: complete

Epic: E09 - Delivery And Local Automation

## Summary

Add a repeatable local image-build flow for `shortener`, `analytics`, and `web`, then connect it
to lightweight quality gates so the repo can verify not only source builds but also the runnable
container artifacts used by local deploys.

## Scope

- standardize image build commands for all shipped services
- add local automation entrypoints for image builds and any required pre-build checks
- ensure the image build flow works with the current Docker-first repository layout
- exclude publishing images to a registry
- exclude multi-environment release pipelines and remote CI wiring

## Acceptance Criteria

- one documented local command builds all service images used by the compose stack
- failed image builds surface clearly enough to identify the broken service
- the image build flow depends on or references the local verify flow from `E09-001`
- local docs explain when to use source builds versus container builds
- the resulting automation does not require hand-editing service-specific Docker commands

## Assumptions

- local Docker image builds are sufficient for this phase; no registry push is needed yet
- service Dockerfiles remain owned by each service and are not consolidated into a shared build
  system
- the compose stack is the source of truth for which images must build successfully

## Implementation Notes

- prefer a single automation entrypoint that loops over the existing service Dockerfiles rather
  than bespoke one-off commands in docs
- keep image-build gates narrow and reproducible; avoid introducing buildx, provenance, or
  multi-arch complexity at this stage
- if build time becomes painful, optimize later; first make the flow explicit and repeatable
- keep the automation readable so future CI wiring in E09 can reuse it directly

## Progress Checklist

- [x] confirm scope and assumptions
- [x] define the canonical local image build commands
- [x] add automation entrypoints for building all runtime images
- [x] connect image builds to the local quality-gate story
- [x] document the workflow and expected outputs
- [x] run validation
- [x] pass review or resolve review findings

## Validation

- `task image-build`
- docker output confirms `docker-compose-shortener`, `docker-compose-analytics`, and `docker-compose-web` build successfully
- [docs/local_delivery_workflows.md](docs/local_delivery_workflows.md) documents `task image-build` as the artifact gate before deploy

## Review Notes

- `task image-build` now runs `task verify` first, then builds all runtime images used by the local stack
- the workflow stays Docker-first and reuses the existing service-owned Dockerfiles
- no new tests were added for this task by request

## Review Findings

### P0 Critical

- [x] no open P0 findings

### P1 High

- [x] no open P1 findings

### P2 Medium

- [x] no open P2 findings

### P3 Low

- [x] no open P3 findings
