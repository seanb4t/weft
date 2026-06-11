<!-- markdownlint-disable MD013 -->
<!-- adr-render: source=bd:weft-yup; do not edit manually; use `/adr update weft-yup` -->

# feature skill: adaptive gates, epic-first composition, three-door routing

**Date:** 2026-06-11
**Status:** Accepted
**Decision:** weft-yup
**Deciders:** Sean Brandt

## Context

weft needed a lightweight front door for incremental work against an existing codebase. `new-project` correctly refused such requests but had nowhere to route them (the 2026-06 dogfood dead-end); its full flow is ~28 min / ~125k tokens — wrong for a small feature. GSD's analogue is `/gsd-quick`, which gates discuss/recon/validate behind explicit opt-in flags (`--discuss`, `--research`, `--validate`). weft's substrate — beads as the brain, the `discuss` skill persisting HOW to the epic `design` field, and the `plan emit --epic` re-plan path — makes a different composition possible, and the design must also decide what happens when the repo is not yet weft-managed.

## Decision

The `feature` skill mints the epic first (`bd create --type epic`), then **adaptively** (by judgment, not flags) invokes the real `discuss` skill (locked HOW → `epic.design`) and at most one `Explore` recon pass, emitting picks via `plan emit --epic`. `plan check` + `--dry-run` are always mandatory (no `--validate` opt-out). An unmanaged repo (no `.beads/` or empty warp) is routed to `onboard`; incremental-work requests arriving at `new-project` are routed to `feature` — forming a closed three-door system (new-project → feature → onboard) with no dead-ends. Purely prompt-layer; `feature` is a thin composition of already-shipped units.

## Rationale

- Epic-first composition lets the real `discuss` skill persist HOW to `epic.design`, maintaining the weft-b19 contract (design field = discuss→planner contract) rather than baking HOW only into pick descriptions.
- Adaptive gates (not flags) are consistent with weft-cfp's addendum (phasing is auto-discovered, not flagged); applying the same principle to discuss/recon preserves a single coherent control model across all weft skills.
- `plan check` + `--dry-run` are non-optional because weft's emit path is destructive; removing the validate opt-out closes a footgun GSD's `--validate` flag introduces.
- Three-door routing converts dead-ends into a navigable skill graph without either skill implementing the other's logic.
- Purely prompt-layer: no engine change, no new Go verbs.

## Alternatives Considered

**Inline discuss-style, single atomic emit (rejected).** Simpler linear flow, no real-discuss latency on obvious features — but HOW is baked into picks with no separate locked record, diverging from GSD's CONTEXT.md model and from weft-b19's design-field contract; it reuses discuss's *style*, not its mechanism.

**Flag-gated rigor — port GSD's `--discuss`/`--research`/`--validate` (rejected).** Explicit user control, directly mirrors `/gsd-quick` — but contradicts weft's established adaptive-only model (weft-cfp addendum), burdens users with mode selection, and is inconsistent with the `discuss` skill's flagless v1 design.

**Route unmanaged repos inline — feature bootstraps if needed (rejected).** Single entry point — but makes `feature` responsible for onboarding logic, breaking single-purpose discipline and inflating scope. Routing to `onboard` keeps each door lean.

## Consequences

**Positive:** HOW decisions are durably queryable via `bd show <epic>` (design field), not lost in pick descriptions. Consistent adaptive control model across all weft skills — no mode-flag proliferation. Three-door routing eliminates dead-ends; each skill stays single-purpose. The mandatory validation gate prevents silent bad emits on the re-plan path.

**Negative:** Obvious features still incur a precondition check + brief Q&A before picks are emitted (no true zero-overhead fast path). Adaptive judgment is opaque until the skill states its decision aloud; wrong calls require user override rather than a flag.

**Neutral:** GSD's ecosystem-research flag (`--research`) has no weft equivalent and is explicitly out of scope. The `onboard` skill (ccy.7) does not yet exist; routing to it is forward-declared.
