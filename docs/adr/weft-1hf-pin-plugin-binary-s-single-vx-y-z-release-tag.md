<!-- markdownlint-disable MD013 -->
<!-- adr-render: source=bd:weft-1hf; do not edit manually; use `/adr update weft-1hf` -->

# Pin the plugin to the binary's single vX.Y.Z release tag

**Date:** 2026-06-06
**Status:** Accepted
**Decision:** weft-1hf
**Deciders:** Sean Brandt

## Context

ADR weft-1nt established that each release cuts two tags — `vX.Y.Z` (binary, via cocogitto) and `weft--vX.Y.Z` (plugin, via `claude plugin tag`) — and `weft install` pins the marketplace ref to `weft--v<version>`. Seam 8 adopts release-please as the orchestrator and a single release tag, making the second tag namespace and the `claude plugin tag` step redundant.

## Decision

Drop the `weft--vX.Y.Z` plugin-tag namespace and the `claude plugin tag` step. `weft install` pins the marketplace source to `@vX.Y.Z` (the binary's own release tag). `plugin.json.version` is bumped by release-please's GenericJson updater in the release PR. **This supersedes the *mechanism* of ADR weft-1nt while preserving its goal:** installed prompts are pinned to the exact commit the running binary was built from (the same tag), so there is no silent prompt↔verb drift within a release.

## Rationale

- A single tag namespace eliminates the two-tag coordination burden weft-1nt's own "negative consequences" acknowledged.
- The plugin tree at `@vX.Y.Z` is the *exact commit* the binary was built from — a strictly stronger co-distribution guarantee than weft-1nt's same-version lockstep.
- `claude plugin validate --strict` (both plugin-path and marketplace-path) in CI replaces the plugin.json↔marketplace agreement check that `claude plugin tag` provided as a side effect.
- Dev/untagged builds still fail `semverPattern` and refuse to float, preserving the actionable-error behavior.
- release-please natively bumps `plugin.json.version`; no separate plugin-version step is needed.

## Alternatives Considered

- **Retain weft-1nt's separate `weft--vX.Y.Z` plugin tag:** `claude plugin tag` validates plugin.json↔marketplace at tag time, but requires two coordinated tags per release and a headless `claude plugin tag` step in CI, and release-please cannot cut that tag natively — added complexity with no benefit once CI validates both plugin gates. Rejected.
- **Float `weft install` to `main` (no pin):** zero coordination, but `main` may carry prompts referencing unreleased verb flags → silent runtime mismatch. Rejected (same reasoning as weft-1nt).

## Consequences

**Positive:** one tag (`vX.Y.Z`) per release; the installed plugin tree is the exact commit the binary was built from (no intra-release drift possible); CI strict-validation replaces tag-time validation. **Negative:** `weft install` tests update pin assertions `weft--v<version>` → `v<version>`; `claude plugin tag` is no longer used or documented in the release path. **Neutral:** `semverPattern` and the dev-refuses-to-float behavior in `weft install` are unchanged; `marketplace.json` carries no per-plugin version — `plugin.json.version` remains the authoritative in-tree literal.

## References

- Supersedes: weft-1nt
