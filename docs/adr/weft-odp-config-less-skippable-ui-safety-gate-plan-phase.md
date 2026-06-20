<!-- markdownlint-disable MD013 -->
<!-- adr-render: source=bd:weft-odp; do not edit manually; use `/adr update weft-odp` -->

# Config-less skippable UI safety gate in plan-phase

**Date:** 2026-06-20
**Status:** Accepted
**Decision:** weft-odp
**Deciders:** Sean Brandt

## Context

GSD's `/gsd-ui-phase` uses `workflow.ui_phase` and `workflow.ui_safety_gate` config toggles to control whether `ui-phase` is mandatory and whether `plan-phase` emits a gate warning for frontend phases. weft has no `workflow.*` configuration system. This decision settles how `plan-phase` communicates the recommended `ui-phase` step without blocking and without introducing a config surface.

## Decision

`plan-phase` Phase 1.5 evaluates frontend signals (design-system presence, mini-brief/pick keywords) and, when no `ui-spec` bead or `design`-field contract exists, emits a non-blocking prompt asking the user to run `ui-phase` first or skip. There is no config toggle; the gate is always-on but always skippable. GSD's `workflow.ui_phase` and `workflow.ui_safety_gate` toggles are dropped from weft's port scope.

## Rationale

- weft has no `workflow.*` configuration system; introducing one for a single gate would be the only config surface in the prompt layer — disproportionate to the need.
- The opt-in-by-invocation model (`sketch`, `ui-phase` are explicit doors) is the existing pattern for pre-planning skills; the gate is the discovery mechanism, not a hard dependency.
- A non-blocking nudge preserves the "soft handoffs" invariant: no skill hard-chains to another.
- Dropping GSD's toggles is consistent with weft's substrate-translation philosophy: replace GSD mechanisms with simpler weft-native equivalents where the substrate already handles the concern.

## Alternatives Considered

- **Always-on, skippable nudge with no config toggle (chosen):** no configuration system needed; the gate is always evaluated so it cannot be accidentally disabled; the user keeps control by replying "skip." Cost: cannot be permanently silenced for projects that never want `ui-phase`; no machine-readable suppression for automation.
- **Introduce a `workflow.*` config system mirroring GSD (rejected):** per-project control and exact GSD parity, but spec §8 marks a weft `workflow.*` system out of scope; it would be the only config surface in the prompt layer.
- **Make `ui-phase` mandatory (hard gate) for frontend phases (rejected):** guarantees a UI contract exists, but blocks planning for users who want to skip and contradicts weft's opt-in-by-invocation model.

## Consequences

- Positive: no configuration surface is introduced — `plan-phase` stays purely prompt-layer; users who always want to skip are one word from doing so; the gate stays silent on non-frontend phases and on phases that already have a UI contract.
- Negative: projects that never use `ui-phase` cannot permanently suppress the nudge without skipping each time; the frontend-signal heuristic (keyword + design-system detection) may produce false positives on ambiguous phases.
- Neutral: GSD's `workflow.ui_phase` and `workflow.ui_safety_gate` toggles are permanently dropped from weft's port scope.
