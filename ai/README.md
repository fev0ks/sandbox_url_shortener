# AI Workspace

This directory contains the human-facing AI workflow artifacts for the repository.

Current contents:

- `ai/plan/`
  Task board, epics, templates, and execution-state tracking.
- `ai/skills.md`
  Project-local skill memory and role usage guidance.

## Why `.codex` Stays At Repo Root

The repo-local Codex agent configuration stays in `.codex/` at the repository root because that
is the technical discovery path used by Codex for project agents and config.

Use this split:

- `.codex/`
  Tool-facing config and agent prompts.
- `ai/`
  Human-facing AI workflow docs, plan board, and reusable guidance.
