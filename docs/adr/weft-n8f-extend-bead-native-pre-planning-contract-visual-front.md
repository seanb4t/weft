<!-- markdownlint-disable MD013 -->
<!-- adr-render: source=bd:weft-n8f; do not edit manually; use `/adr update weft-n8f` -->

# Extend bead-native pre-planning contract to the visual front

**Date:** 2026-06-20
**Status:** Accepted
**Decision:** weft-n8f
**Deciders:** Sean Brandt

## Context

ADR `weft-fwn` established that pre-planning outputs (`explore`, `spike`) are bead-native — no `.planning/` files. The `sketch` skill introduces a visual-exploration step whose native GSD output is committed HTML files under `.planning/sketches/` plus a project-local skill file, and whose durable design decisions must still flow to `ui-phase` and `plan-phase` via beads. This decision resolves how the visual layer of the pre-planning funnel fits the existing no-`.planning/` contract.

## Decision

Sketch mockups are written to the gitignored `.weft/sketch/` scratch directory (or a `/tmp` fallback) and are intentionally ephemeral. The chosen visual direction is persisted as a `bd remember` entry (key `sketch-<slug>`) and, when epic-scoped, folded into the phase epic `design` field for same-session `ui-phase`/`plan-phase` consumption. The UI contract produced by `ui-phase` is persisted as a `decision` bead (label `ui-spec`) plus the epic `design` field. No `.planning/` files; no committed mockups.

## Rationale

- Completes ADR `weft-fwn`'s no-`.planning/` contract for the visual front of the funnel — `sketch` and `ui-phase` are the only remaining pre-planning skills GSD backed with `.planning/` files.
- gitignored paths are never snapshotted by jj in a colocated repo, so the working copy stays clean with no jj choreography — a property not available in a plain-git repo.
- The epic `design`-field relay (ADR `weft-b19`) gives same-session availability that `bd remember` alone cannot: sketch → ui-phase → plan-phase within one session.
- A `decision` bead (`ui-spec` label) makes the UI contract a first-class, query-able warp citizen rather than a transient text block, consistent with beads-is-the-brain.

## Alternatives Considered

- **Ephemeral mockups to gitignored `.weft/`; durable output to `bd remember` + epic `design` field (chosen):** completes the no-`.planning/` contract; mockups are truly throwaway; chosen direction is bead-native and surfaced by `bd prime`; no new bead type or engine surface. Cost: mockup artefacts are not recoverable after the session.
- **Keep mockups under `.planning/sketches/` (direct GSD port) (rejected):** zero-friction and browsable, but contradicts the Layer B no-`.planning/` replacement contract (`weft-fwn`, `weft-b19`); files accumulate without a lifecycle and diverge from bead state.
- **Throwaway jj change (spike pattern) for sketch artefacts (rejected):** consistent with spike mechanics, but HTML mockups in a colocated repo appear as untracked and need explicit `jj abandon` choreography per session; the chosen direction still needs a separate bead-native persistence path. Gitignored `.weft/` removes the choreography entirely.

## Consequences

- Positive: the full pre-planning funnel (explore → spike → sketch → ui-phase) is bead-native and query-able with no `.planning/` surface left; mockups cannot be accidentally committed (gitignored, not jj-tracked); `ui-spec` decision beads are discoverable via `bd list --label ui-spec` and surfaced by `bd prime`.
- Negative: sketch mockup history is not recoverable after the session — only the textual chosen-direction summary in `bd remember` survives; same-session availability depends on the design-field relay (a session boundary between sketch and ui-phase requires reading `bd memories` instead).
- Neutral: no new bead type or engine surface — `decision` beads with a `ui-spec` label reuse the existing type.
