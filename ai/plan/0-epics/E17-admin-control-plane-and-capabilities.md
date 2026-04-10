# E17 - Admin Control Plane And Capabilities

Status: active

## Summary

Administration epic for introducing explicit admin capabilities instead of a single all-powerful
admin role.

## Goal

Build a bootstrap admin model where access is capability-based and the admin area can grow safely
without assuming every admin can do everything.

## Scope

- admin capability catalog
- identity linkage for admin access
- backend authorization checks for admin-only operations
- `/admin` area inside `services/web`
- minimal mockable and admin-manageable sections for the current product surface
- excludes full enterprise IAM and audit or compliance systems

## Key Decisions

- use capability-based RBAC, not just coarse roles
- keep admin UI in the existing `services/web` runtime under `/admin`
- bootstrap with a small explicit capability catalog matching current functionality
- non-admin users must never reach admin API or UI paths

## Task Links

- `E17-001-define-admin-capability-model-and-storage.md`
- `E17-002-add-admin-authorization-to-backend-surfaces.md`
- `E17-003-build-admin-ui-shell-and-capability-guards.md`
- `E17-004-add-bootstrap-admin-management-flows.md`

## Open Questions

- whether admin bootstrap happens through seed config or manual promotion flow
- whether every admin action needs audit logging in v1
