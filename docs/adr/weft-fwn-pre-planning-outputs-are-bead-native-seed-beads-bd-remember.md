<!-- markdownlint-disable MD013 -->
<!-- adr-render: source=bd:weft-fwn; do not edit manually; use `/adr update weft-fwn` -->

# Pre-planning outputs are bead-native (seed beads + bd remember + throwaway jj), not .planning/ files

**Date:** 2026-06-12
**Status:** Accepted
**Decision:** weft-fwn
**Deciders:** Sean Brandt

## Context

weft-ccy.4 ports GSD pre-planning skills /gsd-explore and /gsd-spike to weft. GSD stores ideation outputs in .planning/{notes,todos,seeds,research}/ files and spike experiments in .planning/spikes/NNN/ directories with a MANIFEST.md verdict. weft replaces that entire file-based pre-planning surface with bead-native equivalents — the single most load-bearing substrate-translation choice in the ccy.4 scope, completing the beads-replace-.planning/ contract (design.md Layer B) for the front of the funnel.

## Decision

explore emits deferred seed-labelled beads (bd create --type task --labels seed, then bd defer <id>) plus bd remember entries; spike runs each experiment in a throwaway jj change (save @ -> jj new -> experiment -> jj abandon -> jj edit back) whose only durable outputs are a closed spike bead (chore, label spike) and a bd remember finding. No .planning/ files, no kept spike code.

## Rationale

- design.md establishes beads as the Layer B replacement for .planning/ files; this completes that replacement for the pre-planning surface.
- Seed beads are schedulable: bd ready excludes deferred seeds automatically; bd list --label seed is the backlog view; a trigger in the description drives promotion (bd undefer).
- jj is purpose-built for throwaway experiments: the working copy is always a commit (in-flight work auto-snapshotted), jj abandon cleanly discards, and jj edit <saved-change-id> restores the user — no stash/branch dance.
- bd remember findings are injected by bd prime at every future session start, making validated spike results available to discuss/plan-phase without a call edge.
- No new bead type or engine surface: seeds are ordinary deferred beads with a label; spike beads are chore-typed with a spike label.

## Alternatives Considered

**Bead-native (chosen)** — outputs land in the warp graph (schedulable, query-able, surfaced by bd prime); no out-of-band files diverge from bead state; spike code disposable via jj abandon; consistent with every other ccy skill. Costs: bd remember surfaces next-session (same-session spike->plan uses the epic design-field path); seed deferral is two-step (bd create has no --status).

**Retain GSD .planning/ files** — zero-friction port, but contradicts the Layer B replacement contract; files are not schedulable/query-able/prime-surfaced; spike artefacts accumulate without a clean lifecycle. Rejected.

**Hybrid (remember for decisions, .planning/ for spike artefacts)** — richer artefact storage, but two state surfaces, partial replacement, manual spike cleanup. Rejected as inconsistent with the epic spec §7 no-.planning/ constraint.

## Consequences

**Positive:** the whole pre-planning surface (ideation -> feasibility -> plan) is bead-backed and query-able via bd list/bd prime; spike code cannot survive (jj abandon is the only exit); seed beads are first-class warp citizens (can acquire parents, be promoted, participate in bd ready when triggered); no new bead type or engine surface.

**Negative:** bd remember latency — a spike finding written in session N surfaces automatically only in session N+1 (bd prime runs at session start); same-session spike->plan-phase needs the epic design-field workaround. Two-step seed creation (bd create then bd defer) since bd create has no --status flag.

**Neutral:** sketch (3rd GSD pre-planning skill) split to a follow-up bead — its HTML output is not bead-shaped and its ui-phase consumer is unported (backlog sequencing, not a substrate contract). Both skills are prompt-layer, no engine change.
