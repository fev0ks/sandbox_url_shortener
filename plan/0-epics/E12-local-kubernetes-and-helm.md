# E12 - Local Kubernetes And Helm

## Summary

Infrastructure evolution epic for moving from the default Docker Compose runtime to a local
Kubernetes workflow based on `kind` and `Helm`.

## Goal

Run the project in a local Kubernetes cluster with Helm-managed packaging so rollout, probes,
ingress, and local deployment behavior can be exercised without using cloud infrastructure.

## Scope

- local `kind` cluster setup
- Helm chart for `api-http-stdlib` and `worker`
- local values files and Helm install or upgrade workflow
- ingress, service, probes, config, and image wiring for local Kubernetes
- excludes external cloud providers and managed Kubernetes targets

## Key Decisions

- Docker Compose remains the default early-stage runtime
- `kind + Helm` becomes the later local production-like runtime
- Minikube is optional and remains outside the main path
- local Kubernetes work starts only after the Compose-based stack and observability loops are stable

## Task Links

- `E12-001-prepare-kind-cluster-bootstrap.md`
- `E12-002-build-helm-chart-for-api-and-worker.md`
- `E12-003-wire-local-ingress-probes-and-config.md`
- `E12-004-document-kind-and-helm-local-deploy-flow.md`

## Open Questions

- whether infrastructure dependencies should stay outside the main chart or be partially templated
- whether local TLS is worth adding in the first kind milestone
