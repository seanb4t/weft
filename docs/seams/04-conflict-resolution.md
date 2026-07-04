<!--
  ~ SPDX-License-Identifier: Apache-2.0
  ~ Copyright 2026 Weft Contributors
-->

# Seam 4 — Conflict-resolution UX

> Status: **shipped** (design-reviewer READY, round 2). Sub-spec of
> [`docs/design.md`](../design.md) §9 open seam 4. Tracked as bead `weft-hjx.4`
> (child of `weft-hjx`). Rounds 1–2 findings fixed inline (pick-land gate into
> seam 1; resolution-workspace kind into seam 3 reaper; explicit squash --into).
> Implemented and released.

## 1. Scope

What happens when `shed integrate` (seam 1) produces first-class jj
**conflicts** — the downstream of seam 2's deliberate *tolerate* path (§2 §4.2:
incidental file-overlap is allowed to become a conflict rather than serialized).
This seam defines the resolution flow, the `weft conflict` verbs, the resolver
agent's contract, and escalation.

Out of scope: the integration that surfaces conflicts (seam 1 `shed integrate`);
the workspace primitives reused here (seam 3); the planning policy that decides
which overlaps to tolerate (seam 2).

## 2. Model: resolver executor + escalation

Conflicts are resolved **post-hoc** (design.md §4), by a **fresh-context
resolver agent** — not the orchestrator (whose context must stay clean,
design.md §1) and not, by default, the human. The engine choreographs the
dangerous jj steps; the agent supplies the merge judgment; the human is the
escalation path, not the first responder.

This mirrors GSD's auto-fix → ask-user pattern (`gsd-code-fixer`) but is
beads-native: escalation is a `bd human` flag, not an interactive prompt.

### 2.1 Why resolve at the *lowest* ancestor

jj's heal-the-stack property (grounded: deepwiki `jj-vcs/jj`): resolving a
conflict at the lowest conflicted commit and `jj squash`-ing it triggers
auto-rebase of all descendants, and conflict-simplification prevents recursive
conflicts. So **one resolution can heal several picks**. Weft resolves
**bottom-up**, re-querying `conflicts()` after each finalize — a descendant
conflict often vanishes once its ancestor is fixed, and is never resolved twice.

## 3. The flow

`shed integrate` emits `conflicts[]` on exit 0 (seam 1) — each entry carries
`{bead, change, paths, lowest_ancestor}`. The orchestrator then loops:

```
while conflicts() is non-empty:
  L = lowest conflicted ancestor
  weft conflict open L          # engine: jj new L in a resolution workspace;
                                #         emit the conflicting beads as context
  → dispatch a fresh resolver agent (host runtime) into that workspace:
        read the conflicting beads' descriptions (= the plan, §5) + markers,
        EDIT THE MARKERS to a correct merge, remove them
  weft conflict finalize L      # engine: jj diff --git (verify only the
                                #         resolution shows) → jj squash (heals
                                #         descendants) → re-check conflicts()
  if still conflicted / verify fails / agent gave up:
        bd human <bead>         # escalate; block land on affected picks
```

Independent (non-conflicted) picks in the wave proceed to verify/land normally;
only the picks under an unresolved conflict are blocked.

**Conflicts array schema note:** `shed integrate.conflicts[]` is `[{bead, change}]`
(actionable) — each entry is directly consumable by `conflict open <bead>`, and
the orchestrator uses this form to drive the resolution loop above. By contrast,
`resume.conflicts[]` is `[]string` (bare change-ids, observability only); resume
cannot map change-ids to beads because it does not have access to the wave stack.

## 4. The `weft conflict` verbs

Extends the [seam 1](01-command-surface.md) surface. Two coarse verbs **bracket**
the agent's marker-editing — the engine owns the jj choreography (dangerous,
multi-step), the agent owns the merge judgment.

| Verb | Kind | Wraps | Notes |
|---|---|---|---|
| `conflict open <lowest>` | coarse | `jj new <lowest>` in a resolution workspace (§4.1) + emit the conflicting beads (the "sides") and `paths` as resolver context | Sets `ui.conflict-marker-style = diff` for the workspace (§5). Output is the resolver's brief: which beads collided, on which paths, what each intended. |
| `conflict finalize <lowest>` | coarse | `jj diff --git` (assert only the resolution shows) → `jj squash --into <lowest>` (fold the resolution in, heal descendants) → re-query `conflicts()` + reap the resolution workspace | `--into <lowest>` is **explicit** (not bare `jj squash`) so the fold targets the conflicted ancestor even if `@` is not its direct child. Exit 0 + `{healed: [...], remaining_conflicts: [...]}`. A still-conflicted result is **data**, not an error (seam-1 contract). |

There is no `conflict list` verb — the conflict set comes from `shed integrate`'s
`conflicts[]` and from `weft resume` (which already surfaces unresolved
conflicts, seam 1 §4.5). `jj log -r 'conflicts()'` is the ground truth.

### 4.1 Resolution workspace identity & reaping

The resolution workspace is a **second workspace kind** (seam 3's reaper is now
kind-aware):

- **Name:** `<sanitized-bead-id>-resolve` (the lowest conflicted ancestor's
  bead-id, sanitized per seam 3 §3, plus a `-resolve` suffix). Path follows
  seam 3 layout: `../<repo>_worktrees/<sanitized-bead-id>-resolve/`.
