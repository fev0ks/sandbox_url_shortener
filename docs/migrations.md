# Migrations

This project uses [golang-migrate](https://github.com/golang-migrate/migrate) to manage PostgreSQL
schema changes. Migrations are explicit SQL files that run outside the application process.

For the canonical local verify and deploy workflow, use
[Local Delivery Workflows](local_delivery_workflows.md).

## Prerequisites

Install the `migrate` CLI. Pick one method:

```bash
# macOS (Homebrew)
brew install golang-migrate

# Go install
go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest

# Docker (no local install needed)
# See the "Docker Compose" section below.
```

Verify:

```bash
migrate -version
```

## Directory Structure

```text
services/
  shortener/
    migrations/
      postgres/
        000001_create_links_table.up.sql
        000001_create_links_table.down.sql
```

Each schema change is a pair of files:

- `NNNNNN_description.up.sql` — applies the change
- `NNNNNN_description.down.sql` — reverts the change

The six-digit prefix determines execution order. Use sequential numbering starting from `000001`.

## Running Migrations

### Local (CLI)

Start PostgreSQL first if it is not already running:

```bash
task deploy   # canonical local deploy flow
```

or start only PostgreSQL if you are running migrations manually:

```bash
docker compose -f deploy/docker-compose/docker-compose.yml up postgres -d
```

Apply all pending migrations:

```bash
make migrate-up
```

This runs:

```bash
migrate -path services/shortener/migrations/postgres \
        -database "postgres://shortener:shortener@localhost:5432/shortener?sslmode=disable" \
        up
```

### Local (Docker Compose)

A one-shot `migrate` service is included in `docker-compose.yml`.

- `task deploy` (preferred) brings the stack up and triggers the `migrate` service before
  `shortener` starts
- `make run` uses the same compose flow when you want the lower-level helper

You can also invoke the migration runner directly:

```bash
docker compose -f deploy/docker-compose/docker-compose.yml run --rm migrate
```

### Rolling Back

`migrate-down` is a local development recovery tool. Do not treat it as a production rollback
strategy.

Roll back the last migration:

```bash
make migrate-down
```

This runs:

```bash
migrate -path services/shortener/migrations/postgres \
        -database "postgres://shortener:shortener@localhost:5432/shortener?sslmode=disable" \
        down 1
```

The `1` means "revert one migration". Omit it to revert all (dangerous in production).

### Migrating To A Specific Version

```bash
migrate -path services/shortener/migrations/postgres \
        -database "$DB_DSN" \
        goto <version>
```

Where `<version>` is the numeric prefix (e.g. `1` for `000001`).

### Checking Current Version

```bash
migrate -path services/shortener/migrations/postgres \
        -database "$DB_DSN" \
        version
```

## Creating A New Migration

```bash
migrate create -ext sql -dir services/shortener/migrations/postgres -seq <description>
```

Example:

```bash
migrate create -ext sql -dir services/shortener/migrations/postgres -seq add_user_table
```

This generates:

```
services/shortener/migrations/postgres/000002_add_user_table.up.sql
services/shortener/migrations/postgres/000002_add_user_table.down.sql
```

Edit both files before applying.

## Dirty State

If a migration fails halfway, golang-migrate marks the database as "dirty". You will see:

```
error: Dirty database version <N>. Fix and force version.
```

To recover:

1. Manually fix or revert the partial changes in the database.
2. Force the version to the last known good state:

```bash
migrate -path services/shortener/migrations/postgres \
        -database "$DB_DSN" \
        force <last_good_version>
```

For example, if version 2 failed and version 1 was clean:

```bash
migrate force 1
```

Then re-run `migrate up` after fixing the SQL.

## Rules

These rules come from the project's [data and logging strategy](data_and_logging_strategy.md):

- migrations are forward-applied explicitly, never implicitly on app boot
- each schema change gets paired `up` and `down` SQL files
- migration execution is part of CI verification and the deploy flow
- do not put DML (INSERT, UPDATE, DELETE of application data) in migration files
- keep each migration small and focused on one logical change
- test both `up` and `down` locally before committing

## Operational Pattern

| Environment | How migrations run |
|---|---|
| Local dev | `task deploy`, `make run`, or manual `make migrate-up` / docker-compose `migrate` |
| CI | Run migrations before integration tests to validate schema |
| Deploy | Run migrations before switching traffic to the new version |
