<!--
  ~ SPDX-License-Identifier: Apache-2.0
  ~ Copyright 2026 Weft Contributors
-->

# Unattended-trust hardening (roadmap §3) — design

**Date:** 2026-07-04
**Bead:** `weft-x38` (design bead; promoted to the milestone epic at plan-to-beads)
**Status:** Approved by Sean (brainstorming session); pending design-review gate
**Grounded in:** roadmap §3/§5/§9 (decisions #1, #4 confirmed 2026-07-04), seam §8
deferred scope (seams 02, 03, 04, 11; seam 07 §11), and the shipped `internal/cli`
code — traces recorded as `bd note` lines on `weft-x38`.

## Context

The 2026-07-03 deep review found the engine complete against its spec with one
structural gap: **per-epic projection exists (`resume`), whole-warp health does
not.** Three finished-or-nearly-finished picks sat stranded in workspaces for
11 days — invisible to `bd ready` (correctly: `in_progress` isn't ready),
invisible to `bd blocked` (nothing blocks them), visible only by manually
joining bead state against jj workspace state. Roadmap §3 names the move: from
*choreography that works when watched* to *choreography that reports when
unwatched*. Roadmap §9 #1 confirms this as the next milestone; §9 #4 confirms
materializing it into the warp now.

Five components, all currently prose-only (zero beads track them):

1. **`weft doctor`** — the whole-warp join (generalizes seam 7 §11's deferred
   install-health-check into bead × workspace × change × PR).
2. **`executor_live` liveness** (seam 3 §5/§10) — `reap.go:53-61` explicitly
   defers the guard; today every `in_progress` bead's workspace is kept, so a
   crashed executor over-retains forever.
3. **Resolver oscillation guard** (seam 4 §8) — `conflict open`/`finalize` are
   stateless; nothing bounds resolve attempts on an unresolvable merge.
4. **Replan removed-pick supersede** (seam 2 §7/§8) — `plan.go:234-236`
   computes and reports `removed[]` but never enacts it.
5. **`finish reconcile` re-verification** (seam 11 §7) — the merge-commit
   branch (`jj rebase -b @ -o main --skip-emptied`, `finish.go:488`) is flagged
   higher-risk: `-b @` may drag a parked escalated sibling onto main.

The primitive vocabulary (bead↔change spine, overlap forest, conflict-as-data)
is done and does not change.

## Decisions (Sean, 2026-07-04 brainstorming)

1. **One epic** for the whole milestone. Cost accepted: parked P3 picks hold
   the epic open (release-cadence trigger) until they land.
2. **Doctor = report + propose.** Read-only join; each finding carries a
   category and a suggested recovery command. Doctor never mutates — recovery
   stays owned by the existing verbs (`reap`, `conflict open`,
   `finish reconcile`, `bd close`).
3. **`executor_live` = inference, no new signal.** Recency over existing state
   (jj op log + workspace mtimes) against a configurable threshold. No PID
   file, no heartbeat: the engine never spawns agents, so it cannot own a PID
   natively, and a prompt-written PID protocol is exactly the
   works-when-watched fragility this milestone exists to remove.
4. **Priorities:** doctor P2, executor_live P2, finish-reconcile re-verify P2;
   oscillation guard P3, replan supersede P3. Nothing at P4 — all five are the
   declared milestone.

## New invariants (the milestone's contract)

- **I1 — Doctor never mutates.** Its verb contract is diagnosis; exit 0 even
  with findings (diagnosis is not failure). Mutation lives in the verbs it
  suggests.
- **I2 — A pick with woven or landed work can never be silently dropped from a
  plan.** Replan hard-fails (exit 2) if a removed pick is `in_progress` or
  `closed`.
- **I3 — Reap distinguishes dead from busy.** `in_progress` + live → skip;
  `in_progress` + not-live → crashed → reap (the seam 3 §5 table, at last
  fully implemented).
- **I4 — Conflict resolution terminates.** Attempts are bounded; the bound's
  exhaustion is a forced escalation, not a loop.

## Components

### 1. `internal/liveness` (P2)

A small package answering one question: *when was this workspace last worked?*

- **Signals (existing state only):** the committer timestamp of the
  workspace's working-copy commit (`jj log -r '<name>@' -T
  'committer.timestamp()'` — jj refreshes it on every per-workspace snapshot,
  i.e. every jj command the executor runs there), joined with a max-mtime walk
  of the workspace directory (ignoring `.jj/`), which guards the
  edited-files-but-ran-no-jj-command window. Empirically validated 2026-07-04
  in this repo: an idle `worktree-agent-*` workspace's `@` timestamp was 12
  days old while an active workspace's was minutes old. The op log is NOT
  used: jj 0.43's op templates expose no workspace attribution (verified —
  no `path`/workspace keyword; only `args` text).
- **API shape:** `LastActivity(runner, root, wsName, wsPath) (time.Time, error)`
  plus `Live(t time.Time, threshold time.Duration) bool`. Callers own policy.
- **Config:** `[liveness] threshold = "45m"` in `.weft/config.toml` (a
  `time.ParseDuration` string; exact key naming settled at plan time).
  Conservative default: a thinking-but-quiet executor can look dead; the cost
  is bounded because `reap` runs at orchestrator startup / `resume` (seam 3
  §5.1), not mid-wave, and jj snapshots mean a sealed pick survives reaping
  (seam 3 §2 "reaping is always safe").

### 2. `weft doctor` (P2; needs liveness)

New read-only verb: `weft doctor [--epic E] [--json]`. The whole-warp join:

- **bd side:** all `in_progress` beads (global `bd list --status in_progress`,
  verified available), and closed/missing beads reached from the jj side.
- **jj side:** `jj workspace list` resolved kind-aware via `workspace.Resolve`
  (exactly reap's join key, including `-resolve` handling); change state per
  `jj-change:<id>` label — exists, conflicted (`conflicts() &` scoped, the
  `resume.go` pattern with the same injection guard), or already in `trunk()`.
- **gh side (best-effort):** PR state per epic bookmark. Any gh failure
  degrades to a warning in the envelope and never aborts the join — the
  `deleteRemoteBranch` posture (`finish.go:403`). Doctor must be fully useful
  offline.

**Finding categories** (each finding: `{category, reason, bead, workspace,
change, evidence, suggest}` in the uniform envelope):

| Category | Meaning | Suggests |
|---|---|---|
| `stray` | `in_progress` bead + workspace, last activity beyond threshold; or sealed change already in `trunk()` but bead never closed | `weft reap` / `bd close` |
| `orphan` | workspace whose bead is closed or missing | `weft reap` |
| `lost` | `in_progress` bead with no workspace | `weft pick redo` / human |
| `conflicted` | sealed change in `conflicts()` | `weft conflict open <bead>` |
| `unreconciled` | PR merged but local bookmark/epic state still present | `weft finish reconcile <epic>` |
| `foreign` | workspace resolvable to no bead (e.g. `worktree-agent-*` leftovers) | manual sweep |

The `foreign` category answers state.md's open question directly: doctor
*flags* foreign workspaces; `reap` keeps its bead-linked remit and never
touches them.

`stray` deliberately groups two root causes with the same recovery surface —
a probable crash (`in_progress` + workspace past the liveness threshold) and
an incomplete `pick land`/`shed cleanup` (sealed change already in `trunk()`,
bead never closed). So that consumers can branch without parsing prose, each
finding carries a machine-readable `reason` field (`stale-activity` |
`landed-unclosed` for stray; one value per category elsewhere) alongside the
human-oriented `evidence`.

Text output is a one-line-per-finding summary; exit 0 regardless of findings
(I1). Consumers (SessionStart hook, a future skill) branch on the envelope.

### 3. Reap `executor_live` wiring (P2; needs liveness)

Replace `reap.go`'s blanket `in_progress` skip with the seam 3 §5 decision
table: `in_progress` + live → skip; `in_progress` + not-live → crashed → reap.
Everything else keeps today's fail-safe behavior (`beadStatus`'s
infrastructure-vs-missing distinction is untouched). Reap also gains
`--dry-run` for parity with the trust posture — report what would be reaped
without forgetting anything.

This pick also closes a latent hazard the design round surfaced in the code:
today a *foreign* workspace (name resolving to a bead that does not exist —
e.g. a Claude Code `worktree-agent-*` workspace) falls through `beadStatus`'s
missing-bead path and is **forgotten** by reap, breaking whatever session owns
it (its directory survives only because the `wtRoot/name` join points
nowhere). The guard: a missing bead is an orphan **only if** its directory
exists under the worktrees root (weft puts every workspace it creates there);
missing bead + no directory under `wtRoot` → foreign → skip and leave for
doctor to report. This makes the spec's "reap keeps its bead-linked remit"
claim true rather than assumed.

### 4. `finish reconcile` re-verification (P2; independent)

Seam 11 §7's two risks are *partially* retired already —
`internal/weave/finish_topology_test.go` (landed 2026-06-09, bead
`weft-8ou.7`) proves the merge-commit `jj rebase -b @ -o main --skip-emptied`
leaves a **clean** parked sibling untouched
(`TestReconcileMergeBranchLeavesParkedSiblingUntouched`, which replays the
literal jj command) and that real `weft finish open`'s `-r` collapse leaves an
escalated trunk-sibling's parent unchanged (`TestFinishOpenCollapseTopology`,
which uses deliberately non-conflicting picks). This pick owns the remaining
delta, not a re-derivation:

- The parked escalated sibling **actually conflicted** (jj may select or
  materialize a conflicted commit differently under rebase/`--skip-emptied`
  than a clean one — the untested half of the seam 11 §7 claim).
- Driving the **real `weft finish reconcile` verb** end-to-end (merge-style
  detection, fetch, rebase, bookmark/remote-branch handling; gh mocked) rather
  than replaying the raw jj command.

Deliverable: integration tests covering that delta, plus whatever scoping fix
they force. Verification-first: the tests are the point; a code change is
conditional on what they reveal.

### 5. Resolver oscillation guard (P3; independent)

- **State:** attempt counter as a bead label `resolve-attempts:<n>` — the
  `jj-change:<id>` precedent: crash-durable, machine-readable, visible in
  `bd show`. Increment = remove old + add new (labels are set-valued).
- **Mechanics:** `conflict open` increments the counter; at
  `[conflict] max_resolve_attempts` (default 3) it refuses to open a
  workspace, adds the `human` label, and emits `escalated: true` — the same
  escalation shape `finalize` already emits (`conflict.go:162-174`).
  A successful `finalize` (healed, not escalated) clears the counter.
- The cap is a forced-escalation bound (I4), not a retry budget: the
  orchestrator's resolve loop terminates even when an agent thrashes on an
  unresolvable merge.

### 6. Replan removed-pick supersede (P3; independent)

Enact what `planReplan` today only reports:

- Removed pick **open** → `bd close` with a note "removed by replan of
  `<epic>` (was `weft-ref:<ref>`)". Audit trail preserved; never deleted.
  `bd supersede` requires `--with <new>` (verified) and a replan diff cannot
  identify a successor ref, so close-with-note is the honest primitive. When a
  future authoring surface can express an explicit successor mapping,
  `bd supersede` slots in without changing this contract.
- Removed pick **`in_progress` or `closed`** → hard error, exit 2 (I2). The
  envelope names the pick and why. A plan that drops woven or landed work is
  structurally wrong and must be investigated, not applied.
- Dry-run reports the same classification (`removed`, `removed_blocked`)
  without mutating.

### 7. Milestone exit test (P2; needs doctor + reap wiring)

Roadmap §7.4's exit test as an integration-gated E2E in the seam-10 style
(pinned `bd`+`jj` in CI): simulate a killed executor mid-pick (in_progress
bead + workspace with backdated activity) and a stranded workspace (closed
bead + lingering workspace); assert one `weft doctor` run surfaces both with
the right categories and suggestions, and `weft reap` collects exactly the
crashed one while leaving live work alone.

## Warp shape (what plan-to-beads materializes)

Seven picks under one epic (promoted from `weft-x38`):

| # | Pick | P | Blocks on |
|---|---|---|---|
| 1 | `internal/liveness` recency signal + config | P2 | — |
| 2 | `weft doctor` whole-warp join | P2 | 1 |
| 3 | reap `executor_live` wiring + `--dry-run` | P2 | 1 |
| 4 | `finish reconcile` forest re-verification | P2 | — |
| 5 | resolver oscillation guard | P3 | — |
| 6 | replan removed-pick supersede + I2 guard | P3 | — |
| 7 | milestone exit test (E2E) | P2 | 2, 3 |

Wave shape: {1, 4, 5, 6} → {2, 3} → {7}.

Rejected alternative: five coarse picks (liveness folded into reap, exit test
folded into doctor). Simpler on paper, but it serializes doctor behind reap's
internals and buries the milestone's acceptance test inside one component.

## Error handling & testing conventions

- All new **bd and jj** subprocess calls follow the established fail-safe
  pattern: infrastructure anomalies are hard errors (exit 2), never silently
  treated as absence (`beadStatus` is the reference; doctor reuses it).
  **gh is the documented exception, in doctor only:** its PR join is
  best-effort (Component 2) per the shipped `deleteRemoteBranch` precedent
  (`finish.go:403`) — a gh failure degrades to an envelope warning so doctor
  stays fully useful offline. Everywhere else (e.g. `finish reconcile`'s
  `prState`), gh keeps its existing hard-fail contract.
- Every revset interpolation is allowlist-validated (`changeIDPattern` /
  `workspaceRevPattern` / `epicIDPattern` — existing guards).
- Unit tests via the injectable Runner (mocked subprocesses) for every verb
  change; integration tests (build-tagged, seam-10 style) for doctor, reap
  liveness, finish re-verification, and the exit test.
- Envelope discipline: new keys are `[]`-initialized, both branches of every
  verb emit the same key set (seam 9 guard).

## Out of scope

- Implementing the picks (they are the epic's children; this bead delivers the
  reviewed plan + materialized warp).
- Self-dogfood meta-loop (roadmap §2 exit 1); fovea onboard (exit 2).
- Pulled-by-need infra: golangci-lint, multi-arch/signing, config-schema
  refinements, non-Claude hosts.
- Doctor `--fix` (enactment) — revisit only if report+propose proves too thin
  in dogfood, mirroring seam 7 §11's posture on `install`.
- Any change to the primitive vocabulary (spine, overlap forest,
  conflict-as-data).

## ADR candidates

- **Liveness is inferred, never declared** (no PID/heartbeat protocol; the
  engine trusts only state it can observe).
- **Doctor reports, verbs recover** (report+propose contract; I1).
- **Replan cannot drop woven work** (I2 — supersede policy for removed picks).
