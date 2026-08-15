# Skills + SDD light coupling under ask-atlas Main Flow

We fuse `services/skills` (ask-atlas Main Flow) with `atlas-sdd` (Role Executors + Enterprise Spec Baseline) by **Light Coupling**: keep skill and agent bodies separate; cross-link routers; hand off only through the **Documents Contract** (`documents/…`). ask-atlas remains the sole orchestrator; SDD never becomes a second trunk. Enterprise standards live in plugin `spec/` with per-repo **Project Override** and optional **Compliance Mode** (`standard` vs `enterprise-strict` in business-repo config). Ops/data roles are **Side Ramps**. Closing review is **Enterprise-Aware Review** (Standards include enterprise `spec/`).

## Status

accepted

## Considered Options

- **SDD as primary router** — rejected: loses grilling / wayfinder / tdd discipline.
- **Single mega-plugin or absorbing skills into agents** — rejected: couples upgrade cycles and blunts both packs.
- **Dual equal entry points** — rejected: no single enterprise process.
- **Documents + tickets both formal** — rejected: unauditable “which artifact won?”

## Consequences

- Router edits (`ask-atlas`, `sdd-workflow`) and a bridge checklist are the implementation surface — not a merge of the two trees.
- QA Role Executor and `/code-review` stay complementary (cases vs diff Standards/Spec).
- Align ops readers with `documents/test-cases/` when wiring Side Ramps (fix path drift vs `test-case-generator`).
