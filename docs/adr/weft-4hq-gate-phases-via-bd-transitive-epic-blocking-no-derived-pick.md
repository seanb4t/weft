<!-- markdownlint-disable MD013 -->
<!-- adr-render: source=bd:weft-4hq; do not edit manually; use `/adr update weft-4hq` -->

# Gate phases via bd transitive epic blocking; no derived pick edges

**Date:** 2026-06-10
**Status:** Accepted
**Decision:** weft-4hq
**Deciders:** Sean Brandt

## Context

The phased emission model (spec docs/superpowers/specs/2026-06-10-phased-emission-engine-enablers-design.md, bead weft-ccy.5) needs phase N+1 work to be unavailable until phase N ships. Readiness gating could be encoded as derived pick-level edges or delegated to bd. A live probe against a scratch bd DB (2026-06-10) settled it: `bd ready` excludes children of a blocks-blocked epic transitively, even when the children carry no edges of their own, and releases them when the blocker closes.

## Decision

Emit only epic-to-epic `blocks` edges between phase sub-epics (from authored `needs` in the roadmap warp-plan); rely on bd transitive epic gating for everything below. No derived pick-level gating edges are emitted or derived. The behavior is pinned by the `TestRoadmapEmitAndTransitiveGating` integration test, which must stay in the suite.

## Rationale

- The live probe confirmed bd already implements exactly the gating the phased model needs; weft-side derivation would duplicate it.
- Derived edges grow with picks-per-phase and would need re-derivation on every JIT re-plan; epic-to-epic edges are N-1 for N phases, period.
- Phase release is atomic: closing the phase epic via `weft finish` releases the entire next phase.
- The auto-gating mechanism was fully designed and then deleted on the empirical evidence — recording this prevents reintroduction.

## Alternatives Considered

- **Derived rootless-pick auto-gating edges** (each rootless pick in a phase blocked-by the prior phase epic): explicit in the graph and independent of bd internals, but O(picks) extra edges per phase, couples emission to scheduling logic bd already owns, and the probe showed it is redundant. Rejected.
- **No graph gating (purely procedural rhythm):** simplest payload, but early-planned picks would be ungated and epic-wide `bd ready` misleading. Rejected.

## Consequences

- **Positive:** minimal graph payloads; no weft-side pick-gating logic; atomic phase release.
- **Negative:** phase gating depends on bd transitive blocking semantics staying stable; the integration test is the tripwire — a bd regression surfaces in weft CI, and the test must never be skipped.
- **Neutral:** the gating mechanism is invisible in the warp-plan file itself; only the phase `needs` edges appear there.
