# E12-001 - Prepare kind Cluster Bootstrap

Status: complete

Epic: E12 - Local Kubernetes And Helm

## Summary

Define and add the local `kind` bootstrap baseline needed to run project-owned services in a
repeatable Kubernetes cluster without replacing the Docker Compose default workflow.

## Scope

- add repo-owned `kind` cluster configuration for local development
- define namespace, local image-loading, and cluster lifecycle commands
- decide the first-pass strategy for infrastructure dependencies during local Kubernetes runs
- add Taskfile or Makefile entrypoints only when they make the workflow repeatable
- exclude Helm chart implementation for application services, which belongs to `E12-002`
- exclude cloud Kubernetes, managed registries, and production cluster concerns

## Acceptance Criteria

- a documented local command creates a predictable `kind` cluster for this project
- cluster creation does not disturb the existing Docker Compose default runtime
- locally built project images can be made available to the `kind` cluster
- the initial dependency strategy is explicit: Compose-backed dependencies, chart-managed
  dependencies, or a documented hybrid
- cleanup/reset commands or instructions are available for deleting the local cluster

## Assumptions

- Docker Compose remains the default runtime while `kind + Helm` is introduced as an optional
  production-like local workflow
- local Kubernetes should avoid external cloud services and managed registries
- first-pass dependency handling should stay simple and reversible

## Implementation Notes

- added `deploy/kind/cluster.yaml` for a stable local cluster named `sandbox-url-shortener`
- mapped kind NodePorts to host ports `18080` for `shortener` and `19090` for `analytics`
- added Taskfile cluster lifecycle commands: `k8s:cluster:create`, `k8s:cluster:delete`, and
  `k8s:reset`
- added `k8s:dependencies` to start Compose-backed PostgreSQL, Redis, and ClickHouse and run the
  shortener migration service before app deployment
- added `k8s:images` to build local `shortener` and `analytics` images and load them into kind
- dependency decision: first-pass local Kubernetes runs app workloads in kind while stateful
  infrastructure remains in Compose

## Progress Checklist

- [x] confirm scope and assumptions
- [x] choose the local cluster name, namespace, and config location
- [x] add `kind` cluster config and lifecycle commands
- [x] add local image build/load flow for project services
- [x] document the dependency strategy for local Kubernetes
- [x] run validation
- [x] pass review or resolve review findings

## Validation

- `task --list` shows the new `k8s:*` commands
- `ruby -e 'require "yaml"; YAML.load_file("deploy/kind/cluster.yaml")'` passes
- `brew install kind` installed `kind v0.31.0`
- `task k8s:cluster:create` created `sandbox-url-shortener` and switched to
  `kind-sandbox-url-shortener`
- `kubectl wait --for=condition=Ready node/sandbox-url-shortener-control-plane --timeout=120s`
  passes

## Review Notes

- implementation complete and approved

## Review Findings

### P0 Critical

- [x] no open P0 findings

### P1 High

- [x] no open P1 findings

### P2 Medium

- [x] no open P2 findings

### P3 Low

- [x] no open P3 findings
