---
description: Walk a multi-phase project epic phase-by-phase through the per-phase rhythm — discuss, plan, execute, verify-work, finish — pausing at the two question gates and each PR merge wall. --auto skips the question gates.
argument-hint: "[project-epic-id] [--auto]"
---

<!--
  ~ SPDX-License-Identifier: Apache-2.0
  ~ Copyright 2026 Weft Contributors
-->

<!-- pure composition of weft's own skills/verbs — no GSD command maps to this; no new semantics -->

# phase-driver workflow

Thin outer loop over weft's per-phase rhythm. Given a **project epic** whose
children are phase sub-epics (a roadmap emitted by `new-project`'s multi-phase
path), it walks each phase through `discuss → plan-phase → execute → verify-work
→ finish open`, pausing only at the two interactive question gates (`discuss`,
`verify-work`) and at each phase's PR merge wall. It adds **no new semantics** —
every step invokes an existing skill or verb. Beads is the brain: the driver
holds no state of its own and re-derives its position from bead/PR artifacts on
every invocation, so re-running it mid-walk always resumes exactly where it left
off.

Use `new-project` to plan the roadmap first. `feature` covers single-epic
incremental work — which `phase-driver` also drives, as the degenerate one-phase
case below.

---

## Phase 0 — Resolve the target

Parse the argument: `<project-epic-id>` and an optional `--auto` flag (see
**Auto mode**). Load the target and enumerate its phase sub-epics:

```
bd show <project-epic-id> --json
bd list --parent <project-epic-id> --type epic --json
```

- **Has phase sub-epics** → a multi-phase roadmap; proceed to Phase 1.
- **No phase sub-epics** (the target is itself a feature / single-phase epic that
  carries picks, not phases) → **degenerate case**: treat the target epic *as*
  the single phase and run the rhythm once on it — Phases 2–4 with `<phase>` =
  the target epic, skipping the Phase 1 roadmap loop.

---

## Phase 1 — Select the current phase

The current phase is the **first non-closed phase sub-epic in dependency
order** — the one whose upstream phases (its `blocks` predecessors) are all
closed. `bd` folds transitive block-gating into the stored status, so a phase
still blocked by an earlier open phase carries the `blocked` status and is
excluded by:

```
bd list --parent <project-epic-id> --type epic --status open,in_progress --json
```

- **Empty** → every phase sub-epic is closed: the roadmap is complete. Journal a
  final `phase-driver: roadmap complete` breadcrumb on the project epic and exit.
- **One or more** → the current phase `<phase>` is the first by dependency order.
  (A `blocked` phase never appears here; if more than one is unblocked, take the
  earliest in the roadmap's `blocks` chain — `bd show` each candidate's
  blocked-by edges to order them.)

---

## Phase 2 — Re-derive the step cursor (beads is the brain)

Do not trust any remembered position. Compute the cursor from artifacts,
evaluating these signals **in canonical step order** —
`STEPS = [discuss, plan-phase, execute, verify-work, finish]` — and stop at the
**first step that is not concluded**. A later signal never overrides an earlier
unconcluded step; that ordering is what keeps the driver from shipping unfixed
work.

| Step | Concluded when | Probe |
|---|---|---|
| discuss | epic `design` field non-empty, **or** a `discuss:` note present | `bd show <phase> --json` (`.design`); notes |
| plan-phase | phase epic has ≥1 child pick | `bd list --parent <phase> --json` |
| execute | **no** open child picks (all closed/sealed) | `bd list --parent <phase> --status open,in_progress --json` is empty |
| verify-work | a `verify-work: all <N> deliverables passed` note **or** a `verify-work: skipped (--auto)` note is present, **and** no open child picks | `bd show <phase>` notes + the execute probe |
| finish | an open or merged PR exists for branch `<phase>` | `gh pr view <phase> --json state` |

If `bd` or `gh` is unavailable or errors, report the gap and stop — never guess
the position or advance on incomplete information.

**Fix-picks invariant.** A `verify-work: <F> of <N> deliverables failed; <F>
uat-fix picks filed` note is **not** a conclude signal — it co-occurs with open
`uat-fix` child picks, which make *execute* unconcluded (earlier in STEPS), so
the cursor lands on `execute`. The next pass weaves the fix picks, execute
concludes again, and only then does the (now-passing) verify-work re-run. There
is no state in which execute is unconcluded but verify-work is concluded.

---

## Phase 3 — Walk the rhythm from the cursor

Starting at the cursor step, run each remaining step in `STEPS` order. Steps
behind the cursor are already concluded — never re-run them. After each step,
journal a note on the phase epic (see **Journal**).

**discuss** (gate 1):
- **`--auto`** → skip; write
  `printf '%s' "discuss: skipped (--auto)" | bd note <phase> --stdin` so the
  cursor records the step concluded.
- **else** → invoke the `discuss` skill against the phase: `discuss <phase>`. It
  asks the HOW gray-area questions and locks decisions into the phase epic's
  `design` field (and writes its own `discuss: locked …` note). If it locks
  nothing (no gray areas), journal a `discuss: completed (no decisions)` note so
  the cursor still sees the step concluded.

