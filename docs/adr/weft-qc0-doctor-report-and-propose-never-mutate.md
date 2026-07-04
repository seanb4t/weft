---
title: "Doctor: report and propose, never mutate"
---
<!-- markdownlint-disable MD013 -->
<!-- adr-render: source=bd:weft-qc0; do not edit manually; use `/adr update weft-qc0` -->

**Date:** 2026-07-04
**Status:** Accepted
**Decision:** weft-qc0
**Deciders:** Sean Brandt

## Context

Whole-warp health had no single verb — per-epic `resume` exists, but strays/orphans/divergence were discoverable only by manually joining bead state against jj workspace state. Adding `weft doctor` raises whether the diagnostic verb should also heal what it finds.

## Decision

`weft doctor` is permanently read-only: it exits 0 even with findings (diagnosis is not failure), and every finding carries a machine-readable category/reason plus the existing verb that recovers it (`weft reap`, `weft conflict open`, `weft finish reconcile`, `bd close`). Doctor never mutates.

## Rationale

- Avoids duplicating reap/conflict/finish/bd's mutation logic inside a diagnostic verb — every mutation keeps exactly one owner.
- Exit 0 on findings lets automation (SessionStart hook, CI, skills) branch on structured envelope data without conflating findings with verb failure.
- Auto-fix is explicitly deferred, not designed away — revisit only if report+propose proves too thin in dogfood (mirrors seam 7 §11's posture on `install`).

## Alternatives Considered

- **Report + propose (chosen):** read-only join; findings carry suggested recovery commands; single owner per mutation concern.
- **Report only (rejected):** consumers would re-derive the recovery mapping doctor already knows.
- **Propose + enact via `--fix` (rejected for v1):** one command heals the warp, but mixes diagnosis with mutation and risks silently recovering state a human should see first.

## Consequences

- Positive: safe to run anywhere, anytime — hooks, CI, mid-wave — with zero mutation risk.
- Negative: operators issue a second command to heal each finding.
- Neutral: `--fix` remains a scoped future extension, not a rewrite.
