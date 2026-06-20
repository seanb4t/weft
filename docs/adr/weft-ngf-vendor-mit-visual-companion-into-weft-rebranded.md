<!-- markdownlint-disable MD013 -->
<!-- adr-render: source=bd:weft-ngf; do not edit manually; use `/adr update weft-ngf` -->

# Vendor the MIT visual companion into weft, rebranded

**Date:** 2026-06-20
**Status:** Accepted
**Decision:** weft-ngf
**Deciders:** Sean Brandt

## Context

The `sketch` and `ui-phase` skills require a browser-based visual companion (a zero-dependency Node.js server with live click-to-select and event capture). `obra/superpowers` ships an identical server under the MIT license. weft does not assume `dev-flow` is installed alongside it, and `dev-flow`'s installed script path carries a per-release version-hash segment that would be fragile to reference. This decision settles how weft obtains and ships the companion.

## Decision

The five companion files (`server.cjs`, `start-server.sh`, `stop-server.sh`, `frame-template.html`, `helper.js`) are vendored into `plugin/skills/sketch/scripts/visual-companion/` and rebranded to weft (scratch dir `.weft/sketch/`, weft chrome). Vendored files retain their MIT headers (not relicensed to Apache-2.0); a top-level `NOTICE` records MIT attribution and an `UPSTREAM.md` file pins the `obra/superpowers` source commit and enumerates weft modifications.

## Rationale

- weft must be self-contained as a Claude Code plugin; runtime discovery of `dev-flow`'s version-hashed path is explicitly rejected as fragile, and it would assume `dev-flow` is installed alongside weft.
- MIT permits modification; the rebrand (`.weft/` scratch dir, chrome) is a permitted modification and the original copyright is preserved.
- A single vendored copy under `plugin/skills/sketch/scripts/` is referenced by both `sketch` and `ui-phase` via `${CLAUDE_PLUGIN_ROOT}`, avoiding duplication within the plugin tree.
- The `NOTICE` + `UPSTREAM.md` pin gives future contributors a clear upgrade path and a precise diff surface for weft modifications.

## Alternatives Considered

- **Vendor into weft plugin tree, rebranded, MIT attribution retained (chosen):** self-contained, no `dev-flow` runtime dependency; one shared copy; provenance via NOTICE + UPSTREAM. Cost: upstream changes require a manual sync; ~26 KB of non-markdown files added to the plugin tree.
- **Reference `dev-flow`'s installed companion path at runtime (rejected):** zero duplication, but the path includes a per-release version-hash segment (brittle across upgrades) and requires `dev-flow` to be installed — an assumption weft rejects.
- **Reimplement a minimal companion from scratch (rejected):** no licensing/upstream-drift concerns, but significant implementation cost for a non-core capability the MIT server already satisfies.

## Consequences

- Positive: no runtime dependency on `dev-flow` — weft is self-contained; provenance is explicit (UPSTREAM records the source commit + modified lines); the MIT exception is isolated to one directory while the rest of weft stays Apache-2.0.
- Negative: upstream `obra/superpowers` changes require a manual sync (no automatic tracking); the vendored non-markdown files (`.cjs`/`.sh`/`.js`/`.html`) must be accepted by `claude plugin validate --strict`.
- Neutral: internal `BRAINSTORM_*` env var names are intentionally not rebranded, to minimize upstream drift.
