<!-- markdownlint-disable MD013 -->
<!-- adr-render: source=bd:weft-0pf; do not edit manually; use `/adr update weft-0pf` -->

# Apply re-plan deferred edges post-import via bd dep add; rename envelope key to applied_edges

**Date:** 2026-06-10
**Status:** Accepted
**Decision:** weft-0pf
**Deciders:** Sean Brandt

## Context

On the re-plan path (`plan emit --epic`), edges touching a newly created pick cannot be wired inside `bd import`: bd import has no intra-batch forward references — `depends_on_id` must be an existing id (empirically confirmed against the live bd, 2026-06-10). The pre-existing seam-2 §8 gap surfaced these DeferredEdges as data without applying them, leaving JIT-planned phases with unordered picks.

## Decision

After `bd import` succeeds and the post-import readback rebuilds the ref-to-id map, resolve each deferred edge and apply it via `bd dep add <from-id> <to-id> --type blocks`. Any unresolvable endpoint or failed dep add exits 2 (the warp is structurally incomplete — loud, never silent). The envelope key renames `deferred_edges` to `applied_edges`: the semantics change IS the feature, and a name that says deferred would misstate the contract.

## Rationale

- The import-time alternative is not viable: the no-forward-reference constraint is a hard bd contract, not a preference.
- The post-import readback map already exists (`warpReadback`); edge application is a natural extension of the existing flow.
- Hard-failing on any unwired edge matches the established `VerifyReplan` posture: complete or loudly broken.
- No machine consumer reads `deferred_edges` today (it was surfaced for humans), so the rename window is free now and closed later.

## Alternatives Considered

- **Wire edges inside the bd import payload via depends_on_id:** single atomic operation, but impossible for new picks — ids do not exist until import runs. Rejected on the confirmed bd contract.
- **Keep surfacing without applying (status quo):** leaves JIT-phase picks unordered in bd ready; the phased model cannot function. Rejected.
- **Keep the deferred_edges key with changed semantics:** avoids any consumer churn but permanently misnames the contract. Rejected.

## Consequences

- **Positive:** new-pick edges are live immediately after re-plan; bd ready orders JIT-planned picks correctly; the envelope is honest.
- **Negative:** each edge is a separate CLI invocation; a failure partway leaves some edges wired (mitigated: hard fail names the unwired edge for investigation).
- **Neutral:** removed-pick supersede remains the open §8 sub-seam.

## Addendum (2026-06-10, PR #50)

The post-import wiring mechanism also covers parent-child links: bd import ignores the JSONL parent field entirely (verified), so planReplan wires bd dep add --type parent-child for created picks from the positional import ids before the scoped readback. Same hard-fail posture.
