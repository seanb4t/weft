<!-- markdownlint-disable MD013 -->
<!-- adr-render: source=bd:weft-0pq; do not edit manually; use `/adr update weft-0pq` -->

# Replan hard-fails on dropping woven work

**Date:** 2026-07-04
**Status:** Accepted
**Decision:** weft-0pq
**Deciders:** Sean Brandt

## Context

`planReplan` computed a `removed[]` list for picks dropped from a re-plan but never enacted it — removed picks vanished from the plan with no trace, the same discoverable-only-by-archaeology failure the unattended-trust milestone exists to end. Seam 2 §8 left the reconciliation policy open, and `bd supersede` requires `--with <new>` (verified), so a successor-less removal cannot use it.

## Decision

An open removed pick is closed with an audit reason naming the replan (`bd close <id> -r "removed by replan of <epic> (was weft-ref:<ref>)"`). A removed pick with any non-`open` status — `in_progress`, `closed`, `blocked`, `hooked`, `deferred`, `pinned`, or any unknown/future status — hard-fails the replan (exit 2, before any mutation): a plan can never silently drop woven or landed work (invariant I2). This fail-closed classification mirrors `reap.go`'s `beadStatus` fail-safe posture: only a status the code positively recognizes as safe to close (`open`) is removable; every other value, known or not, blocks.

## Rationale

- `bd supersede`'s required `--with <new>` does not fit a replan diff, which has no successor mapping — close-with-reason is the honest primitive.
- Silently dropping woven or landed work is the exact invisible-until-archaeology failure this milestone removes.
- Closing (never deleting) preserves history; `bd supersede` can slot in later once an authoring surface can express successor intent, without changing this contract.

## Alternatives Considered

- **Close-with-note for open picks + hard-fail for any non-open pick (chosen):** preserves audit trail without inventing a nonexistent successor; makes silent drops structurally impossible.
- **Silent report only — status quo (rejected):** a plan can orphan in-progress or landed work with no signal.
- **`bd supersede` as the removal mechanism (rejected):** requires `--with <new>`; a replan diff cannot identify a successor ref.

## Consequences

- Positive: replan can never structurally lose tracked work; dry-run reports the same classification (`removed`, `removed_blocked`) without mutating.
- Negative: intentionally dropping in-progress work requires stopping/closing it by hand first, then replanning.
- Neutral: closes the open-pick half of the seam 2 §8 sub-seam; successor-aware supersede stays future work.
