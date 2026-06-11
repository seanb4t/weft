<!-- markdownlint-disable MD013 -->
<!-- adr-render: source=bd:weft-mvh; do not edit manually; use `/adr update weft-mvh` -->

# Emit JIT per-phase picks via the re-plan --epic upsert path

**Date:** 2026-06-11
**Status:** Accepted
**Decision:** weft-mvh
**Deciders:** Sean Brandt

## Context

Phase C (weft-ccy.3) introduces JIT per-phase planning: each phase's picks are authored and emitted after that phase's `discuss` gate, not up-front. The Phase B engine (weft-ccy.5) already supports `plan emit --epic <id>` as a re-plan upsert path that wires parent-child links and applies `blocks` edges post-import. The new `plan-phase` skill drives this path targeting a phase sub-epic, with the phase's planning context (goal, research-digest slice, requirement IDs, HOW decisions) sourced from the phase sub-epic bead rather than any sidecar file.

## Decision

Per-phase picks are emitted just-in-time through the existing `plan emit --epic <phase-id>` upsert path. Each phase sub-epic bead's `description` carries the phase mini-brief (goal, research-digest slice, requirement IDs) authored at roadmap-emit time, supplemented by the `design` field populated by `discuss`. `plan-phase` reconstructs full per-phase context from `bd show <phase-id>` alone.

## Rationale

- No new engine mode required: `plan emit --epic` is already the re-plan upsert path, so plan-phase is purely prompt-layer.
- Beads is the brain: phase context persists on the phase sub-epic bead, survives session boundaries, and is retrieved with `bd show <phase-id>` — no sidecar files.
- An empty or absent `design` field is valid input (discuss may be skipped); plan-phase degrades gracefully to planning from the mini-brief alone.
- The first run on an empty phase is the all-creates path; a revision re-run is a natural upsert — same verb, no mode distinction.
- Inter-phase `blocks` edges gate execution (`bd ready`), not planning, so the JIT rhythm is a prompt-layer convention rather than an engine constraint.

## Alternatives Considered

**Up-front full-project pick planning (rejected).** Simpler orchestration, no JIT rhythm — but defeats the purpose of phasing: HOW decisions for later phases depend on the outcomes of earlier phases. Forces premature design commitment; contradicts the ship-and-reshape test.

**Sidecar CONTEXT.md per phase (rejected).** Explicit, human-readable per-phase context — but contradicts the "beads is the brain" invariant, creates an out-of-band file that drifts from bead state, and requires file management the engine does not own.

## Consequences

**Positive:** Phase context is durable across sessions with no file-management overhead. `plan-phase` is a composable standalone unit the Phase-E driver reuses. The re-plan upsert path is exercised by normal phase planning, not just edge-case revisions.

**Negative:** Roadmap emit must author a sufficiently detailed phase mini-brief at planning time; a thin description degrades plan-phase quality. The JIT convention (plan after discuss, after the prior phase ships) is enforced only by skill sequencing, not engine structure.

**Neutral:** Phase-level context carry is the only new concern at roadmap-emit time; pick-authoring methodology is unchanged. Nothing prevents planning phases out of order; the rhythm is a convention.
