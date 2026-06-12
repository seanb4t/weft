<!--
  ~ SPDX-License-Identifier: Apache-2.0
  ~ Copyright 2026 Weft Contributors
-->

# `phase-driver` skill — thin interactive outer loop over the per-phase rhythm (ccy.8) — design

**Date:** 2026-06-11
**Bead:** weft-ccy.8 (parent epic: weft-ccy — Restore GSD Layer-A interactive/phased loop)
**Status:** design
**Port source:** none — pure composition of weft's own skills/verbs (no GSD command maps to this)

## Context

`weft-ccy` restores GSD's per-phase interactive rhythm over weft's stable verb
surface + beads. Phases A–D landed the pieces: the two interactive gate skills
(`discuss` ccy.1, `verify-work` ccy.2), the engine enablers (ccy.5), the phased
planner / JIT roadmap (ccy.3), and the lightweight entries (`feature`,
`onboard`, ccy.6/.7). Each phase of a multi-phase project is a sub-epic that
ships as one PR, and the per-phase rhythm is run **by hand** today:

```text
discuss → plan-phase → execute --epic <phase> → verify-work → finish open
```

The 06-09 spec (S1, "Driver, layered later"; decision row "Both, layered")
defers a **thin driver skill** that walks this rhythm phase-by-phase so the
operator does not re-issue five skills per phase across an N-phase roadmap. This
is phase E of the `weft-ccy` reshape. The bead is explicit: *"Pure composition
of the gate skills + existing verbs; no new semantics."*

## Decisions made in this session

| Decision | Choice |
|---|---|
| PR granularity | **Per-phase PR, unchanged.** No stack-and-collapse. `epic = ship unit = one PR` is preserved; a phase is a genuine ship/review boundary. |
| "Too many boundaries" pain | Solved at the **planner (ccy.3)**, not here: create a phase only at a real checkpoint (hard dependency, deploy/migration gate, review seam); continuous parallelizable work is *waves within one phase*, which `execute` already handles wall-free. The driver walks whatever phasing exists. |
| Merge wall | The driver **pauses at each phase's PR for the human to merge**, then reconciles and advances. Forced by reality: `bd ready` will not release phase N+1's picks until phase N's epic closes on `reconcile`. Serial — phase N lands in trunk before N+1 starts. |
| Position model | **Cursor re-derived from artifacts every invocation** (robust to out-of-band human action) **+ a per-step journal note** on the phase epic (audit trail, skipped/clean-step disambiguation, cross-phase lessons). No driver-state file; beads is the brain. |
| Entry / walk | `phase-driver <project-epic-id>`; walk sub-epics in dependency order, next = first non-closed; never reorder (`blocks` edges authoritative). |
| Single-phase | Degenerate case: target has no phase sub-epics → run the rhythm **once** on the epic. `phase-driver` doubles as "walk one epic through the full interactive rhythm." |
| Auto mode | `--auto` **skips the two question gates** (`discuss`, `verify-work` — neither has a headless mode) and trusts the machine `pick verify` gate; **still pauses at each merge wall**. v1 scope: "unattended weaving up to a ship boundary," not "unattended whole project." |
| Name | `phase-driver` (matches `verify-work` / `plan-phase` hyphenated pattern). |

## 1. What the skill is

A thin outer loop. Given a project epic, it walks each phase sub-epic through
the per-phase rhythm, pausing only at the two interactive gates the spec names
(`discuss`, `verify-work`) and at the per-phase merge wall, journaling each step
to beads. It introduces **no new semantics** — every step is an invocation of an
existing skill or verb. Remove the loop, the merge-wall pause, and the journal,
and what remains is exactly the five skills an operator runs by hand today.

It is *not* a new scheduler (that is `bd ready`), *not* a new emission mode (the
roadmap already exists), and *not* a replacement for any gate (it invokes them).

## 2. Position model — re-derive the cursor, journal the walk

Because the driver is re-invoked at every merge wall (and after any dropped
session), it must answer "which phase, which step?" from durable state, not
in-memory progress. Two distinct jobs, resolved differently:

**Authoritative cursor — re-derived from artifacts.** A note cannot be the
authority for position: a human acts out of band (merges the PR, closes the
epic, files a fix pick) and a written `step=…` marker goes stale. Artifacts
cannot lie that way. Each invocation the driver computes:

- **Current phase** = the first phase sub-epic, in dependency order, that is not
  closed (the one `bd ready` would release next). Single-phase target → the
  target epic itself.
