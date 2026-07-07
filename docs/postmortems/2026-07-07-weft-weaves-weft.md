<!--
  ~ SPDX-License-Identifier: Apache-2.0
  ~ Copyright 2026 Weft Contributors
-->

# Post-mortem: the first `weft weaves weft` self-weave

**Date:** 2026-07-07
**Milestone:** roadmap §7 step 5 / v1.0 exit criterion 1 ("weft weaves weft")
**Weave session:** `746cd661-f664-4bea-a64a-0d18aea09914`
**Retro tracker:** `weft-rcc`
**Delivered:** `weft status` — epic `weft-j4c`, picks `weft-6xi` (Go subcommand) + `weft-79f`
(`/weft:status` skill) — merged as **PR #115** (+ steering groom PR #116).

## Verdict

**Exit criterion 1 partially passed, with a serious asterisk. weft can *weave* code,
but it cannot yet *ship a working feature* autonomously.** The authoring core ran
self-hosted and first-try; but the loop leaked at both ends, the `finish` stage was
completed entirely by hand, and — most importantly — **the delivered feature shipped
broken**, uncaught by the loop's own verification.

Every defect lived at a boundary where weft hands off to something outside itself: a
prebuilt binary, `gh`, real `bd`, a merged-and-deleted branch.

## What held — weft's strong center (self-hosted, first-try)

The authoring half of the loop worked autonomously on real code:

`weft-planner` → `weft plan check` → `weft plan emit` → `weft shed form/isolate/integrate`
→ `weft-executor` (TDD) → `weft pick verify` / `weft pick land` → clean loop termination.

Two waves, zero conflicts. The overlap-group model was correct: the skill pick, which
shares no files with the subcommand pick, was isolated on bare `trunk()` — an *ordering*
dependency, not a stack — and the orchestrator handed the predecessor's output contract
across the seam when dispatching the second executor. This is the part of weft that is
genuinely ready.

## Where it leaked

Each leak forced a fallback to raw `bd` / `gh` / `jj`:

| # | Stage | Symptom | Bead |
|---|-------|---------|------|
| 1 | `plan emit` | Picks created but left `parent:null` / `dependencies:null` (silent under-wiring). Root cause: a stale GoReleaser `dist/` binary predating the edge-wiring fix. Warp repaired by hand. | `weft-uwq` (closed via #114) |
| 2 | `finish open` | `gh pr create` omits `--head`; fails in a colocated jj repo (git HEAD is detached). The milestone PR was opened by a **manual `gh` command**. | `weft-fxj` (P1) |
| 3 | `finish reconcile` | Errors on the merged-and-deleted head branch / pruned remote bookmark. Reconciled by hand. | `weft-ojw` (P2) |
| 4 | The deliverable | `weft status` shipped **broken** (see below). | `weft-1ve` (P1) |

## The centerpiece: the deliverable shipped broken

Running the delivered feature for the first time (the weave never did):

```
$ weft status
no epics — warp is empty
aggregate: 0 epic(s) — closed 0, in_progress 0, blocked 0, open 0 (done 0, remaining 0)
```

…while the warp holds **19 open beads and 10+ closed epics**. Root cause: `status.go`
calls `bd list` **without `--all`** (lines 111 and 172), so it only ever sees *open*
work. Consequences:

- Closed epics are invisible — with zero open epics, a full warp reports as "empty".
- Closed children are counted as zero, so **"done" (= closed) is structurally always 0**
  — directly contradicting the epic's own acceptance criteria ("done = closed").

A "what's done vs. what's left" tool that can never show *done*.

### Why the loop didn't catch it

This is the most important lesson of the weave. The feature passed every gate — unit
tests, `weft pick verify`, clean land — and shipped non-functional, because:

- The unit tests fed **canned/fake `bd` JSON** (the `routeRunner` fixture) that did not
  model `bd`'s open-by-default listing behaviour.
- `weft pick verify` ran those same fakes.
- **The built command was never executed against the real warp** during the weave.

A green loop certified *compilation + fake-fixture behaviour*, not *integration
correctness*. TDD-with-fakes is necessary but not sufficient for a tool whose entire job
is to shell out to `bd` and `jj`.

## Followups

Filed by this post-mortem:

- **`weft-1ve`** (P1, bug) — `weft status` missing `--all`; can't show done/closed work.
- **`weft-4e8`** (P2) — add a real-execution smoke gate to the execute→verify loop for
  CLI / integration picks (the meta-fix).
- **`weft-p8t`** (P2) — E2E integration coverage for the whole `finish` family against a
  real merge lifecycle (covers `weft-fxj` + `weft-ojw`).

Filed by the weave: `weft-uwq` (closed), `weft-fxj`, `weft-ojw`. Related enhancement:
`weft-wv8` (surface non-epic standalone beads — a sibling facet of `weft-1ve`). Declined
(binary-currency guard — weft-on-weft is the edge case). Noted, not filed: gopls
module-root false positives in executor workspaces (environmental DX tax; recorded in
project memory).

## The path forward

Two tightly-linked fixes unlock the next weave to be genuinely self-sufficient:

1. **Harden `finish`** (`weft-fxj` + `weft-ojw`, with the E2E coverage in `weft-p8t`) so
   the loop *closes itself* — pass `--head <bookmark>` to `gh pr create`, and tolerate a
   merged-and-deleted branch / pruned bookmark on reconcile.
2. **Add real-execution verification** (`weft-4e8`) so the loop *cannot certify a broken
   feature* — build the binary and run its actual command against real (or scratch) state
   before landing.

As it stands, a human must both *ship* the work (`gh`/`jj`) and *notice it doesn't work*.
Closing those two gaps is the difference between "wove the code" and "shipped a working
feature".
