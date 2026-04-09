# E04-001 - Scaffold Next.js Web Service

Status: complete

Epic: E04 - Web Frontend

## Summary

Create a new `services/web` service based on `Next.js + TypeScript` with a minimal but production-
shaped structure: app shell, environment wiring, API base URL config, Dockerfile, and local
compose integration.

## Scope

- add `services/web` as a standalone frontend service
- initialize `Next.js` with TypeScript and an `app/` router structure
- define frontend env contract for talking to shortener API
- add service-owned Dockerfile and local run/build tasks
- wire the service into local docker-compose
- excludes feature pages beyond a placeholder shell
- excludes backend API changes

## Acceptance Criteria

- `services/web` exists as an isolated service with `package.json`, TypeScript config, and
  `Next.js` app entrypoints
- the app starts locally and renders a placeholder home page
- frontend config points to the shortener API through an explicit env var, not hardcoded URLs
- `deploy/docker-compose/docker-compose.yml` can start the frontend alongside shortener
- the repo docs mention the new service and its ownership boundary

## Assumptions

- Node.js toolchain can be introduced into this repo without restructuring the whole monorepo
- `services/web` owns its own package manager files and build tooling
- the initial frontend can stay simple and does not need SSR caching strategy decisions yet

## Implementation Notes

- keep the service boundary parallel to the Go services: `services/web`
- prefer a small, explicit structure over framework-generated noise
- define env names early, for example `NEXT_PUBLIC_API_BASE_URL` or equivalent
- make local dev ergonomic, but do not hide cross-service URLs behind magic
- update architecture docs if the repo-level service map changes

## Progress Checklist

- [x] confirm scope and assumptions
- [x] scaffold `services/web`
- [x] add env contract for backend API base URL
- [x] add Dockerfile and local compose wiring
- [x] update repo docs for the new service
- [x] run validation
- [x] pass review or resolve review findings

## Validation

- `npm install` in `services/web` — completed, lockfile generated
- `npm run typecheck` in `services/web` — passes
- `npm run build` in `services/web` — passes
- `docker compose -f deploy/docker-compose/docker-compose.yml config` — passes
- `task build` — passes
- `docker build -f services/web/Dockerfile .` — passes

## Review Notes

- `services/web` scaffolded with `Next.js + TypeScript`, placeholder landing page, and explicit
  `NEXT_PUBLIC_API_BASE_URL`
- service-owned Dockerfile added and wired into docker-compose as `web`
- root `.dockerignore` added so Docker builds do not send frontend artifacts and git metadata in
  the context

## Review Findings

### P0 Critical

- [x] no open P0 findings

### P1 High

- [x] no open P1 findings

### P2 Medium

- [x] no open P2 findings

### P3 Low

- [x] no open P3 findings
