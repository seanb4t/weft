<!-- markdownlint-disable MD013 -->
<!-- adr-render: source=bd:weft-p27; do not edit manually; use `/adr update weft-p27` -->

# Discover phases via the ship-and-reshape test, residual ambiguity to fewer phases

**Date:** 2026-06-11
**Status:** Accepted
**Decision:** weft-p27
**Deciders:** Sean Brandt

## Context

The weft-planner must decide when to emit a phased roadmap versus a single-epic picks plan. Auto-discovered phasing (no flag, per the weft-cfp addendum) requires an objective, bias-described test so contributors understand why a given project does or does not get phased. The failure modes of under-phasing versus over-phasing are asymmetric, which drives a specific tie-breaking rule.

## Decision

A phase boundary exists exactly where the planner identifies a genuine ship-and-reshape inflection point — a place where you would want to ship and get human feedback before committing to the next chunk's HOW. Residual ambiguity resolves to **fewer phases**, because over-phasing is harder to undo than under-phasing. The human approval gate (the roadmap `--dry-run` before emit) is the backstop for rare edge cases, deliberately not the primary mechanism.

## Rationale

- Phases add gates, not scheduling — wave decomposition already handles dependency ordering within a single epic; a phase is warranted only when a mid-project human checkpoint changes the HOW of subsequent work.
- Under-phasing loses a checkpoint, but the work still ships and is verified at the end — cheap and recoverable.
- Over-phasing manufactures PR boundaries and forced gates that are structurally annoying to unwind — harder to undo.
- The human approval gate catches the rare residual case without becoming the primary mechanism; if the planner routinely over-phased and relied on humans to collapse, the gate would turn adversarial and erode trust.
- Coverage of the multi-phase path does not require over-triggering; the phase-F dogfood validation gate proves the path with a deliberately multi-phase stimulus.

## Alternatives Considered

**Bias toward more phases (rejected).** Maximizes mid-project checkpoints and human oversight — but over-phasing creates manufactured PR boundaries and forced gates, and turns the approval gate adversarial if it fires routinely.

**Always single-phase unless the user explicitly requests phases (rejected).** Predictable, no planner judgment required — but contradicts the weft-cfp addendum (auto-discovered, no flag) and loses the value of JIT planning for genuinely multi-phase work.

## Consequences

**Positive:** Phase boundaries are grounded in a single testable question contributors can reason about. The fewer-phases default minimizes structural friction for the common case. The human approval gate retains trust by being a backstop, not a routine blocker.

**Negative:** Planner quality determines correctness; the test cannot be mechanically verified. A pathologically under-phasing planner silently skips mid-project checkpoints.

**Neutral:** The degenerate single-phase case (test fails → epic + picks, today's shape) is behaviorally identical to the current one-shot flow. The phase-F dogfood bead's acceptance criteria must pin a concrete multi-phase stimulus to validate the phased path fires.