- **Reaper join:** seam 3's `weft reap` recognizes the `-resolve` suffix, strips
  it, `desanitize`s the remainder to the **owning** bead-id, and reaps unless
  that bead is `in_progress` with a live resolver. Bead-ids never end in
  `-resolve`, so the suffix is an unambiguous kind discriminant.
- **Lifecycle:** `conflict finalize` reaps the workspace on the happy path
  (`jj workspace forget` + `rm -rf`, seam 3); an interrupted `finalize` leaves
  an orphan that `weft reap` collects via the rule above. No new lifecycle
  machinery — only the kind-aware name parse.

## 5. Marker style & agent safety

- **Pin `ui.conflict-marker-style = "diff"`** (jj's default). It is the only
  built-in style that represents **multi-sided (3+) conflicts natively** — git
  (diff3) style falls back to snapshot for >2 sides. A shed can produce N-sided
  conflicts, so diff style is the deterministic choice, and it matches the jj
  skill's resolution recipe (apply each `%%%%%%%` diff to the `+++++++`
  snapshot). `conflict open` sets it on the resolution workspace.
- **The resolver agent MUST edit markers directly and MUST NOT run
  `jj resolve`** — `jj resolve` launches an interactive merge tool and hangs in
  a non-interactive agent (jj skill, agent rules). The resolver edits the file,
  removes the markers, and returns; jj recognizes the resolution on its next
  working-copy scan.
- The resolver follows the jj agent profile already in force (`--no-pager`,
  `--git` diffs, change-ids not commit-hashes, edit-markers-not-`jj resolve`).
  It does not `jj commit` — it edits markers and returns; `conflict finalize`
  does the squash — so the profile's `-m`-always and start-of-task `jj git
  fetch` do not apply to it.

## 6. Escalation

When `conflict finalize` reports the change is still conflicted, or the resolver
agent reports it cannot produce a correct merge (genuine semantic ambiguity, or
a 3+-sided conflict it cannot reconcile), Weft escalates:

- `bd human <bead>` flags the conflicted pick's bead for human decision (the
  beads-native equivalent of GSD's "Fix now / Continue" prompt).
- The affected picks (the conflicted ancestor and any descendants not yet
  healed) are **blocked from `pick land`** until a human resolves or
  `pick redo`s them. The gate is concrete: a change that is still in
  `conflicts()` must not be sealed/landed — `pick land` asserts the pick's
  change is conflict-free before `bd close`.
- The rest of the wave is unaffected — independent picks land, and the epic
  simply cannot `finish` until the flagged conflict is cleared.
- `weft resume` (seam 1 §4.5) surfaces both signals in their **respective
  fields** — an unresolved conflicted change under `conflicts:` (the jj
  `conflicts()` ground truth), a `bd human`-escalated pick under `blocked:` — so
  a resuming session sees both what is conflicted and what was escalated, without
  conflating them in one field.

Escalation is **not** automatic abandonment: the conflicted change and its
markers persist (jj records conflicts in the commit), so a human can resolve at
leisure with a full merge tool, or `weft pick redo` to re-run the pick from
scratch.

## 7. Not a warp bead

Conflict resolution is a **transient sub-step of wave integration**, not a new
warp bead. The warp is the *plan* (design.md §3); a conflict is a mechanical
consequence of weaving two picks, not a planned unit of work. It is tracked
where it belongs:

- unresolved → the conflicted bead's `bd human` flag (§6);
- resolved → the heal is absorbed into the existing pick's change (no trace
  needed beyond `jj op log`).

This keeps the warp clean — consistent with seam 1's deliberate non-verbs and
seam 3's "reaping is a sub-step, not a bead."

## 8. Open sub-seams (next design steps)

- The resolver agent's prompt/brief format (what `conflict open` emits) — and
  whether it ports from a GSD agent (seam 5).
- Loop-bound / oscillation guard: a cap on resolve attempts per conflict before
  forced escalation (avoid an agent thrashing on an unresolvable merge).
- Post-resolution verification depth: does `finalize` also re-run the pick's
  `verify` gate, or only assert `conflicts()` shrank?
- Whether `ui.conflict-marker-style` is pinned per-workspace or repo-wide
  (coupled to the jj agent-config the engine writes).

## 9. Cross-spec note

Introduces the `weft conflict` verb group (`open`, `finalize`) — additive to the
seam-1 surface. Consumes seam 1 `shed integrate`'s `conflicts[]` and is the
resolution path for seam 2's tolerated overlaps. Two reconciliations were
applied to keep the READY seams consistent (both updated alongside this spec):

- **seam 1 `pick land`** now asserts the change is conflict-free (∉
  `conflicts()`) before `bd close` — the §6 land-gate is a real invariant, so it
  lives in the verb definition, not only here.
- **seam 3 `weft reap`** is now kind-aware: it recognizes `<…>-resolve`
  resolution workspaces (§4.1), strips the suffix to the owning bead, and reaps
  by the same liveness rule — so an interrupted `conflict finalize` never
  strands a workspace the reaper can't classify.

## Attribution

Resolution model (auto-fix agent + human escalation) contrasts with **GSD
Core**'s `gsd-code-fixer` + "Fix now / Continue" gate, MIT-licensed, © its
contributors. Weft is independently licensed Apache-2.0.
