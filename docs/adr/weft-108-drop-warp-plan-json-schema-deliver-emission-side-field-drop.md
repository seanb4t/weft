<!-- markdownlint-disable MD013 -->
<!-- adr-render: source=bd:weft-108; do not edit manually; use `/adr update weft-108` -->

# Drop warp-plan JSON Schema; deliver emission-side field-drop guard

**Date:** 2026-06-07
**Status:** Accepted
**Decision:** weft-108
**Deciders:** Sean Brandt

## Context

Seam-2 §8 deferred two coupled items: a formal `warp-plan.json` JSON Schema and the `bd create --graph` input mapping. Seam 9 had to decide whether to build the schema. On inspection, `warp-plan.json` is weft-native (it replaces GSD's `ROADMAP.md`/`PLAN.md` markdown) and its sole author is the `weft-planner` LLM agent — not a human in an editor — so there is no `$schema`/IDE consumer. `internal/plan.Validate()` already performs structural *and* relational validation (ref uniqueness, charset, priority range, `needs` resolves, self-`needs`, reserved `@epic`) that a JSON Schema cannot fully express. Meanwhile the emission-side risk — `bd create --graph` silently dropping unknown fields — is a real, observed correctness gap (confirmed live against bd 1.0.5).

## Decision

Rescope seam 9: drop the warp-plan JSON Schema and deliver only the emission-side bd field-drop guard. The schema stays deferred under `weft-hjx` until a human or third-party plan-authoring surface exists.

## Rationale

- `warp-plan.json` is authored exclusively by the `weft-planner` agent, which already has a validation feedback loop via `weft plan check`; there is no human/IDE `$schema` consumer to serve.
- A JSON Schema would re-encode only the structural subset of `Validate()` and cannot express its relational checks, creating a second source of truth and drift risk for no gain.
- The silent field drop is a confirmed, grounded data-integrity risk; fixing it delivers immediate value.
- Deferring the schema until a real consumer exists avoids premature dual-source-of-truth coupling.

## Alternatives Considered

- **Deliver both the JSON Schema and the emission guard (original seam-2 §8 scope).** Completes the full plan and aids hypothetical future authors, but re-encodes part of `Validate()` (drift risk), serves no current consumer, and delays the grounded correctness fix. Rejected.
- **Generate the schema from the `WarpPlan` Go struct (invopop) for zero drift.** Eliminates drift but still serves no consumer and adds a codegen dependency for a deferred-value artifact. Rejected for now (revisit if a human-authoring surface appears).

## Consequences

- Positive: closes the real silent-warp-data-loss gap without adding schema drift; seam scope stays focused and deliverable.
- Negative: the warp-plan JSON Schema remains deferred; a future human-authoring surface will need a separate seam.
- Neutral: the deferral is tracked under `weft-hjx`, not lost.
