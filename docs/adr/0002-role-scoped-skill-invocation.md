# Role-Scoped Skill Invocation under Light Coupling

We extend **Light Coupling** so atlas-sdd **Role Executors** may **invoke** mapped skills from `services/skills` (Skill tool or `/` command) during a stage, without absorbing skill bodies into agent `.md` files. ask-atlas remains the sole Main Flow router; `sdd-workflow` only orchestrates Roles. Formal handoff stays the **Documents Contract**. Under **enterprise-strict**, the 1→2→3→4→6 Role chain is mandatory after grill, but Roles may still invoke mapped skills (e.g. `/tdd` in Role 4); closing `/code-review` stays on Main Flow and stays complementary to the QA Role.

## Status

accepted

## Considered Options

- **Skill Absorption** (paste skill steps into agents) — rejected: couples upgrade cycles; already rejected in ADR 0001.
- **Prescription-only** (agents name skills; only parent invokes) — deferred: weaker than Invocation for Role autonomy; may revisit if Task tool cannot load skills.
- **sdd-workflow as strict-mode trunk** — rejected: Dual trunk.
- **Ban skill invocation under enterprise-strict** — rejected: confuses “who orchestrates stages” with “may use TDD discipline while implementing”.

## Consequences

- CONTEXT gains **Role-Scoped Skill Invocation**; Light Coupling / Compliance Mode / Stage Bridge wording updated.
- Need an explicit role→skill map (router + optional agent frontmatter pointers, not copied bodies).
- **Grill stays on the parent session** (`grill-with-docs`); Role 1 formalizes after Bridge Sync and must not run a second interview.
- ADR 0001 remains the packaging/bridge baseline; this ADR only adds Invocation.
- Enterprise discoverability of the skill pack is ADR 0003 (`atlas-skills` marketplace plugin + installer best-effort).
