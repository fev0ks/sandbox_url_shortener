# Plan Board

This directory is the project-local task board for planned and in-flight work.

Use it to keep implementation intent, current status, and completion evidence inside the
repository so a future Codex or human session can resume work without relying on chat history.

## Stages

- `0-epics`
  Active long-lived product or architecture tracks that group related tasks.
- `0-epics/completed`
  Completed epics kept as lightweight history after their task set is done.
- `1-backlog`
  Tasks that are not started yet.
- `2-active`
  Tasks currently being designed or implemented.
- `3-review`
  Tasks that are implemented and waiting for validation, review, or final fixes.
- `4-complete`
  Tasks that are finished and kept as lightweight project history.

The numeric prefix is part of the workflow. Keep it so folders sort in execution order.

## How To Use The Board

- One active epic = one Markdown file in `0-epics`.
- Move a finished epic into `0-epics/completed`.
- One task = one Markdown file.
- Move the file between stage folders to reflect status.
- Keep the same filename across stages.
- Epic files should use names such as `E01-core-redirect-flow.md` or
  `E02-admin-and-analytics.md`.
- Task files should include the epic number in the filename, for example:
  `E01-001-create-short-link-endpoint.md`
  `E01-002-resolve-and-redirect-by-code.md`
  `E02-001-basic-click-analytics.md`
- Start small. Split a task only when it becomes hard to review or resume.

## Relationship To Other Docs

- `AGENTS.md` defines repo-wide engineering rules and role workflow.
- `docs/skills.md` defines which reusable skill or role mindset applies.
- `plan/0-epics` captures the bigger product or architecture intent.
- `plan/0-epics/completed` keeps finished epics out of the active planning surface.
- `plan/` tracks concrete tasks and their current state.

Do not duplicate repo rules in every task file. Link to them when needed.

## Epics And Tasks

Epics and tasks have different purposes:

- Epics describe the larger feature area, architecture slice, or product capability.
- Tasks describe a concrete unit of execution that can move through the board.

Rules:

- Active epic files stay in `0-epics`.
- Completed epic files move to `0-epics/completed`.
- Task files move between `1-backlog`, `2-active`, `3-review`, and `4-complete`.
- Each task should reference exactly one primary epic unless there is a strong reason not to.
- The epic number must appear both in the task filename and near the top of the task file.

Move an epic to `0-epics/completed` when:

- its intended scope is finished or intentionally closed
- its linked tasks are complete or explicitly abandoned
- its open questions are resolved, deferred elsewhere, or no longer relevant

## Epic File Structure

Use the template in `plan/templates/epic.md`.

Each epic file should contain:

1. `Status`
   Usually `active` or `completed`.
2. `Summary`
   What this big area is and why it exists.
3. `Goal`
   The intended product or technical outcome.
4. `Scope`
   What this epic includes and excludes.
5. `Key Decisions`
   Important chosen constraints, defaults, or architecture direction.
6. `Task Links`
   Related task files grouped under this epic.
7. `Open Questions`
   Meaningful unresolved decisions that affect future tasks.

## Recommended Task File Structure

Use the template in `plan/templates/task.md`.

Each task file should contain these sections:

1. `Epic`
   The primary epic id and title, for example `E01 - Core Redirect Flow`.
2. `Summary`
   A short statement of the goal and intended outcome.
3. `Scope`
   What is included and, when useful, what is explicitly excluded.
4. `Acceptance Criteria`
   Concrete conditions that make the task done.
5. `Assumptions`
   Defaults or open constraints the implementation is relying on.
6. `Implementation Notes`
   Planned or actual technical approach, affected boundaries, and important decisions.
7. `Progress Checklist`
   A checkbox-based list of concrete steps or milestones for the task.
8. `Validation`
   Commands run, checks planned, or remaining verification.
9. `Review Notes`
   Final review blockers, follow-ups, or approval outcome.
10. `Review Findings`
   Prioritized review findings tracked with checkboxes.

Keep sections short. The goal is resumability, not documentation theater.

## What Changes By Stage

### `1-backlog`

At minimum, the file should have:
- `Epic`
- `Summary`
- `Scope`
- `Acceptance Criteria`
- `Assumptions`

`Implementation Notes`, `Validation`, and `Review Notes` can stay as placeholders.

### `2-active`

The file should be decision-complete enough for implementation:
- `Implementation Notes` should describe the chosen approach
- `Progress Checklist` should show current activity through checked and unchecked items
- `Validation` should list the intended checks

If a non-trivial task starts, this file should reflect the architect pass before coding continues.

### `3-review`

The file should capture the completion state:
- `Implementation Notes` should reflect what was actually done
- `Validation` should list executed checks and their outcome
- `Review Notes` should list blockers or explicitly say review is pending
- `Review Findings` should group active findings by priority and show resolution progress with
  checkboxes
- `Progress Checklist` should be mostly complete, with any remaining unchecked items explained

Only move a task here when implementation is substantially complete.

### `4-complete`

The file should be clean project history:
- `Acceptance Criteria` should be satisfied
- `Validation` should show completed checks
- `Review Notes` should record approval or final resolution of blocking issues
- `Review Findings` should be fully resolved, trimmed to only relevant history, or explicitly show
  that no findings remained
- `Progress Checklist` should be fully checked or intentionally trimmed to the completed path

## Progress Checklist Format

Use Markdown checkboxes for visible progress:

- `[ ]` not started
- `[x]` done

Keep checklist items concrete and implementation-oriented, for example:

- `[x]` define redirect handler contract
- `[x]` add service method for link resolution
- `[ ]` add repository persistence for short links
- `[ ]` add `go test ./...` coverage for happy path and not-found case

Use `Progress Log` style notes only inside `Implementation Notes` or `Review Notes` when a short
explanation is needed. The main progress signal in task files should be the checklist.

Trim stale placeholders before moving a file into this stage.

## Review Findings Format

Use a separate section for review findings so the card itself shows what still blocks completion.

Recommended priorities:

- `P0`
  Critical; must be fixed before merge or before the feature can be considered usable.
- `P1`
  High priority correctness, safety, or contract issue.
- `P2`
  Important quality, maintainability, or test gap that should be fixed in the current task unless
  explicitly deferred.
- `P3`
  Low priority follow-up, cleanup, or non-blocking improvement.

Use Markdown checkboxes here too:

- `[ ]` open finding
- `[x]` resolved finding

Example:

### P1 High

- [x] fix redirect handler to return 404 instead of 500 on unknown code
- [ ] add repository test for duplicate shortcode collision path

When there are no findings at a priority, use a short checked line such as:

- `[x]` no open P1 findings

## Why This Is Lighter Than `docs/templates`

The `skibookers` templates are meant for a larger monorepo with product specs, execution plans,
service maps, and ADRs. This project is smaller.

For this repo, the right level is:
- one lightweight board
- one reusable task template
- explicit stage expectations

If the project grows, this directory can later add more templates such as:
- feature-spec
- API-contract
- migration-plan

Do not add those until the current task template stops being enough.
