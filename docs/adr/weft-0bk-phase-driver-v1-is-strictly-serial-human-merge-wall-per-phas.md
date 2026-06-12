<!-- markdownlint-disable MD013 -->
<!-- adr-render: source=bd:weft-0bk; do not edit manually; use `/adr update weft-0bk` -->

# phase-driver v1 is strictly serial: human merge-wall per phase, no auto-merge or stacking

**Date:** 2026-06-12
**Status:** Accepted
**Decision:** weft-0bk
**Deciders:** Sean Brandt

## Context

The phase-driver skill walks a multi-phase project epic through the per-phase rhythm (discuss -> plan-phase -> execute -> verify-work -> finish). A key v1 constraint is that phases are strictly serial: the driver pauses at each PR merge wall for a human merge, never merges PRs unattended, and never starts phase N+1 before phase N PR lands in trunk. Auto-merge and phase stacking are explicitly deferred. The --auto flag skips the question gates (discuss, verify-work) but still walls at every merge. This serialization is load-bearing: the driver re-entry logic (check `gh pr view <phase> --json state`) is designed around the single-phase-open-at-a-time invariant, and any later automation layer must opt in to stacking or auto-merge explicitly.

## Decision

v1 phase-driver is strictly serial: at most one phase PR open at a time; the driver pauses at every merge wall for a human merge; --auto skips only the two question gates. Stacked-phase execution and unattended auto-merge are explicitly out of scope and must be introduced as named future overlays (an opt-in flag or separate skill), not retrofitted into the serial driver contract.

## Rationale

- The merge-wall invariant makes re-entry deterministic: `gh pr view <phase> --json state` uniquely identifies the suspended step with no cursor ambiguity (at most one open phase PR).
- weft-cfp and its phasing addendum establish that each phase lands in trunk before the next starts; the serial constraint is a direct consequence of that phasing model.
- Auto-merge bypasses the human review gate and needs trusted CI + merge permission with high blast radius if a phase ships broken — deliberately deferred, not overlooked.
- Stacking phases would require changes to `weft finish reconcile` and the driver re-entry logic; keeping v1 serial bounds both, preserving pure skill composition with no new engine semantics.

## Alternatives Considered

**Serial execution with human merge wall (chosen)** — simplest safe invariant; unambiguous re-entry (one open phase PR at a time); human retains merge authority at every trunk boundary; keeps stacked-PR conflict resolution out of v1. Cost: slower throughput on long CI/review cycles; unattended runs pause at every boundary.

**Stacked phases (N+1 starts before N merges)** — higher parallelism, shorter wall-clock. Rejected: driver must manage stacked-PR rebases after upstream merges; re-entry cursor becomes ambiguous with multiple open phase PRs; much more complex reconcile.

**Auto-merge (driver merges unattended)** — fully unattended end-to-end. Rejected for v1: bypasses human code review, requires trusted CI + merge permission, high blast radius. Deferred as a future overlay.

## Consequences

**Positive:** Re-entry is unambiguous (<=1 open phase PR at any time); human retains merge authority at every trunk-integration boundary; driver stays pure skill composition with no new engine semantics.

**Negative:** Multi-phase projects with long review cycles must pause the driver at each phase boundary; --auto cannot fully automate an overnight multi-phase run without human merge intervention.

**Neutral:** A future auto-merge overlay or stacking mode can be added as an explicit opt-in without altering the serial driver contract.
