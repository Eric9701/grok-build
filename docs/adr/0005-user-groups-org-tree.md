# ADR 0005: Merge the org tree into `user_groups` (additive columns)

## Status

Accepted (2026-08-20)

## Context

The atlas-admin redesign needs a four-level org structure: 业务部 → 业务条线 → 条线分组 → 开发人员. The existing `user_groups` table is flat and already backs model entitlement (Effective Model Set) via `group_members` / `group_models`, which atlas-server queries directly with SQL.

Two options were on the table:

1. A separate org-tree table owned by atlas-admin, joined to entitlement data.
2. Merge: make `user_groups` the tree itself by adding nullable `parent_id` and `node_type` columns.

## Decision

Merge. `user_groups` gains two additive, nullable/default columns (`parent_id` self-reference, `node_type` in dept/line/squad). atlas-server's SQL and code stay untouched. Members and model assignments attach only at squad level, with no cross-level inheritance, so existing entitlement JOINs remain correct as-is. Existing rows keep `parent_id = NULL` and surface as "待归类" (unclassified) until an admin places them.

Migration is executed by a manual SQL script (`scripts/migrate.sql` in the atlas-admin repo), not by service-startup auto-migration.

## Consequences

- The "flat User Group" glossary entry is retired; the term now means the squad-level leaf Org Node (`services/atlas-server/CONTEXT.md`).
- Any future consumer of `user_groups` must handle the tree (or filter by `node_type`).
- Rollback is dropping the two columns; entitlement data is unaffected either way.
