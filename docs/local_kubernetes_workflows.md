# Local Kubernetes Workflows

This document describes the optional local `kind + Helm` runtime for exercising Kubernetes
deployment behavior. Docker Compose remains the default local development and delivery runtime.

## Runtime Shape

The first local Kubernetes pass deploys only project-owned application workloads into `kind`:

- `shortener`
- `analytics`

Infrastructure dependencies stay in Docker Compose:

- PostgreSQL
- Redis
- ClickHouse

This keeps the Helm chart focused on application packaging. It also avoids prematurely templating
stateful infrastructure before the project needs a full in-cluster data plane.

Kafka producer and consumer paths are disabled in the default `kind` values. The current Compose
Kafka broker advertises `localhost:29092`, which is correct for host clients but not for clients
inside Kubernetes pods. Re-enable Kafka for the Kubernetes runtime only after adding a pod-reachable
advertised listener.

## Prerequisites

Install these tools:

```bash
kind
kubectl
helm
docker
task
```

The local cluster config lives at:

```text
deploy/kind/cluster.yaml
```

The Helm chart lives at:

```text
deploy/helm/url-shortener
```

## Deploy

Run the full local Kubernetes deployment:

```bash
task k8s:deploy
```

This command:

1. starts Compose-backed dependencies: PostgreSQL, Redis, and ClickHouse
2. runs the shortener PostgreSQL migration service
3. creates the `sandbox-url-shortener` kind cluster if it does not exist
4. builds local `shortener` and `analytics` images
5. loads those images into kind
6. runs `helm upgrade --install`
7. waits for both Kubernetes Deployments to roll out

## Access

The kind cluster maps stable host ports to NodePort services:

| Service | Host URL |
| --- | --- |
| shortener | `http://localhost:18080` |
| analytics | `http://localhost:19090` |

Smoke check:

```bash
task k8s:smoke
```

Manual checks:

```bash
curl -fsS http://localhost:18080/health/ready
curl -fsS http://localhost:19090/health/ready
```

## Useful Commands

Render the chart without applying it:

```bash
task k8s:template
```

Show Kubernetes resources:

```bash
task k8s:status
```

Inspect service logs:

```bash
task k8s:logs
```

## Generate Pod Logs

Use the local kind NodePort to create shortener traffic when you want logs to inspect with
`kubectl logs`.

Health endpoints are intentionally skipped in access logs, so use create and redirect API traffic
instead.

Generate a mixed batch:

```bash
task k8s:generate-logs
```

The task creates successful links, follows their redirects, requests missing short codes, and sends
extra create attempts to exercise rate-limit logs. Tune the batch size when needed:

```bash
LOG_CREATE_COUNT=5 LOG_MISSING_COUNT=100 LOG_RATE_LIMIT_COUNT=50 task k8s:generate-logs
```

Inspect the shortener logs:

```bash
kubectl -n sandbox-url-shortener logs deploy/sandbox-url-shortener-shortener | grep link_create
kubectl -n sandbox-url-shortener logs deploy/sandbox-url-shortener-shortener | grep redirect
kubectl -n sandbox-url-shortener logs deploy/sandbox-url-shortener-shortener | grep not_found
kubectl -n sandbox-url-shortener logs deploy/sandbox-url-shortener-shortener | grep rate_limited
```

Tail logs while sending traffic:

```bash
kubectl -n sandbox-url-shortener logs -f deploy/sandbox-url-shortener-shortener
```

The default kind values disable Kafka producer and consumer paths, so this traffic is intended for
shortener pod log inspection. Analytics pod logs are still available, but redirect events will not
flow into analytics until Kafka is enabled for the local Kubernetes runtime.

Delete the Helm release and kind cluster:

```bash
task k8s:reset
```

Delete only the cluster:

```bash
task k8s:cluster:delete
```

## Helm Values

Default chart values are in:

```text
deploy/helm/url-shortener/values.yaml
```

Local kind overrides are in:

```text
deploy/helm/url-shortener/values-kind.yaml
```

The local values use `host.docker.internal` to reach Compose-backed dependencies from pods. This is
the expected path on Docker Desktop. If that name is not available in your Docker runtime, override
the dependency hosts with an address reachable from the kind node containers.

## Troubleshooting

### Missing Local Images

Symptoms:

- pods stay in `ImagePullBackOff`
- `kubectl describe pod` mentions `sandbox-url-shortener/shortener:local` or
  `sandbox-url-shortener/analytics:local`

Recovery:

```bash
task k8s:images
kubectl -n sandbox-url-shortener rollout restart deploy/sandbox-url-shortener-shortener
kubectl -n sandbox-url-shortener rollout restart deploy/sandbox-url-shortener-analytics
```

### Dependency Endpoint Failure

Symptoms:

- `shortener` is not ready
- `analytics` is not ready
- logs show PostgreSQL, Redis, or ClickHouse connection failures

Recovery:

```bash
task k8s:dependencies
docker compose -f deploy/docker-compose/docker-compose.yml ps postgres redis clickhouse
task k8s:status
task k8s:logs
```

If the dependency host is not reachable from kind, override the relevant Helm values before running
`helm upgrade --install`.

### Port Mapping Failure

Symptoms:

- pods are ready but `task k8s:smoke` cannot reach `localhost:18080` or `localhost:19090`

Recovery:

```bash
kind get clusters
kubectl -n sandbox-url-shortener get svc
docker ps --filter name=sandbox-url-shortener-control-plane
```

If the cluster was created before `deploy/kind/cluster.yaml` had the port mappings, recreate it:

```bash
task k8s:reset
task k8s:deploy
```

## Relationship To Compose

Use Compose for the default development loop:

```bash
task deploy
task smoke
```

Use `kind + Helm` when the goal is to validate Kubernetes packaging, probes, rollout behavior, and
local production-like deployment mechanics:

```bash
task k8s:deploy
task k8s:smoke
```
