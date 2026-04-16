# E09-001 - Build Local Verify Flow For Lint Test And Build

Status: complete

Epic: E09 - Delivery And Local Automation

## Summary

Turn the existing ad-hoc local quality commands into one reliable verification flow built around
`Taskfile.yaml`, so a contributor can run one command and get the expected lint, test, and build
signal for the repository.

## Scope

- align `Taskfile.yaml` with the repo's real quality gates for Go services and the web app
- make `task verify` the canonical local verification entrypoint referenced by repo docs
- ensure bootstrap commands install required local tooling deterministically
- exclude container image build and deploy orchestration
- exclude migration safety policy changes from E13

## Acceptance Criteria

- `task verify` runs the repo's intended local quality gate without manual prerequisites beyond Go,
  Node, Docker, and Task
- the verification flow covers Go lint, Go tests, Go builds, and the web build or an explicit
  documented reason why a web check is deferred
- `Taskfile.yaml` is the primary quality gate entrypoint and repo docs point to it instead of
  competing wrappers
- the command output is understandable enough that a contributor can tell which stage failed
- the flow is reproducible on a clean local checkout

## Assumptions

- `Taskfile.yaml` remains the canonical local automation surface, while `Makefile` can keep
  narrower runtime and migration helpers
- `services/web` stays part of the local verification story because it is a first-class service in
  the repo
- formatting can stay as a separate explicit step unless the verify flow clearly benefits from
  enforcing it

## Implementation Notes

- start from the existing `Taskfile.yaml` tasks instead of introducing another automation tool
- remove ambiguity between `Makefile` and `Taskfile.yaml`: quality gates should live in Task,
  runtime helpers can stay in Make
- keep the first version simple: explicit sequential tasks are preferable to clever wrappers
- if the web build is too heavy for every verify run, split the flow into `verify-go` and
  `verify-web` and make `verify` compose both intentionally

## Progress Checklist

- [x] confirm scope and assumptions
- [x] audit the current `Taskfile.yaml` coverage and identify missing quality checks
- [x] update the Task tasks for the intended local verify flow
- [x] document the canonical verification command in repo docs
- [x] run validation
- [x] pass review or resolve review findings

## Validation

- `task --list`
- `task verify`
- visual check: [docs/local_delivery_workflows.md](docs/local_delivery_workflows.md) now points to `task verify` as the canonical quality gate

## Review Notes

- `Taskfile.yaml` now exposes explicit `verify-go`, `verify-web`, and `verify` stages
- `task verify` is green and runs lint, Go tests, Go builds, and the web build sequentially
- no new tests were added for this task by request; the existing validation flow is reused

## Review Findings

### P0 Critical

- [x] no open P0 findings

### P1 High

- [x] no open P1 findings

### P2 Medium

- [x] no open P2 findings

### P3 Low

- [x] no open P3 findings
