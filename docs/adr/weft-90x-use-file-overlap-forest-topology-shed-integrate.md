<!-- markdownlint-disable MD013 -->
<!-- adr-render: source=bd:weft-90x; do not edit manually; use `/adr update weft-90x` -->

# Use file-overlap forest topology for shed integrate

**Date:** 2026-06-09
**Status:** Accepted
**Decision:** weft-90x
**Deciders:** Sean Brandt

## Context

`shed integrate` built one lexicographic linear stack across all wave members
(`trunk() <- b0 <- ... <- bN`, `sort.Strings` tiebreaker), manufacturing
ancestry between picks that share no files. A *shed* is by definition a wave of
mutually independent `bd ready` picks, so that ancestry is artificial. When a
conflict is escalated (left unresolved for a human), its change stays conflicted
and jj cascades the conflict to every descendant — blocking individually-clean
picks from landing (they fail `pick land`'s `changeConflicted` gate and cannot
be pushed). Because `bd` assigns the ids that determine lex order, neither the
orchestrator nor a fixture can force the escalate-prone pick to sort last. A
single `integrate` over a mixed heal/escalate wave was therefore infeasible; the
seam-10 E2E used two `integrate` calls as a workaround (finding `weft-78k`).

## Decision

`shed integrate` partitions the wave into connected components over the "share
at least one file" relation (union-find on `jj diff --name-only` output), stacks
linearly only within each component rooted on `trunk()`, and emits `data.groups`
(a forest) replacing the flat `data.stack`. Conflict detection stays up-front via
the existing scoped `conflicts()` revset. The `--skip-emptied` omission from ADR
`weft-hjx.7` is **retained within each group** (this decision changes the stack
*structure*, not that policy; `weft-hjx.7` remains in force per-group).

## Rationale

- jj conflicts are per-file: picks touching disjoint file sets cannot collide,
  so file-overlap grouping is a sound and complete partitioning criterion.
- Cross-group cascade is structurally eliminated — an escalated pick in group A
  has no ancestry relation to group B, so it cannot poison it.
- Determinism is preserved: within-group lex order by change-id + group order by
  lex-smallest member is a total, reproducible order.
- The single-wave E2E (`weft-78k`) becomes deterministic regardless of
  bd-assigned id order, closing the blocker that forced wave-splitting.

## Alternatives Considered

- **Single lexicographic linear stack (status quo).** Simple cursor-advance loop;
  no file-metadata query. Rejected: manufactures ancestry between file-disjoint
  picks; an escalated tail cascades to all lex-higher picks; manual escalate-last
  ordering is impossible when bd assigns ids.
- **Multiple integrate calls with manual escalate-last ordering.** No engine
  change; works for known fixtures. Rejected: non-deterministic under bd-assigned
  ids; requires predicting which picks escalate before they are rebased; does not
  scale to multiple independent conflict pairs.
- **File-overlap forest (chosen).** Provably safe; eliminates false cascade;
  removes the orchestrator's reorder burden; degenerate cases reduce to correct
  extremes.

## Consequences

Positive: a single `shed integrate` over a mixed heal/escalate wave is correct
and deterministic; the execute loop no longer reorders picks or splits waves;
in-group cascade (picks that genuinely share a file) is preserved and honest.

Negative: one extra `jj diff --name-only` call per pick; the `shed.integrate`
envelope changes from flat `stack` to nested `groups` (execute.md + plugin
SKILL.md must update in lockstep); `finish open` must add a collapse step to
reassemble the forest into a pushable line.

Neutral: conservative file-level grouping may co-group picks that do not actually
conflict (they stack cleanly and both land — safe). Relates to ADR `weft-hjx.7`
(single-stack `--skip-emptied` omission), which this decision references and
keeps in force within groups rather than superseding.
