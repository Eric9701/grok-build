# Agent Engineering Process (Atlas)

How Atlas routes R&D work through skills and enterprise role agents so delivery follows enterprise standards without dual competing workflows.

## Language

**Main Flow**:
The ask-atlas path from idea to ship: grill-with-docs → (optional to-spec / to-tickets) → implement (tdd + code-review).
_Avoid_: Dual trunk, parallel equal pipelines, SDD-as-primary-router

**Enterprise Spec Baseline**:
The standards packaged in the atlas-sdd plugin under `spec/` (`${GROK_PLUGIN_ROOT}/spec`). The default authority for how roles produce work.
_Avoid_: Treating business-repo docs alone as the only enterprise standard; public x.ai/GCS coding guides as enterprise baseline

**Project Override**:
Project-local standards that supersede the Enterprise Spec Baseline for the same topic — via `.atlas/` / `.grok/` agents, project docs, or `documents/` conventions agreed for that repo.
_Avoid_: Silent drift (local practice with no recorded override)

**Stage Bridge**:
A phase boundary where Main Flow hands work to (or receives from) an atlas-sdd role Task via agreed artifact paths under `documents/`, without merging skill and agent bodies.
_Avoid_: Absorbing skills into agents; copying full specs into Task prompts; forbidding Role-Scoped Skill Invocation (invocation ≠ absorption)

**Role Executor**:
An atlas-sdd Task subagent (`atlas-sdd:<role>`) that executes a stage under Enterprise Spec Baseline (and Project Override). Not the router.
_Avoid_: general-purpose for SDD role work; calling role agents the "main flow"

**Light Coupling**:
Fusion mode: router cross-references + Stage Bridge paths; skill packs and SDD agents stay separate packages/bodies. Roles may **invoke** mapped skills (Role-Scoped Skill Invocation) but must not copy or rewrite skill bodies into agents.
_Avoid_: Single mega-plugin merge; rewriting skills into agent steps; Dual equal entry points

**Role-Scoped Skill Invocation**:
A Role Executor calls the Skill tool or `/` command for a mapped `services/skills` skill during its stage; the skill text stays in the skills package. Complements Stage Bridge (artifacts) — does not replace Documents Contract handoff. **Grill (`grill-with-docs`) stays on the parent session**; Role 1 formalizes after Bridge Sync.
_Avoid_: Skill Absorption (pasting skill steps into agent `.md`); treating skill output as a second formal artifact tree outside `documents/`; Role 1 running a second interview (`ask_user_question`) instead of parent grill

**Compliance Mode**:
Per-repo setting for how strictly Role Executors are required. Default is **standard** (design docs optional if equivalent `documents/detailed-design/` already exists before code; grill/to-spec may stay on Main Flow). **enterprise-strict** forces the SDD role chain after grilling (**1→3→4→6**); **Role 2 (architecture) is optional** in both modes. Roles may still use Role-Scoped Skill Invocation; Main Flow skills outside the chain stay limited to grill and closing code-review. User nouns (需求文档 / 设计文档 / 实现代码) name the **current stage**, not a Role shortcut. 「设计文档」/「详细设计」→ Role 3; 「架构」/「架构设计」→ optional Role 2.
_Avoid_: One global mandate for every repo; silent “sort of enterprise” without a named mode; banning `/tdd` inside Role 4 under strict; forcing Role 2 before Role 3; treating a missing architecture doc as a blocker

**Documents Contract**:
The sole hard handoff tree under the business repo: `documents/requirements-analyst/`, `documents/detailed-design/`, `documents/test-cases/` (and ops/data dirs when those roles run). Architecture under `documents/architecture-design/` is **optional input** to Role 3, not a required contract path. CONTEXT.md, ADRs, and tickets are upstream drafts until bridged here.
_Avoid_: Dual-track formal artifacts; issue tracker as the only SDD handoff; leaving `documents/` optional when a Role Executor ran; requiring `documents/architecture-design/` before detailed design

**Enterprise-Aware Review**:
code-review Standards axis checks Enterprise Spec Baseline (relevant subset) plus Project Override plus a Fowler smell baseline; Spec axis still checks the originating ticket/spec.
_Avoid_: Standards that ignore plugin `spec/`; replacing Standards entirely with the QA Role Executor

**Side Ramp**:
Role Executors that join only on explicit user intent — architecture (`2`) when they ask for 架构/架构设计; ops (`7`) for deploy/ops; data (`8`) for warehouse/ETL. Not part of the default 1→3→4→6 chain.
_Avoid_: Running architecture/ops/data on every feature by default; inserting Role 2 between requirements and detailed design

**Bridge Sync**:
At Main Flow phase boundaries, the session copies or writes agreed content from CONTEXT/tickets/specs into the Documents Contract before spawning a Role Executor. Role Executors read `documents/`; they do not own the bridge. Skill Invocation output is informal until Bridge Sync.
_Avoid_: Agents inventing a second artifact tree; requiring humans to maintain `documents/` with no session checklist; treating skill scratch as formal deliverables

**Enterprise Skill Pack**:
The ask-atlas engineering skills shipped for enterprise discovery as the separate marketplace plugin **`atlas-skills`** (synced from `services/skills/engineering`). Installers best-effort install it beside **`atlas-sdd`**. Native discovery paths and `[skills].paths` still win bare-name conflicts.
_Avoid_: Merging skills into `atlas-sdd`; hard-failing CLI install when marketplace/git is down; treating affinity as a runtime ACL

**Device Auth Login**:
The VS Code extension auth-required onboarding command: `atlas login --device-auth` (terminal `shellArgs`: `login`, `--device-auth`; `shellPath` is the resolved CLI binary, e.g. `atlas.exe` on Windows).
_Avoid_: Bare `grok login` / `atlas login` without `--device-auth` on that onboarding path
