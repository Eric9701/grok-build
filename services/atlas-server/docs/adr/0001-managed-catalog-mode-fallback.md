# Managed Catalog Mode falls back when Effective Model Set is empty

`GET /atlas/v1/models` enters **Managed Catalog Mode** only when the user's **Effective Model Set** (Direct Assignment ∪ Group Assignments) is non-empty. An empty set keeps today's behavior: probe → upstream → builtin. We chose this over “empty means deny” so introducing User Groups expands entitlement without silently locking users who have neither direct nor group models. Default model remains a Direct Assignment concern only; groups never set `is_default`.

## Status

accepted

## Considered Options

- **Empty = deny (return no models)** — rejected for this iteration: breaks existing accounts with no `user_models` rows.
- **Per-user `force_managed` flag** — deferred: extra admin surface; revisit if open fallback becomes a policy problem.
- **Enforce allowlist on `/v1/responses` now** — deferred: listing and assignment first; inference enforcement is a separate change.