- **Step within the phase**, from concrete signals:

  The note texts below are the **actual** strings emitted by the composed
  skills — the cursor matches the established `<skill>:`-prefix convention
  (`verify-work` SKILL.md §9; `discuss` SKILL.md), not invented formats:

  | Signal | Step concluded |
  |---|---|
  | epic `design` field populated, *or* a `discuss:` note present (`discuss: locked N decisions …` from the skill, or `discuss: skipped (--auto)` from the driver) | discuss done |
  | phase epic has child picks | plan-phase done |
  | all child picks sealed/closed | execute done |
  | `verify-work: all <N> deliverables passed` note (the skill's all-pass end-state), *or* `verify-work: skipped (--auto)` (driver) — **and** no open fix picks | verify-work done |
  | PR open for the phase epic (`gh pr list --head <branch>`) | finish open done |
  | PR merged | reconcile pending → run it, advance phase |

  A `verify-work:` *failure* note (`verify-work: <F> of <N> deliverables failed;
  <F> uat-fix picks filed`) is **not** a conclude signal — it co-occurs with open
  `uat-fix` picks, which the strict-ordering rule below resolves.

  **Strict ordering — the rule that resolves the fix-picks case.** The signals
  are evaluated *in order*, and the cursor is the **earliest unconcluded step**:
  a later signal NEVER overrides an earlier unconcluded one. This is what keeps
  the driver from shipping unfixed work. When `verify-work` files fix picks, two
  things are true at once — there is no `verify-work: all <N> deliverables passed`
  note (it failed), and there are now open child picks. The open picks make *execute* unconcluded,
  which is earlier than verify-work, so the cursor is **`execute`** regardless of
  any verify-work note. The next run weaves the fix picks, execute concludes
  again, and only then is the (now-passing) `verify-work` re-run. The
  verify-work-done signal is therefore deliberately the **all-pass note only**
  (`verify-work: all <N> deliverables passed`) or the driver's `--auto` skip
  note — a failure note (`verify-work: <F> of <N> deliverables failed; …`) is
  *not* a conclude signal; it is a journal entry whose open `uat-fix` picks reset
  the cursor to execute. There is no state in which execute is unconcluded but
  verify-work is concluded.

**Journal — rich per-step notes on the phase epic.** Weft's analog of GSD's
per-phase SUMMARY/CONTEXT (beads is the brain; no sidecar files). The driver
writes one note per step: what discuss decided (the `design` field already holds
the locked decisions; the note records that it ran), what plan-phase emitted,
what execute landed, what verify-work found and which fix picks it filed,
lessons, and deferred ideas. The journal earns its keep three ways: (1) audit
trail for the human and for review; (2) disambiguation — a *skipped* discuss or
a *clean* verify leaves no artifact, so a one-line note distinguishes "done,
nothing to show" from "not done yet"; (3) cross-phase carry — lessons and
deferrals from phase N feed phase N+1's `discuss`. A one-line "phase N woven"
breadcrumb also lands on the project epic for the roadmap-level view.

## 3. The loop

```text
phase-driver <project-epic-id> [--auto]

  resolve project epic; enumerate phase sub-epics (dependency order)
  STEPS = [discuss, plan-phase, execute, verify-work, finish]   (canonical order)

  loop:
    phase  ← first non-closed sub-epic   (none left → roadmap complete, exit)
    cursor ← first step in STEPS that is NOT concluded for `phase`   (§2)

    starting at `cursor`, walk STEPS forward, running each in turn:
      discuss      [gate 1]  invoke `discuss <phase>`        (--auto: skip + note)
      plan-phase             invoke `plan-phase <phase>`     (approval gate; reject → stop)
      execute                invoke `execute <phase>`        (loops waves to empty)
      verify-work  [gate 2]  invoke `verify-work <phase>`    (--auto: skip + note;
                                                              files fix picks → re-derive
                                                              cursor lands back on execute)
      finish                 invoke `finish open <phase>`    → PR opened
                             journal "phase woven"; PAUSE  ──► tell operator:
                               "PR #N open for phase <phase>. Merge it, then
                                re-run phase-driver <project-epic-id>."

    on re-invocation, PR merged:  `finish reconcile`; advance to next phase
```

`cursor` is the **first unconcluded step** (not "last concluded"), and the walk
runs that step and every later step in `STEPS` order; concluded steps are simply
already behind the cursor, so they are never re-run. There is no ordinal `<`
comparison — re-derivation skips concluded steps by construction.

The merge-wall pause is the only place the driver yields control outside the two
named gates, and it is not a *question* — it is a hard dependency on a human (or,
later, an auto-merge overlay) merging the PR before `bd ready` can release the
next phase. (The 06-09 spec described the driver as "pausing at the two
interactive gates"; that named the *question* gates. The merge wall is an
additional, non-question hard-dependency pause the 06-09 prose elided — a
refinement, not a contradiction. `plan-phase`'s own approval gate is a third
non-question pause in the same category.) Re-invocation is cheap and idempotent: the re-derived cursor skips
every concluded step, so re-running `phase-driver` mid-walk resumes exactly
where it left off.

## 4. Auto mode

`--auto` is the only sanctioned path toward unattended multi-phase weaving, and
it is deliberately partial in v1. It **skips the two question gates** — it does
not invoke `discuss`/`verify-work` in a headless form, because those skills
intentionally ship no auto mode (06-09 spec §3/§4). Skipping `discuss` means the
planner proceeds on whatever the epic `design` field already holds (or its own
inference); skipping `verify-work` means the walk trusts the machine `pick
verify` gate that `execute` already ran per pick. `--auto` **still pauses at
each merge wall** — v1 does not merge PRs unattended. Honest scope: "unattended
weaving up to each ship boundary."

When `--auto` skips a gate it writes the disambiguating journal note the cursor
expects (§2), with an explicit prefix so it is never confused with a real run or
a failure: `discuss: skipped (--auto)` and `verify-work: skipped (--auto)`. Note
that `verify-work: skipped (--auto)` is a distinct signal from the interactive
skill's all-pass note (`verify-work: all <N> deliverables passed`) — both
conclude the verify-work step for cursor purposes, but only the latter asserts
deliverables were actually walked. Stacked-PR / auto-merge that would remove the
merge wall is a future overlay (see Out of scope).

## 5. Composition surface (no new semantics)

| Step | Existing surface invoked | Interactive? |
|---|---|---|
| discuss | `discuss <phase-epic-id>` (ccy.1) | gate 1 |
| plan | `plan-phase <phase-epic-id>` (ccy.3) | approval gate — pauses for human; rejection can stop the driver |
| execute | `execute <phase-epic-id>` | autonomous wave loop |
| verify-work | `verify-work <phase-epic-id>` (ccy.2) | gate 2 |
| finish | `weft finish open` → (human merge) → `weft finish reconcile` | merge wall |
| schedule | `bd ready` / dependency-ordered sub-epic enumeration | — |

Everything the driver does is one of the above plus the loop, the merge-wall
pause, and `bd note` journaling. Strip those three and you have today's manual
rhythm.

## 6. Error / edge handling

| Situation | Driver behavior |
|---|---|
| Target epic has no phase sub-epics | Single-phase degenerate: run the rhythm once on the target epic. |
| `plan-phase` approval rejected | Surface to operator and stop. `plan-phase`'s approval gate is **before** its `weft plan emit`, so a rejection emits **zero** picks — no partial state. The cursor sees no child picks and correctly lands before plan-phase on re-invocation. (The driver relies on this atomicity: emit is one upsert call gated by approval; it never half-creates a phase's picks.) |
| `execute` checkpoints / escalates a pick | Stop at the execute step; surface. The cursor sees picks not yet sealed and resumes execute on re-invocation. |
| `verify-work` files fix picks | Phase epic is not done (open picks) → cursor stays at execute; the next run weaves the fix picks, then re-verifies. |
| PR not yet merged at re-invocation | Re-derive lands on "finish open done, merge pending"; re-state the merge instruction; do not advance. |
| Human closed the phase epic out of band | Cursor sees it closed → advance to the next phase. |
| `bd` / `gh` unavailable | Degraded mode: report the gap, stop; do not guess position. |

