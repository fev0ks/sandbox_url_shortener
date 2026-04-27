# Local Delivery Workflows

This document is the operator-facing runbook for local verification, image builds, deploys,
rollback, and recovery.

## Canonical Commands

Use these commands as the default local workflow:

```bash
task verify
task image-build
task deploy
```

Supporting commands:

```bash
task smoke
task stop
task reset
```

`Makefile` remains available for narrower runtime and migration helpers such as `make migrate-up`
and `make migrate-version`, but `Taskfile.yaml` is the canonical entrypoint for verification and
local delivery.

The optional local Kubernetes workflow is documented separately in
[Local Kubernetes Workflows](local_kubernetes_workflows.md). Use it when you specifically want to
exercise `kind + Helm`; Docker Compose remains the default local runtime.

## Verify

Run:

```bash
task verify
```

This flow runs:

- Go lint via `golangci-lint`
- Go tests
- Go builds for `shortener` and `analytics`
- the Next.js web build

Use it before building images or deploying the local stack.

## Build Runtime Images

Run:

```bash
task image-build
```

This command first runs `task verify`, then builds the runtime images for:

- `shortener`
- `analytics`
- `web`

Use it when you want an artifact gate without starting the stack.

## Deploy The Local Stack

Run:

```bash
task deploy
```

This command:

1. runs `docker compose up --build -d`
2. lets the one-shot `migrate` service apply PostgreSQL migrations
3. starts the runtime services after dependencies become ready
4. executes HTTP smoke checks against `shortener`, `analytics`, and `web`

Low-level helper:

```bash
make run
```

`make run` uses the same compose stack, but `task deploy` is the preferred command because it
includes smoke verification.

## Smoke Checks

Re-run smoke checks without redeploying:

```bash
task smoke
```

This checks:

- `http://localhost:8080/health/ready`
- `http://localhost:9090/health/ready`
- `http://localhost:${WEB_PORT:-3000}`

## Rollback And Recovery

The local workflow is recovery-oriented. If a deploy goes bad, restore a known-good local state,
inspect the failing component, then redeploy.

### Failed Verify Run

Symptoms:

- `task verify` fails in lint, tests, Go build, or web build

Recovery:

1. rerun the failing stage directly: `task lint`, `task test`, `task build`, or `task web-build`
2. fix the underlying issue
3. rerun `task verify`

### Failed Image Build

Symptoms:

- `task image-build` fails while building one service image

Recovery:

1. inspect the failing service output from the build log
2. rerun the service build directly if needed:

```bash
docker compose -f deploy/docker-compose/docker-compose.yml build shortener
docker compose -f deploy/docker-compose/docker-compose.yml build analytics
docker compose -f deploy/docker-compose/docker-compose.yml build web
```

3. rerun `task image-build`

### Failed Migration During Deploy

Symptoms:

- `task deploy` fails smoke checks
- `shortener` never becomes ready
- `docker compose ps` shows `migrate` exited with an error

Recovery:

1. inspect the migration logs:

```bash
docker compose -f deploy/docker-compose/docker-compose.yml logs migrate
```

2. inspect the current migration state:

```bash
make migrate-version
```

3. fix the migration SQL or the database state
4. rerun the migration manually if needed:

```bash
docker compose -f deploy/docker-compose/docker-compose.yml run --rm migrate
```

5. rerun `task deploy`

For local-only rollback while debugging, `make migrate-down` is available, but treat it as a
development escape hatch, not as a general deploy strategy.

### Unhealthy Compose Service

Symptoms:

- `task smoke` fails
- a service remains in `starting`, `unhealthy`, or `exited`

Recovery:

1. inspect service state:

```bash
docker compose -f deploy/docker-compose/docker-compose.yml ps
```

2. inspect logs for the failing service:

```bash
docker compose -f deploy/docker-compose/docker-compose.yml logs shortener
docker compose -f deploy/docker-compose/docker-compose.yml logs analytics
docker compose -f deploy/docker-compose/docker-compose.yml logs web
```

3. fix the failing dependency or configuration
4. rerun `task deploy` or `task smoke`

### Full Stack Reset

Use this only when the local stack is too dirty to recover incrementally.

Safe routine teardown:

```bash
task stop
```

Destructive reset:

```bash
task reset
```

`task reset` removes containers and volumes. PostgreSQL, Redis, ClickHouse, Prometheus, Grafana,
and Elasticsearch local state will be lost.

After a reset:

```bash
task deploy
```
