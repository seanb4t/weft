<!-- markdownlint-disable MD013 -->
<!-- adr-render: source=bd:weft-b19; do not edit manually; use `/adr update weft-b19` -->

# Use epic bead design field as discuss-to-planner contract medium

**Date:** 2026-06-10
**Status:** Accepted
**Decision:** weft-b19
**Deciders:** Sean Brandt

## Context

The discuss skill (weft-ccy.1) locks per-phase implementation decisions (library choices, API shape, data layout) that the phase-C planner must consume when structuring picks. GSD Core writes these to a .planning/CONTEXT.md file; weft eliminates .planning/ files (beads is the brain, per ADR weft-cfp rationale). A durable, queryable, epic-scoped medium for cross-skill structured state is needed.

## Decision

Locked decisions from discuss are persisted to the epic bead design field via `bd update <epic-id> --design`, structured as markdown headings mirroring GSD CONTEXT.md sections: `## Domain`, `## Decisions`, `## Canonical refs`, `## Specifics`, `## Deferred`. The phase-C planner reads them via `bd show <epic-id>`. Bead notes are used in parallel for audit/resume only. Existing design-field content is merged, never silently overwritten.

## Rationale

- The design field is the only medium colocated with the epic that survives Dolt sync, is addressable by id, and is structured enough for a planner to read programmatically.
- Eliminates file-path coupling between discuss and the planner — both address the epic by id, not by path.
- Merge semantics are explicit: read existing content and append new sections, preserving prior locked decisions across discuss invocations.
- Consistent with the "beads is the brain" invariant (CLAUDE.md, ADR weft-cfp) — no .planning/ or CONTEXT.md artifacts.

## Alternatives Considered

- **.planning/CONTEXT.md file (direct GSD port):** human-editable and toolable, but contradicts the no-.planning substrate contract, is not synced with bead state, and requires per-epic file-path management. Rejected.
- **Bead notes only (`bd note`):** already used for audit, but notes are an append-only log, not a structured queryable document — a planner cannot reliably parse N notes into a coherent decisions map, and there are no merge semantics. Rejected.

## Consequences

- **Positive:** the phase-C planner has a single, stable, epic-scoped location for HOW decisions; discuss output is durable across machines/sessions via Dolt sync; any skill with `bd show` access can consume the contract.
- **Negative:** no schema enforcement on the design field — the planner parses markdown headings, so heading renames are a silent breaking change; merge logic is prompt-layer only with no engine validation.
- **Neutral:** the heading structure mirrors GSD CONTEXT.md sections; a future schema ADR could formalize it.
