# Skills Memory

This file is the project-local memory for future reusable skill creation and use.

## Repo Rule

- In this repo, agents provide role and context isolation
- Skills provide reusable methodology
- Repo docs provide project truth

## `go-system-design`

### When To Use

Use for feature planning, backend decomposition, API design, storage design, and task shaping
before implementation.

### Responsibilities

- clarify the goal and scope
- propose simple interfaces and boundaries
- identify risks, assumptions, and open decisions
- define validation expectations and test shape

### Non-Goals

- no code writing
- no final code review

### Trigger Phrases

- "design this feature"
- "plan the backend"
- "how should we structure this"
- "define the API and storage"
- "what is the simplest architecture for this"

### Typical Inputs

- feature request
- product idea
- storage or API constraint
- current repo structure and existing code

### Expected Outputs

- short implementation plan for small tasks
- decision-complete design outline for medium or large tasks
- clear assumptions, risks, and validation approach

### Read First In This Repo

- `AGENTS.md`
- `docs/skills.md`
- `go.mod`
- current entrypoint and relevant packages

### Must Avoid

- writing code
- overengineering
- introducing distributed-system complexity without an explicit need
- making product decisions silently when a short assumption would be clearer

## `go-backend-implementation`

### When To Use

Use for implementing Go services, handlers, repositories, tests, and config wiring after the
design direction is clear.

### Responsibilities

- translate an approved plan into code
- keep layering clear
- add focused tests and run narrow validation
- update project memory docs when behavior or architecture changes materially

### Non-Goals

- no architecture ownership
- no final sign-off

### Trigger Phrases

- "implement this"
- "add the endpoint"
- "wire the service"
- "write the handler and repository"
- "add tests for this flow"

### Typical Inputs

- approved plan or design note
- concrete bug or feature request
- existing Go packages and interfaces

### Expected Outputs

- code changes aligned with repo conventions
- focused tests for non-trivial logic
- concise summary of changed files and validations run

### Read First In This Repo

- `AGENTS.md`
- `docs/skills.md`
- approved architect output
- relevant code paths

### Must Avoid

- skipping the architect pass for non-trivial work
- bloated abstractions
- mixing business logic into handlers
- adding dependencies without clear value

## `go-code-review`

### When To Use

Use for changed-files review after implementation.

### Responsibilities

- find blocking issues
- catch regressions and boundary violations
- check risky contracts, missing tests, and maintainability problems

### Non-Goals

- no rewriting the change
- no architecture replanning unless a blocking flaw is found

### Trigger Phrases

- "review this change"
- "do a code review"
- "check for regressions"
- "is this safe to merge"

### Typical Inputs

- changed files or diff
- intended behavior
- validation evidence when available

### Expected Outputs

- blocking findings only when they are real
- optional non-blocking notes
- final verdict: `APPROVED` or `NEEDS CHANGES`

### Read First In This Repo

- `AGENTS.md`
- `docs/skills.md`
- changed files
- any task or design context linked to the change

### Must Avoid

- style-only nitpicks
- broad redesign suggestions unless required for correctness
- reviewing untouched areas as if they were part of the task

## Locked Bootstrap Conventions

- Agent names are `architect`, `engineer`, and `reviewer`
- Framework baseline is stdlib-first HTTP backend via `cmd/api-http-stdlib`
- Skills are specified in repo docs, not implemented as repo-local skill folders
- There is no `builder` or `qa` role at bootstrap stage
- `docs/skills.md` is the single project-local source of truth for future skill creation and use
