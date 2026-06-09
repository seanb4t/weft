---
description: Weave a wave of ready picks — form the shed, isolate, dispatch executors, integrate, resolve conflicts, land.
argument-hint: "[epic-id]"
---

<!--
  ~ SPDX-License-Identifier: Apache-2.0
  ~ Copyright 2026 Weft Contributors
-->

<!-- adapted from /gsd-execute-phase (GSD Core, MIT) -->

# execute workflow

Thin orchestrator for `/weft-execute`. Drives the full weave loop: wave
formation, per-pick isolation and dispatch, verify gate, dep-ordered
integration, conflict resolution, landing, and workspace cleanup. Loops until
the epic's ready set is empty.

**Adapted:** wave orchestration methodology, verify-gate discipline, and
fresh-context per-pick dispatch are adapted from GSD Core's execute-phase
workflow.

**Rewritten (§4 tool-layer mapping):** all per-step execution uses only the
stable weft verb surface (seams 1–4). GSD's per-checkout git-isolation
choreography and its state-tracking artifacts are replaced by `weft shed
isolate`/`shed cleanup` (jj workspaces), beads as the authoritative state
store, and `weft resume` as the between-wave state projection. The
GSD-era build-tool integration and shared tracking files are dropped; the
warp lives in beads.

---

## 0. Prerequisites

Before the first wave, run `weft reap --epic <id>` to collect any crash-orphan
workspaces left by a previous interrupted session. This is idempotent and safe
to call even when no orphans exist. If `weft resume --epic <id>` reports
unresolved conflicts from a prior wave, resolve them before forming a new wave
(the conflict open/finalize loop in §4 step 6 applies).

---

## 1. Inputs

| Input | Description |
|-------|-------------|
| `epic-id` | The bead ID of the driving epic (required). |

---

## 2. Verify-gate methodology

The verify gate (`weft pick verify <bead>`) exits `0` in all cases where the
engine ran successfully — **including when the gate verdict is failing**. A
non-zero exit signals an engine invocation error, not a failing pick. Branch
exclusively on `data.pass` in the JSON envelope:

```json
{"ok": true, "verb": "pick.verify", "data": {"pass": true,  "bead": "...", "change": "..."}}
{"ok": true, "verb": "pick.verify", "data": {"pass": false, "bead": "...", "change": "...", "reason": "..."}}
```

When `data.pass` is `false`, the executor re-examines and fixes, then re-runs
`weft pick verify` before sealing. The orchestrator does not seal on a failing
gate.

Where deeper adversarial review is warranted (e.g., security-sensitive beads,
or beads with acceptance criteria that require cross-file analysis), dispatch
`weft-reviewer` (see `${CLAUDE_PLUGIN_ROOT}/agents/reviewer.md`) fresh into the pick's
workspace. The reviewer emits its verdict in the `pick.verify` envelope shape;
the orchestrator reads `data.pass` the same way.

The verdict is always DATA. Exit codes are engine-health signals, not gate
verdicts.

---

## 3. Fresh-context per-pick dispatch

Each pick receives its own fresh agent context. This is not optional:
accumulated context from earlier picks contaminates reasoning about later ones.
The executor (`${CLAUDE_PLUGIN_ROOT}/agents/executor.md`) is dispatched one instance per
pick, into that pick's isolated workspace, with the bead-id as its only
cross-session anchor. The executor does not share state with sibling executors
in the same wave.

The model used for each executor is determined by the bead's `model:*` label
(the per-bead routing convention — see `${CLAUDE_PLUGIN_ROOT}/references/bead-change-spine.md`),
falling back to the default declared in the agent's frontmatter (`sonnet` for
`${CLAUDE_PLUGIN_ROOT}/agents/executor.md`) when the bead carries no `model:*` label. The
orchestrator reads the bead label before dispatch and routes accordingly.

---

## 4. The weave loop

The loop runs until `weft shed form --epic <id>` returns an empty wave. At that
point the epic's ready set is exhausted and the loop terminates naturally. There
is no explicit finish step in this loop — `weft pick land` + `weft shed cleanup`
closes each wave; `weft resume` projects what remains; the next iteration of the
outer loop checks whether anything is still ready.

### Step 1 — Form the wave

```
weft shed form --epic <id>
```

Returns the set of ready bead-ids as JSON (the current shed). The wave size is
capped by `shed.max` (seam 3 config). If the returned wave is empty, the
epic's ready set is exhausted — exit the loop.

### Step 2 — Isolate workspaces

```
weft shed isolate <bead>...
```

For each wave member: sets bead status to `in_progress`, then creates an
isolated jj workspace at the correct revision. The `in_progress` transition
happens at isolation, not at executor return (seam 3 lifecycle invariant: a
crash mid-isolate leaves a reapable workspace, not a ghost bead). Runs `jj git
fetch` once per wave before creating workspaces.

### Step 3 — Dispatch executors (parallel, one per pick)

For each pick in the wave, dispatch a fresh `weft-executor` agent
(`${CLAUDE_PLUGIN_ROOT}/agents/executor.md`) into its isolated workspace. Executors run in
parallel up to `shed.max`; each is independent. The executor:

