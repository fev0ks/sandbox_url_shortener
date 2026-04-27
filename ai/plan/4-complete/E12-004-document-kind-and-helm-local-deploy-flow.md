# E12-004 - Document kind And Helm Local Deploy Flow

Status: complete

Epic: E12 - Local Kubernetes And Helm

## Summary

Document the complete local `kind + Helm` workflow and add any final task entrypoints needed so a
future session can build images, create the cluster, deploy workloads, smoke test, and clean up.

## Scope

- add or update docs for the local Kubernetes workflow
- document prerequisites, cluster bootstrap, image loading, Helm install or upgrade, smoke checks,
  logs, troubleshooting, and cleanup
- align Taskfile commands with the documented workflow
- call out the relationship between Docker Compose and `kind + Helm`
- exclude new runtime features beyond documentation and small automation polish
- exclude production deployment docs for cloud or managed Kubernetes

## Acceptance Criteria

- docs provide an end-to-end command sequence for local Kubernetes deployment
- docs explain when to use Compose versus `kind + Helm`
- smoke-check commands or manual checks are concrete and match implemented routes/ports
- troubleshooting covers common local failures: missing images, unhealthy pods, dependency
  endpoints, and ingress or port-forward issues
- cleanup instructions restore the local machine to a predictable state

## Assumptions

- `E12-001`, `E12-002`, and `E12-003` have established the implemented workflow
- Compose remains the default local runtime and should not be described as deprecated
- documentation should be concise enough to stay operational rather than tutorial-heavy

## Implementation Notes

- added `docs/local_kubernetes_workflows.md` with prerequisites, deploy flow, access URLs, useful
  commands, Helm values, troubleshooting, and Compose relationship
- linked the Kubernetes workflow from `docs/local_delivery_workflows.md`
- updated `docs/system_design.md` so the architecture description matches the implemented first
  pass: app workloads in kind, stateful dependencies in Compose
- kept command names synchronized with the new `Taskfile.yaml` `k8s:*` tasks

## Progress Checklist

- [x] confirm scope and assumptions
- [x] create or update local Kubernetes workflow docs
- [x] align Taskfile command names and docs
- [x] add troubleshooting and cleanup sections
- [x] run static validation and capture remaining runtime blockers
- [x] pass review or resolve review findings

## Validation

- `task --list` shows documented Kubernetes helper commands
- `task k8s:template` renders the documented Helm release command
- `helm lint deploy/helm/url-shortener` passes
- `task k8s:deploy` completes cluster use, dependency startup, image build/load, Helm install,
  rollout waiting, and smoke checks
- manual `curl -fsS http://localhost:18080/health/ready` returns `{"status":"ok"}`
- manual `curl -fsS http://localhost:19090/health/ready` returns `{"status":"ok"}`
- cleanup command is documented but intentionally not executed so the validated local cluster
  remains available

## Review Notes

- implementation complete with documented deploy and smoke flow validated; approved

## Review Findings

### P0 Critical

- [x] no open P0 findings

### P1 High

- [x] no open P1 findings

### P2 Medium

- [x] no open P2 findings

### P3 Low

- [x] no open P3 findings