## 7. Out of scope

- **Auto-merge / stacked-PR flow** that removes the merge wall (open all phase
  PRs as a stack, merge in dep order later, epic closes at `finish open` rather
  than `reconcile`). A future overlay; needs a small close-on-open semantics
  shift and changes what `verify-work` runs against (stacked workspace vs
  trunk). v1 is serial-merge.
- **Headless discuss / verify-work.** Those skills ship adaptive mode only;
  `--auto` skips the gates rather than running them non-interactively.
- **Re-phasing / roadmap edits mid-walk.** The driver walks the roadmap as
  emitted; changing the phase structure is a `plan`/re-plan concern.
- **New scheduling semantics.** `bd ready` + `blocks` edges remain the
  scheduler; the driver only reads the order.

## 8. Testing & validation

- **Prompt-layer gates:** `claude plugin validate ./plugin --strict` and
  `. --strict`; the intra-tree path-citation grep discipline (reject
  `weft/(agents|references|workflows)/` paths in `plugin/`).
- **Walk-through validation (phase F dogfood):** drive a real multi-phase
  project end-to-end — confirm the cursor re-derivation resumes correctly across
  merge walls, the journal notes accumulate per phase, and `--auto` weaves up to
  each ship boundary without invoking the question gates.
- **Degenerate case:** `phase-driver <feature-epic-id>` (no sub-epics) runs the
  rhythm once and ends at the merge wall.

## ADR impact

No new architectural decision beyond what `weft-cfp` (+ its phasing addendum)
and the 06-09 spec already record. The driver is the "Both, layered" decision's
second layer, realized as pure composition. If `/capture-adrs` finds anything
ADR-worthy after the plan, the likely candidate is the **serial-merge-wall**
choice (driver pauses for human merge; stacked/auto-merge deferred) — capture
only if the plan surfaces it as a load-bearing decision.