1. Reads the bead's description and acceptance criteria (`bd show <bead>`).
2. Applies TDD discipline (see `${CLAUDE_PLUGIN_ROOT}/references/tdd-verify-discipline.md`).
3. Calls `weft pick verify <bead>` internally — branches on `data.pass`.
4. Calls `weft pick seal <bead>` once `data.pass` is `true`, producing the
   sealed jj change with its `jj-change:<id>` spine label.

If the executor returns a `checkpoint` (blocking deviation — see
`${CLAUDE_PLUGIN_ROOT}/agents/executor.md` §Checkpoint protocol), the orchestrator holds
that pick out of the integration step and routes accordingly (human decision or
redo).

### Step 4 — Verify each sealed pick

```
weft pick verify <bead>
```

Run after the executor has sealed but before integration. Exit `0` in all cases
where the engine ran. Read `data.pass`:

- `true` — pick is ready for integration.
- `false` — executor is re-dispatched to fix and re-seal; this is an
  orchestrator-level retry, distinct from the executor's own internal verify
  loop.

Where the bead is flagged for deeper review (e.g., carries a `review:deep`
label), dispatch `weft-reviewer` (`${CLAUDE_PLUGIN_ROOT}/agents/reviewer.md`) into the
pick's workspace. The reviewer produces a `pick.verify`-shaped verdict; branch
on its `data.pass` identically.

### Step 5 — Integrate the wave

```
weft shed integrate <bead>...
```

Partitions the wave's sealed picks by file overlap and rebases each overlap
group as its own sub-stack rooted on `trunk()` — two picks that touch no common
file can never conflict, so independent groups are never stacked together.
Within a group, picks are ordered lexicographically by change-id. Emits:

```json
{
  "ok": true,
  "verb": "shed.integrate",
  "data": {
    "groups": [[{"bead": "...", "change": "..."}], ...],
    "conflicts": [{"bead": "...", "change": "..."}]
  }
}
```

`data.conflicts` is DATA emitted at exit 0 — it is not an error. A conflict
means the integration ran and detected a first-class jj conflict in the
resulting change; the orchestrator consumes `data.conflicts` and routes to
step 6 for each conflicted entry. Picks not in `data.conflicts` proceed
directly to step 7.

**Group semantics:** `weft shed integrate` builds a forest — each file-overlap
group is its own `trunk()`-rooted sub-stack. A first-class jj conflict is
therefore confined to the group whose picks share files: it can cascade to
changes stacked above it *within that group*, but never across groups.
`data.conflicts` may list in-group cascade-conflicted descendants in addition to
the picks that genuinely collided; picks in other groups are unaffected. The
integrate-time snapshot is not the final work list.

### Step 6 — Resolve conflicts

Work through `data.conflicts` iteratively, lowest-change-first. Do NOT resolve
every Step-5 entry blindly in one pass — re-query after each finalize, because
healing un-cascades: `conflict finalize` squashes the resolution into the
conflicted ancestor and jj conflict-simplifies its descendants, so one heal can
clear the cascade conflicts of everything above it. After each finalize,
consult `data.remaining_conflicts` (and/or `weft resume`'s `data.conflicts`)
for what is still conflicted, then resolve the next.

**Per-group fixpoint:** one `integrate` yields a forest with conflicts confined
to file-overlap groups, so there is no global stack order to manage and no need
to defer escalated picks to the end of one linear stack. Resolve each group's
conflict independently, lowest-change-first within the group: healing
un-cascades *within* the group —
`weft conflict finalize` squashes the resolution into the conflicted ancestor
and jj conflict-simplifies its in-group descendants, so resolving lowest-first
clears the cascades above it. An escalated change is never squashed, so it
leaves its own group's tail conflicted for a human; because groups are
independent, it cannot affect any other group's picks, which land normally. The
fixpoint is per-group and needs no global ordering. An escalated pick is parked
on `trunk()` by `finish open` (excluded from the collapsed line), not reordered;
`weft resume` surfaces it (and anything stacked above it in its group) as still
conflicted and unlanded.

**a. Open the conflict:**
```
weft conflict open <bead>
```
Creates a resolution workspace from the conflicted change. Emits the resolver
brief: which beads collided, on which paths, what each pick intended. The
conflict state is a first-class jj object (seam 4); no intermediate tracking
file is written.

**b. Dispatch the resolver:**
Dispatch `weft-resolver` (`${CLAUDE_PLUGIN_ROOT}/agents/resolver.md`) fresh into the
resolution workspace. The resolver edits conflict markers directly (diff style,
as pinned by `conflict open`), verifies the conflict count drops to zero via
`jj --no-pager st`, and returns a structured result. The resolver does NOT
commit; the engine squashes.

**c. Finalize the conflict:**
```
weft conflict finalize <bead>
```
Asserts only the resolution shows (via `jj diff --git`), squashes the resolution
into the conflicted ancestor, re-queries `conflicts()`, and reaps the resolution
workspace. Exits `0` with `data.{escalated, healed, remaining_conflicts}` —
result is DATA regardless of remaining conflicts. `data.escalated` is `true`
when the resolution is still conflicted (bead flagged `human`). Re-query
`data.remaining_conflicts` (or `weft resume`) to determine what to resolve next.

If `data.escalated` is `true` (resolver could not fully reconcile), the
orchestrator escalates by adding the `human` label to the bead:

```
bd update <bead> --add-label human
```

This is the escalation path. There is no separate escalation command. Beads
carrying the `human` label are blocked from `pick land` until a human resolves
or overrides.

### Step 7 — Land conflict-free picks

For each pick that is not in `remaining_conflicts`:

```
weft pick land <bead>
```

Asserts the pick's change is not in `conflicts()`, then closes the bead via
`bd close --suggest-next`. Landing marks the pick as complete in the warp.
Only conflict-free picks are landed; conflicted picks remain `in_progress` and
surface on the next `weft resume`.

### Step 8 — Clean up workspaces

```
weft shed cleanup <bead>...
```

Tears down the wave's isolated workspaces (per member: `jj workspace forget` +
`rm -rf`). Idempotent. Run after landing, covering all picks that reached
this step.