**plan-phase**:
- Invoke the `plan-phase` skill: `plan-phase <phase>`. It dispatches the planner
  scoped to the phase, gates on **human approval**, then emits the picks into the
  phase sub-epic. If the operator **rejects** the plan at that approval gate, no
  picks are emitted (the gate precedes `weft plan emit`, so there is no partial
  state) — stop and surface; re-invocation resumes here (the cursor still sees no
  child picks).

**execute**:
- Invoke the `execute` skill: `execute <phase>`. It weaves waves until the
  phase's ready set is empty. If it checkpoints or escalates a pick, stop and
  surface — re-invocation resumes execute (the cursor sees unsealed picks).

**verify-work** (gate 2):
- **`--auto`** → skip; write
  `printf '%s' "verify-work: skipped (--auto)" | bd note <phase> --stdin`, then
  trust the per-pick machine `weft pick verify` gate that `execute` already ran.
- **else** → invoke the `verify-work` skill: `verify-work <phase>`. It walks the
  deliverables y/n one at a time and either writes
  `verify-work: all <N> deliverables passed` or files `uat-fix` picks under the
  phase epic. If it files fix picks, re-derive (Phase 2): the cursor returns to
  execute to weave them, then verify-work re-runs.

**finish**:
- Invoke `weft finish open <phase>`. This pushes the phase's stack on a bookmark
  named `<phase>` and opens a GitHub PR (title `<phase-title> (<phase>)`).
- Journal `phase-driver: phase <phase> woven` on the **project** epic.
- **Merge wall — PAUSE.** Tell the operator verbatim: *"PR for phase `<phase>`
  is open (branch `<phase>`). Merge it, then re-run
  `phase-driver <project-epic-id>` to continue."* Stop. `bd ready` will not
  release the next phase until this phase's epic closes, which requires the
  merge (Phase 4).

---

## Phase 4 — Reconcile, close, advance

On re-invocation, Phase 2 finds the finish step concluded (a PR exists for
`<phase>`). Check whether it merged:

```
gh pr view <phase> --json state
```

- **`OPEN`** → still awaiting merge: restate the merge-wall instruction and stop.
- **`CLOSED`** (closed without merging) → an anomaly, not a normal step: the
  phase's PR was abandoned. Surface it plainly and stop — do **not** loop the
  merge-wall message or advance. The operator decides whether to reopen/re-`finish
  open` the phase or rework it; the driver does not guess.
- **`MERGED`** → reconcile, then close the phase:

  ```
  weft finish reconcile <phase>
  bd close <phase> --reason "phase shipped: PR merged + reconciled"
  ```

  `finish reconcile` cleans up the local jj stack after the merge; `bd close` is
  what releases the next phase — **reconcile does not close the epic bead**.
  Journal `phase-driver: phase <phase> shipped` on the project epic, then loop
  back to **Phase 1** for the next phase (multi-phase) or exit (degenerate
  single-phase case).

---

## Auto mode (`--auto`)

`--auto` is the only sanctioned path toward unattended multi-phase weaving, and
is deliberately partial in v1. It **skips the two question gates** — it does not
run `discuss` / `verify-work` headlessly (neither skill ships an auto mode):

- skip discuss → the planner proceeds on whatever the `design` field already
  holds (or its own inference); the driver writes `discuss: skipped (--auto)`.
- skip verify-work → trust the machine `weft pick verify` gate that `execute`
  already ran per pick; the driver writes `verify-work: skipped (--auto)`.

`--auto` **still pauses at every merge wall** — v1 never merges PRs unattended.
Honest scope: unattended weaving up to each ship boundary, not the whole project.

---

## Journal

The driver writes one progress note per step on the **phase epic** (beads is the
brain; weft's analog of GSD's per-phase SUMMARY/CONTEXT — no sidecar files):
what discuss locked, what plan-phase emitted, what execute landed, what
verify-work found and which fix picks it filed, plus any lessons or deferred
ideas to carry into the next phase's `discuss`. A one-line
`phase-driver: phase <phase> woven` / `… shipped` breadcrumb also lands on the
**project epic** for the roadmap-level view.

These notes double as cursor disambiguation: a skipped discuss or a clean verify
leaves no other artifact, so the `discuss:` / `verify-work:` note is what
distinguishes "done, nothing to show" from "not yet done."

---

## What this workflow does NOT do

- It does not plan the roadmap or discover phases — that is `new-project`; the
  driver only walks an existing roadmap.
- It does not introduce new scheduling — `bd ready` + `blocks` edges remain the
  scheduler; the driver only reads the order.
- It does not merge PRs — the human merges (a future auto-merge overlay may
  change this); the driver pauses at each wall.
- It does not stack phases — v1 is serial: phase N lands in trunk before N+1
  starts.
- It does not write `.planning/` files or its own state file — the journal lives
  in bead notes, and position is re-derived from artifacts.
