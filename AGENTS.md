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
- read `docs/skills.md`
- read `go.mod`
- read the current entrypoint and nearby code

## Architecture Baseline

- Use a layered backend structure: `handler/router -> service -> repository`
- Keep business logic out of handlers
- Keep packages small and explicit
- Prefer simple local boundaries over premature abstractions

## Stack Baseline

- Go `1.25`
- HTTP API baseline is `net/http` with `cmd/api-http-stdlib`
- Start simple
- Do not introduce async infrastructure, caching, queues, or background jobs unless the task
  requires them

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
- Prefer explicit interfaces and simple package boundaries
- Avoid overengineering during the bootstrap phase
- After creating or modifying repo files for the current task, stage only those relevant files with
  `git add` before final handoff
- Do not stage unrelated files or user changes outside the current task

## Validation Defaults

Prefer the smallest meaningful validation first:
- `go test ./...`
- `go build ./...`

Add lint, `make` targets, or broader validation only after they exist in the repository.

## Docs Lifecycle

If behavior or architecture changes materially, update `AGENTS.md` or `docs/skills.md` in the
same task.
