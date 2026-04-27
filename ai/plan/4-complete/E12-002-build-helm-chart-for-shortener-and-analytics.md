# E12-002 - Build Helm Chart For Shortener And Analytics

Status: complete

Epic: E12 - Local Kubernetes And Helm

## Summary

Add the first Helm chart that packages the `shortener` API and `analytics` runtime for local
Kubernetes deployment using the bootstrap decisions from `E12-001`.

## Scope

- create a repo-owned Helm chart for `shortener` and `analytics`
- template Deployments, Services, ConfigMaps, and required runtime environment wiring
- support locally built image repositories/tags from the `kind` image flow
- keep chart values local-first and explicit
- exclude ingress, final probe tuning, and route exposure details, which belong to `E12-003`
- exclude broad templating of PostgreSQL, Redis, Kafka, ClickHouse, Prometheus, Grafana, and Tempo
  unless `E12-001` explicitly chose that for the first pass

## Acceptance Criteria

- `helm lint` passes for the chart
- `helm template` renders Kubernetes manifests for both `shortener` and `analytics`
- chart values can select local image names and tags without editing templates
- runtime configuration is represented through values, ConfigMaps, and Secret placeholders where
  appropriate
- service boundaries remain explicit: `shortener` and `analytics` are deployed as separate
  workloads

## Assumptions

- `E12-001` has established cluster naming, namespace conventions, image loading, and dependency
  strategy
- Kubernetes manifests should mirror the existing Compose runtime shape before adding production
  features
- secrets should not be hard-coded into chart templates beyond existing local placeholder values

## Implementation Notes

- added the chart under `deploy/helm/url-shortener`
- added separate templates for `shortener` and `analytics` Deployments, Services, ConfigMaps, and
  Secrets
- added `values.yaml` for defaults and `values-kind.yaml` for local kind NodePort overrides
- chart uses locally built images `sandbox-url-shortener/shortener:local` and
  `sandbox-url-shortener/analytics:local`
- runtime configuration is value-driven; local placeholder secrets are represented through
  Kubernetes Secrets instead of hard-coded Deployment env entries
- stateful infrastructure is intentionally not templated in this first chart

## Progress Checklist

- [x] confirm scope and assumptions
- [x] create chart skeleton and values structure
- [x] add `shortener` Deployment and Service templates
- [x] add `analytics` Deployment and Service templates
- [x] wire local image names, tags, and runtime configuration
- [x] run `helm lint` and `helm template`
- [x] pass review or resolve review findings

## Validation

- `helm lint deploy/helm/url-shortener` passes
- `helm template sandbox-url-shortener deploy/helm/url-shortener --namespace sandbox-url-shortener -f deploy/helm/url-shortener/values-kind.yaml` passes
- rendered YAML parses successfully with Ruby `YAML.load_stream`
- `task k8s:deploy` builds `sandbox-url-shortener/shortener:local` and
  `sandbox-url-shortener/analytics:local`
- `task k8s:deploy` loads both images into the local `kind` cluster
- Helm release `sandbox-url-shortener` installs successfully in namespace `sandbox-url-shortener`

## Review Notes

- implementation complete with static and runtime Helm validation passing; approved

## Review Findings

### P0 Critical

- [x] no open P0 findings

### P1 High

- [x] no open P1 findings

### P2 Medium

- [x] no open P2 findings

### P3 Low

- [x] no open P3 findings