Then run:

```
weft reap [--epic <id>]
```

to collect any orphaned workspaces not covered by the manifest (crash-orphans,
interrupted resolution workspaces). `weft reap` is kind-aware: it recognises
resolution workspaces (the `-resolve` suffix) and reaps them by the same
liveness rule as execution workspaces.

### Step 9 — Project state; loop

```
weft resume --epic <id>
```

Projects the current warp state: which picks are sealed, in-flight, landed,
conflicted, or blocked. This is read-only; `resume` surfaces state, it does
not re-dispatch.

Return to **Step 1**. If `weft shed form` returns an empty wave (no picks
are ready), the loop terminates. The epic is fully woven when `weft resume`
shows all picks landed and none blocked.

---

## 5. Termination

The weave loop terminates naturally when `weft shed form --epic <id>` returns
an empty wave. This occurs when:

- all bead-level work is complete (all picks landed), or
- remaining beads are blocked on `human`-labelled predecessors or unresolved
  external dependencies.

There is no explicit finish verb in this loop. Epic-level finishing (PR
assembly, bookmark management) is a separate concern, outside the execute loop
and not invoked here. The execute loop terminates when the ready set is empty;
finishing is a distinct operator step available as `weft finish open` (to push
the epic's stack and open a GitHub PR) followed by `weft finish reconcile` after merge. The
execute workflow's responsibility ends when the ready set is empty.

---

## 6. Error handling

| Signal | Meaning | Action |
|--------|---------|--------|
| `weft shed form` returns empty wave | Ready set exhausted | Terminate loop |
| `weft pick verify` exits non-zero | Engine invocation failure | Abort; surface error to operator |
| `data.pass: false` (after retries) | Pick cannot pass gate | Checkpoint — surface to human |
| `shed integrate` exits non-zero | Integration engine failure | Abort; surface error; do not land any pick |
| `data.conflicts` non-empty (exit 0) | First-class jj conflicts detected | Route to step 6 (conflict resolution) |
| `data.escalated: true` | Resolution workspace still conflicted | Add `human` label; skip landing for that pick |
| `data.remaining_conflicts` non-empty | More conflicts remain in subtree | Continue iterative resolve loop — re-query and resolve the next; do NOT auto-escalate |
| Executor returns `checkpoint` | Blocking deviation | Hold pick; route per checkpoint kind |
| `shed isolate` partial failure | Some workspaces not created | Run `weft reap` to clean up; retry or redo affected picks |

---

## 7. Dropped GSD mechanics

The following GSD execute-phase mechanics are intentionally absent from this
workflow. They belong to GSD's git-based isolation model and have no equivalent
in the jj + beads substrate:

- **GSD's per-checkout git-isolation choreography** — replaced by `weft shed
  isolate` (jj workspaces, seam 3). The engine's workspace isolation guarantee
  subsumes the host-level git branching sequence that GSD required.
- **GSD's parallel commit safety flags and shared-state file locking** —
  unnecessary: jj workspaces are isolated by design; beads are the state store,
  not a shared mutable file.
- **Intra-wave file overlap detection forcing sequential execution** —
  dropped. jj's change graph makes this unnecessary; conflicts surface naturally
  at `shed integrate` as first-class objects (seam 4), resolved via the
  conflict open/finalize loop rather than by pre-emptively serialising the wave.
- **Completion signal fallback via presence of a generated artifact** — dropped.
  Weft uses `weft pick seal` + the `jj-change:<id>` spine label as the
  authoritative completion signal. No generated tracking artifact is written.
- **Post-wave build gate running the project's test suite** — dropped for this
  loop. Verification is per-pick (`weft pick verify`); cross-pick integration
  tests, if required, are bead-level acceptance criteria surfaced by the verify
  gate.
- **Shared tracking file updates after each wave** — dropped. The warp lives in
  beads; `weft resume` projects state from the authoritative bead + jj graph on
  demand. No wave-end batch write to a shared file is needed or safe under
  parallel execution.
