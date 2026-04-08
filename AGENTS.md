# AGENTS.md

## Purpose

This repository is a Go backend project for a URL shortener and is currently at the bootstrap
stage.

## Scope

These instructions apply to the whole repository unless a nested `AGENTS.md` is explicitly added
later.

## Start Here

Before making changes:
- read `AGENTS.md`
- read `ai/skills.md`
- read `go.mod`
- read the current entrypoint and nearby code

## Architecture Baseline

- Use a layered backend structure: `handler/router -> service -> repository`
- Keep business logic out of handlers
- Keep packages small and explicit
- Prefer simple local boundaries over premature abstractions
- Treat `services/<service>/` as the ownership boundary for service-specific code and assets
- Keep shared cross-cutting infrastructure only under `internal/platform`

## Stack Baseline

- Go `1.25`
- HTTP API baseline is `net/http` with `services/shortener/cmd/shortener`
- Background analytics runtime lives in `services/analytics/cmd/analytics`
- Start simple
- Do not introduce async infrastructure, caching, queues, or background jobs unless the task
  requires them

## Non-Negotiable Architecture Rules

- Handlers must not call repositories or execute SQL
- Services own business logic, orchestration, transaction boundaries, cache strategy, and cache
  invalidation
- Repositories contain persistence logic only and must not contain transport concerns or business
  workflows
- Avoid fat handlers; handlers are transport adapters only
- Do not introduce global mutable state, hidden singletons, or package-level runtime state
- Prefer explicit code over reflection-heavy or runtime-magic designs

## Layer Responsibilities

### Handler Layer

- Parse and validate HTTP input and request metadata
- Call service methods only
- Map domain/application errors to HTTP responses
- Return transport-safe JSON responses only
- Do not contain business decisions or persistence logic

### Service Layer

- Implement use-case logic and orchestration
- Own transaction boundaries and side-effect ordering
- Coordinate repository calls and cache behavior
- Keep methods testable in isolation via dependency injection
- Keep nesting low by extracting private helpers when needed

### Repository Layer

- Encapsulate SQL queries, row scanning, and backend-specific storage details
- Return typed/domain-friendly errors for service handling
- Do not contain HTTP concerns or cross-service workflows

## Repository Layout Rules

- Keep service-owned config in `services/<service>/internal/config`
- Keep service-owned migrations in `services/<service>/migrations`
- Keep service-owned image builds in `services/<service>/Dockerfile`
- Keep `deploy/` for orchestration only: `docker-compose`, Helm, kind, manifests, env overlays
- Do not add new repo-root `migrations/`, `Dockerfile`, or shared config packages unless the
  artifact is truly cross-service infrastructure

## Role Policy

- `architect` is the first pass for non-trivial work
- `engineer` implements approved work
- `reviewer` is the final pass for any file-changing task

## When To Use Which Agent

- Use `architect` for planning, task shaping, architecture decisions, API and storage design, and
  deciding how a non-trivial change should be implemented.
- Use `engineer` after the direction is clear and the task is ready for implementation.
- Use `reviewer` after code or docs changed and you need a final changed-files safety review.

Default flow for non-trivial work:
- `architect -> engineer -> reviewer`

Default flow for very small clear changes:
- `engineer -> reviewer`

## Engineering Rules

- Use `context.Context` first on public boundaries where applicable
- Wrap errors with `%w`
- Do not use `context.Background()` in business logic
- Do not log secrets, tokens, or sensitive payloads
- Do not use panic for expected runtime errors
- Prefer explicit interfaces and simple package boundaries
- Avoid overengineering during the bootstrap phase
- After creating, renaming, deleting, or modifying repo files for the current task, stage all
  those relevant paths before final handoff
- Prefer `git add -A <path>...` when the task includes new files, deletions, or renames so the
  index matches the actual tree, not just the modified files
- Do not stage unrelated files or user changes outside the current task

## Logging Rules

- Use the project structured logger only
- `fmt.Println`, `log.Print*`, and ad-hoc console logging are forbidden
- Include stable fields such as request id, entity id, operation, and duration where relevant
- Keep logs actionable and low-noise

## Coding Standards

- Public exported types and functions should have GoDoc comments in English
- Code comments must be in English and explain intent, not obvious syntax
- Keep functions focused; split orchestration, mapping, and side effects when they start to mix
- Keep ownership and side effects explicit in function signatures
- Prefer pure helpers for mapping and derivation logic when possible

## Contract Constants And Redis Rules

- Raw string literals for Redis key prefixes, idempotency namespaces, header names used as
  contracts, and other reused external-facing keys should be declared as `const`
- Reusing the same Redis or contract key as a string literal in multiple places is forbidden
- Redis is an optimization layer, not the source of truth
- Define explicit TTL for every Redis key
- Use namespaced Redis keys and keep the naming format explicit in code
- Cache and idempotency failures must degrade gracefully unless the task explicitly requires
  fail-closed behavior

## Testability And Review Rules

- Business logic must be testable without real infrastructure
- Put external dependencies behind interfaces when that meaningfully improves service-level tests
- New features must include unit tests for success path, edge cases, and failure paths
- Every bug fix should include a regression test
- Non-trivial behavior branches must be covered by unit tests

## Self-Review Checklist

- Handler contains no repository or SQL calls
- Service owns business logic and side-effect ordering
- Repository contains only persistence concerns
- Errors are wrapped and mapped correctly
- Structured logging is used; no plain prints
- No global mutable state or hidden singleton was introduced
- Redis keys, TTL, and invalidation strategy are explicit where cache is used
- New behavior branches have tests

## Validation Defaults

Prefer the smallest meaningful validation first:
- `task test`
- `task build`
- `task lint`

Use `task format` when Go files need formatting.

Keep `Makefile` for runtime and migration workflows. Use `Taskfile.yaml` as the default local
quality gate entrypoint.

## Docs Lifecycle

If behavior or architecture changes materially, update `AGENTS.md` or `ai/skills.md` in the
same task.
