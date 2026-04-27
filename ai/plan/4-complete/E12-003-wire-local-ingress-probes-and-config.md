# E12-003 - Wire Local Access Probes And Config

Status: complete

Epic: E12 - Local Kubernetes And Helm

## Summary

Complete the local Kubernetes runtime wiring for HTTP access, health probes, service readiness,
and environment configuration so the Helm deployment is usable for manual testing.

## Scope

- add local NodePort access for reaching `shortener` and the `analytics` admin API
- configure Kubernetes liveness and readiness probes for `shortener` and `analytics`
- align Helm values with existing service environment variables and dependency endpoints
- document or encode local-only hostnames and ports
- exclude production TLS, cert-manager, external DNS, and cloud load balancers
- exclude dashboard and load-test changes unless required to validate the Kubernetes runtime

## Acceptance Criteria

- `shortener` is reachable through the chosen local Kubernetes access path
- readiness and liveness probes use existing service health endpoints and reflect real startup
  dependencies
- `analytics` exposes or reports readiness consistently with its existing admin health endpoint
- local values clearly show dependency endpoints for PostgreSQL, Redis, Kafka, ClickHouse, Tempo,
  and other required services
- no secret or token values are committed as real credentials

## Assumptions

- `E12-002` has created the base Helm chart and local values file
- local TLS is deferred unless the implementation finds a concrete blocker without it
- dependency endpoints may point to Compose services, cluster services, or local bridge addresses
  according to the `E12-001` dependency decision

## Implementation Notes

- selected NodePort access with kind `extraPortMappings` instead of requiring an ingress controller
  for the first local Kubernetes pass
- `shortener` is exposed at `localhost:18080` through NodePort `30080`
- `analytics` is exposed at `localhost:19090` through NodePort `30090`
- configured liveness and readiness probes against existing `/health/live` and `/health/ready`
  endpoints for both workloads
- configured local dependency endpoints through Helm values using `host.docker.internal`
- disabled Kafka producer/consumer paths by default because the current Compose broker advertises
  `localhost:29092`, which is not pod-reachable as advertised metadata

## Progress Checklist

- [x] confirm scope and assumptions
- [x] choose local NodePort access shape
- [x] add probe configuration for `shortener`
- [x] add probe configuration for `analytics`
- [x] wire dependency endpoint values for local Kubernetes
- [x] validate manifests for HTTP access and pod readiness configuration
- [x] pass review or resolve review findings

## Validation

- `helm template sandbox-url-shortener deploy/helm/url-shortener --namespace sandbox-url-shortener -f deploy/helm/url-shortener/values-kind.yaml` renders NodePort services and probes
- rendered YAML parses successfully with Ruby `YAML.load_stream`
- `task k8s:deploy` successfully rolls out `deployment/sandbox-url-shortener-shortener`
- `task k8s:deploy` successfully rolls out `deployment/sandbox-url-shortener-analytics`
- `kubectl -n sandbox-url-shortener get pods,svc,deploy -o wide` shows both pods `1/1 Running`
  and both deployments `1/1` available
- `task k8s:smoke` passes against `http://localhost:18080/health/ready` and
  `http://localhost:19090/health/ready`

## Review Notes

- implementation complete with runtime smoke validation passing; approved

## Review Findings

### P0 Critical

- [x] no open P0 findings

### P1 High

- [x] no open P1 findings

### P2 Medium

- [x] no open P2 findings

### P3 Low

- [x] no open P3 findings
