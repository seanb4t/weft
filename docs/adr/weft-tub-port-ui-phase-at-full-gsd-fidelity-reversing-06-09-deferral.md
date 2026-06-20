<!-- markdownlint-disable MD013 -->
<!-- adr-render: source=bd:weft-tub; do not edit manually; use `/adr update weft-tub` -->

# Port ui-phase at full GSD fidelity, reversing the 06-09 deferral

**Date:** 2026-06-20
**Status:** Accepted
**Decision:** weft-tub
**Deciders:** Sean Brandt

## Context

The 2026-06-09 epic spec (`weft-ccy`) explicitly deferred `ui-phase`: "`/gsd-ui-phase` … deliberately not ported in this round." ADR `weft-cfp` records the v1 interaction-model deferral and the additive-restoration roadmap. `weft-ccy.10` is the last open child of the epic. This decision settles whether to port `ui-phase` now at full fidelity or continue the deferral.

## Decision

`ui-phase` is ported now at full GSD fidelity — a two-agent researcher/checker loop (`weft-ui-researcher` + `weft-ui-checker`, ≤2 iterations, five research areas, six validation dimensions) — producing a `decision` bead (`ui-spec`) plus the epic `design` field rather than a `{phase}-UI-SPEC.md` file. This reverses the 06-09 "deliberately not ported in this round" deferral because that round is now complete. `sketch` is ported together with it so the visual door has a native consumer.

## Rationale

- The 06-09 deferral was explicitly "in this round"; that round (`weft-ccy`) is complete, with `weft-ccy.10` as the last child — deferring again would leave the epic permanently open.
- `sketch`'s natural consumer is `ui-phase`; without it the sketch → plan-phase handoff omits the UI-contract locking step that `plan-phase` needs, which is exactly why the two were bundled into this bead.
- The substrate-fit question resolves identically to `explore`/`spike` (ADR `weft-fwn`) and `discuss` (ADR `weft-b19`) — no new substrate patterns are required.
- Additive restoration per ADR `weft-cfp`'s roadmap: `ui-phase` is a thin orchestrator over existing verbs + beads, with no engine change.

## Alternatives Considered

- **Port `ui-phase` now at full GSD fidelity, bundled with `sketch` (chosen):** co-designed skills; substrate-fit and consumer questions both resolve cleanly; completes the epic. Cost: two new agents + a new skill, and a change to the shipped `plan-phase`.
- **Continue deferral — port `sketch` only (rejected):** smaller scope, but reopens the unresolved sketch-consumer question (the very reason sketch was split out with the ui-phase question) and leaves the `weft-ccy` epic permanently open.

## Consequences

- Positive: the `weft-ccy` epic is complete — the full GSD Layer-A pre-planning loop is ported; `plan-phase` gains a UI-contract input path so future frontend phases can lock design tokens before planning.
- Negative: `plan-phase` (a shipped skill) is modified (UI safety gate + planner-context extension).
- Neutral: ADR `weft-cfp` receives an implicit addendum — the phased/interactive surface is now fully restored for the pre-planning front of the funnel.
