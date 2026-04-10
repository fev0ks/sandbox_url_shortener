# E16 - Owned Links And Personal Analytics

Status: active

## Summary

Product epic for turning links and analytics into user-owned resources instead of anonymous global
records.

## Goal

Make every new link belong to an authenticated user and expose only owner-scoped link management
and statistics.

## Scope

- `owner_user_id` on links
- authenticated create, list, get, update, and delete flows scoped to owner
- owner-scoped analytics reads
- personal "My links" and stats views in `services/web`
- excludes shared ownership, teams, and public profile pages

## Key Decisions

- new links can be created only by authenticated users
- users can view only their own links and analytics
- existing UI updates ship inside this epic, not in a separate UI epic
- analytics remains owned by `services/analytics`; ownership filtering is enforced through service
  contracts

## Task Links

- `E16-001-add-link-ownership-model-and-migrations.md`
- `E16-002-enforce-owner-scoped-shortener-api-access.md`
- `E16-003-add-owner-scoped-analytics-reads.md`
- `E16-004-build-my-links-and-personal-stats-ui.md`

## Open Questions

- what migration strategy to use for existing anonymous links
- whether owner transfer is explicitly unsupported in v1
