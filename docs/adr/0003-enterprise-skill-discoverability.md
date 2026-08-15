# Enterprise skill discoverability via atlas-skills marketplace plugin

Enterprise install must make the ask-atlas engineering skill pack discoverable so
**Role-Scoped Skill Invocation** (ADR 0002) works on machines that never clone
the monorepo. We ship skills as a separate marketplace plugin **`atlas-skills`**,
best-effort installed alongside **`atlas-sdd`**. Discovery priority stays
unchanged: native (cwd / repo / user / `[skills].paths`) wins bare names;
plugin skills remain available as `plugin:name` and as bare names when no native
shadow exists. Installers do not fail the whole install when plugin install
fails; use `GROK_SKIP_ATLAS_SKILLS=1` to skip intentionally.

## Status

accepted

## Considered Options

- **Independent marketplace plugin (`atlas-skills`)** — accepted: matches Light Coupling packaging; discovery uses the existing enabled-plugin `skills/` path.
- **Copy into `~/.atlas/skills` only** — deferred as fallback when marketplace is unavailable; not the primary channel.
- **`[skills].paths` to a share** — deferred as optional enhancement via managed settings.
- **Vendor skills under `atlas-sdd/skills/`** — rejected: merges upgrade cycles (ADR 0001).
- **Hard-fail install if skills missing** — rejected: enterprise network/git often flaky; best-effort + warning.
- **Enterprise whitelist filter on Role skill set** — deferred; affinity is prescription, not a runtime ACL.

## Consequences

- Install scripts (`install*.ps1` / `install*.sh`) best-effort install `atlas-skills` after `atlas-sdd`.
- Marketplace lists both plugins; monorepo sync script copies `services/skills/engineering` into `plugins/atlas-skills/skills` before publish.
- Affinity table documents the enterprise discovery precondition.
- Managed settings push of `[skills].paths` remains a later enhancement.
