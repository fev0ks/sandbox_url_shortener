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
- `ai/skills.md`
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
- `ai/skills.md`
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
- `ai/skills.md`
- changed files
- any task or design context linked to the change

### Must Avoid

- style-only nitpicks
- broad redesign suggestions unless required for correctness
- reviewing untouched areas as if they were part of the task

## Locked Bootstrap Conventions

- Agent names are `architect`, `engineer`, and `reviewer`
- Framework baseline is stdlib-first HTTP backend via `services/shortener/cmd/shortener`
- Background event and analytics runtime uses `services/analytics/cmd/analytics`
- Skills are specified in repo docs, not implemented as repo-local skill folders
- There is no `builder` or `qa` role at bootstrap stage
- `ai/skills.md` is the single project-local source of truth for future skill creation and use
- MCP log tools project optional underlying `error` diagnostics and grouped `sample_error`;
  both are redacted and capped by the service before output. Diagnostics are core output
  fields, not service-specific filters, and do not alter error grouping keys.

- MCP supports stdio and authenticated Streamable HTTP via the official Go SDK. HTTP sessions
  belong to the client ID resolved from an injected immutable issued-key registry. The local
  registry is constructed in services/mcp-logs/internal/app/keys.go; updates require MCP rebuild.
- The independent MCP Compose runtime publishes only on loopback at port 8090. Health checks
  test the transport, not upstream availability. Pools are lazy; a PostgreSQL outage must not
  prevent logs/trace tools from serving. Task mcp:* commands operate only on the MCP runtime,
  except explicit mcp:db:prepare which provisions the local reader and audited views.
- Database investigation uses three fixed tools and server-owned aliases/DSNs. The service
  validates PostgreSQL ASTs through a WASM parser, controls read-only transaction scope and
  bounds results. Repositories own pool/transaction mechanics, safe SQLSTATE errors and scanning.
  Only mcp_investigation.links is initially approved; no URLs, titles or account identifiers.
  SQL writes, locks, SELECT star projections and unaudited functions/types/operators are denied.
  Reader grants, a fixed search path, timeout/cancellation and transaction READ ONLY are required
  independently of parsing. Results use ordered columns/row arrays and explicit truncation.

- describe_service projects copied descriptor metadata: examples/notes are redacted and
  length-capped without mutating discovery caches. Repository warnings remain visible alongside
  cardinality hints. Generic tail filters require exact_filterable metadata; Elasticsearch
  resolves physical exact-match paths from mapping information and preserves scalar examples
  and selected values as strings. Analyzed text without an exact subfield is refused.
- The committed local MCP key is an intentionally public educational/demo fixture, not a
  production credential. Public repository examples must make this distinction explicit.
