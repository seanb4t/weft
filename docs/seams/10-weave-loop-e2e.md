<!--
  ~ SPDX-License-Identifier: Apache-2.0
  ~ Copyright 2026 Weft Contributors
-->

# Seam 10 — Prove the weave loop end-to-end

> Status: **draft** (pre-design-reviewer). Sub-spec of
> [`docs/design.md`](../design.md) §9 (capstone — the loop the other seams
> compose). Tracked as bead `weft-w1y` (child of `weft-hjx`). The weave loop
> already exists as a draft: [`weft/workflows/execute.md`](../../weft/workflows/execute.md)
> (332 lines, ported in seam 5, `weft-hjx.5`) drives the verbs delivered by
> seams 1–4/6. This seam does **not** rewrite that workflow or the engine; it
> *proves the loop closes* against the real substrate and corrects the one place
> the draft has already drifted from the shipped verb surface (§5).

## 1. Scope

Prove that the full weave loop — `execute.md` Steps 1–9, the engine verbs it
calls, and the agents it dispatches — runs from a ready epic to a fully woven
epic, exercising **every branch**: clean land, intra-wave conflict + resolve,
verify-fail + retry, and unresolvable-conflict + human escalation.

The engine verbs are already unit-tested in isolation. What has never been
proven is that they *compose* into a terminating loop against real `jj`
workspaces and real `bd` state, and that the orchestration prompt drives them
correctly. This seam delivers that proof in two layers.

In scope:

- A **scripted CI gate** (§3): a `//go:build integration` Go test that builds
  the `weft` binary, stands up a scratch jj+bd repo with a synthetic
  branch-coverage fixture epic, and drives the Step 1–9 verb sequence as real
  subprocesses with the test playing orchestrator and a scripted
  executor/resolver. Asserts each branch closes and the epic terminates woven.
- A **synthetic fixture builder** (§4): a deterministic epic whose picks force
  each loop branch.
- A **CI-job extension** (§3, §8): the existing `integration` job installs only
  `bd` and runs only `./internal/plan/`; it must be extended to install `jj` and
  run `./internal/weave/` so the gate actually executes rather than skipping.
- A **one-time live dogfood** (§6): a cmux-driven worker running
  `/weft-execute` against the fixture, observed and gap-captured as beads.
- A **drift fix to `execute.md`** (§5): the workflow has two stale claims
  against today's engine — (a) §5/§6 say epic finishing is "not yet part of the
  stable verb surface," but `weft finish open/reconcile` (seam 6, `weft-hjx.9`)
  shipped; (b) the step-5 envelope example shows `conflicts` as a top-level
  field carrying `paths`/`lowest_ancestor`, but the engine emits
  `data.conflicts` with `{bead,change}`-only entries (`paths`/`lowest_ancestor`
  are deferred and do not exist — see `internal/cli/shed.go` and seam-4 §8).
  Correct both against the real verb surface.

Out of scope (explicitly deferred or rejected):

- **A re-runnable cmux dogfood harness** — deferred behind evidence. The
  one-time live run decides whether a maintained harness earns its keep (§6).
- **Turning the loop into a `weft weave` Go command** — rejected. It would move
  orchestration out of the prompt layer into the engine, violating the
  design's engine=primitive-verbs / orchestration=prompt split
  ([`design.md`](../design.md) §7). The test harness *mirrors* the orchestrator
  role; it does not become one in the engine.
- **Engine verb-surface changes** — none are planned. If the proof surfaces a
  genuine engine gap (see §7 risk), that becomes a findings bead, not silent
  scope creep here.

## 2. The two layers cover different things

The proof is split because the two hard-to-test surfaces fail differently:

| Layer | Orchestrator | Executor / resolver | jj + bd | Exercises `execute.md`? | Determinism |
|---|---|---|---|---|---|
| Scripted CI gate (§3) | the Go test | scripted (test code) | real | **No** | deterministic, CI-able |
| Live dogfood (§6) | real Claude running `execute.md` | real agents | real | **Yes** | non-deterministic, observed |

The scripted gate proves the **verbs compose and every branch closes**. The
live run proves **`execute.md` + the real agents navigate those same branches**.
Neither subsumes the other: a green scripted gate with a broken prompt still
fails the live run, and a passing live run is not a regression gate. There is
deliberately no redundancy.

## 3. The scripted CI gate

A single integration test, `internal/weave/weave_integration_test.go`, guarded
by `//go:build integration`. It reuses the substrate discipline of the in-repo
precedent [`internal/plan/integration_test.go`](../../internal/plan/integration_test.go)
— `t.TempDir()` for the scratch root, drive real subprocesses never in-process
fakes — and adds `exec.LookPath` + `t.Skip` for each of `jj`/`bd`/`weft` so a
missing binary skips rather than fails (the precedent test installs `bd` in CI
and does not itself skip; the skip guard is new here for local `go test ./...`
ergonomics).

**CI wiring is a required deliverable, not free.** The existing `integration`
job in `.github/workflows/ci.yml` installs only `bd` and runs only
`./internal/plan/` — as-is it would `t.Skip` this test on every run (zero
coverage). The seam must extend that job (or add a sibling) to: install `jj`,
ensure the `weft` binary is available to the test (the test `go build`s it into
its temp dir, so no separate build step is required, only a working Go +
toolchain), and widen the `-tags integration` test path to include
`./internal/weave/`. `.github/workflows/ci.yml` is therefore a file this seam
touches.

### 3.1 Why subprocess-binary E2E, not in-process verb calls

An honest end-to-end proof drives the **built `weft` binary** so the test
exercises cobra wiring, flag parsing, and the JSON envelope emission exactly as
the orchestrator prompt sees them. Driving verb functions in-process (behind a
fake `run.Runner`) would skip that layer and prove less. That fake — not
`run.Exec`, which is the real subprocess impl — remains the right tool for unit
tests; neither is used here.

### 3.2 The harness as orchestrator

The test plays the role `execute.md` plays at runtime — it does not invoke
`execute.md` itself. For each Step 1–9 action it shells out to the real verb and
branches on the JSON envelope (`data.pass`, `data.conflicts[]`,
`data.remaining_conflicts`) exactly as the workflow specifies. Note the engine
nests both the integration conflicts (`data.conflicts`, `shed integrate`) and
the post-resolution remainder (`data.remaining_conflicts`, `conflict finalize`)
under `data`, not at the envelope top level, and each `data.conflicts` entry is
`{bead,change}` only. The "scripted
executor/resolver" is harness code:

- **Executor stand-in:** after `weft shed isolate`, write deterministic files
  into the pick's workspace, then `weft pick seal <bead>`. For the verify-fail
  pick (§4, P3), first write a state that fails `weft pick verify`, observe
  `data.pass:false`, then write the passing state and re-seal.
- **Resolver stand-in:** when `weft shed integrate` reports a conflict (an entry
  in `data.conflicts[]`),
  run `weft conflict open <bead>`, edit the conflict markers in the resolution
  workspace directly (diff style, as `conflict open` pins), then `weft conflict
  finalize <bead>`. For the escalation pick (§4, P4), leave the conflict
  unresolved so `finalize` reports `remaining_conflicts` non-empty.

### 3.3 Binary acquisition

The test builds `weft` once (`go build` into the temp dir) or honors a
`WEFT_BIN` env override, and `exec.LookPath`s `jj` and `bd`. Any of the three
missing → `t.Skip` (matching the precedent), so the default `go test ./...` run
is unaffected and only the `integration` lane exercises it.

## 4. The synthetic fixture

A builder (`internal/weave/fixture_test.go` helper) creates a scratch jj+bd repo
and an epic with four picks and deterministic dependency edges so wave formation
and integration order are predictable. The live run (§6) needs the *same* epic
without the test harness, so the fixture is authored as a committed
`testdata/weave-fixture/warp-plan.json` (emitted via `weft plan emit`) that both
the Go helper and the live run consume; the helper wraps it with the scratch-repo
setup. (Pinning the fixture as a single committed artifact rather than two
divergent seed paths is a plan-time deliverable.)

| Pick | Branch exercised | How the fixture forces it |
|---|---|---|
| **P1** | clean land | self-contained pick; passes verify first try; no file overlap; lands directly in step 7 |
| **P2a + P2b** | intra-wave conflict → resolve | both edit the same region of one shared file; `shed integrate` stacks them and surfaces a first-class jj conflict; routed through the conflict open/finalize loop |
| **P3** | verify-fail → retry | scripted executor's first sealed state fails `weft pick verify` (`data.pass:false`); retry seals a passing state |
| **P4** | unresolvable → human escalation | collides with the wave to produce its **own** `data.conflicts[]` entry (distinct from the P2 pair); the resolver stand-in deliberately does not reconcile it, so `finalize` returns `remaining_conflicts`; harness asserts the `human` label is added and the pick is **not** landed |

The exact collision topology that gives P4 a conflict separate from the P2 pair
(a dedicated partner pick vs. a distinct file/region against the stacked tip) is
**a blocking first task for the plan**: the integration test cannot be written
until the topology is chosen and proven to deterministically yield two
independent conflicts — one the resolver heals (P2), one it leaves
(`remaining_conflicts`, P4). This is the same hard question as the §7 risk
(forcing a deterministic jj conflict from a test harness at all); the plan must
resolve it before the body of the test, and if the engine cannot be coaxed into
it, that becomes a findings bead per §7.

The fixture is the substrate for **both** layers: the scripted gate drives it
with stand-ins; the live run points `/weft-execute` at the same epic.

## 5. `execute.md` drift fix

Grounding found the workflow has already drifted from the shipped engine:

- §4 step (termination) and §5 (Termination) state epic finishing "is deferred
  engine work, not yet part of the stable verb surface this workflow restricts
  itself to." This was true at seam-5 authoring time; **seam 6 (`weft-hjx.9`)
  shipped `weft finish open/reconcile`.**

The fix corrects these passages to reflect that the finish verbs exist, and
clarifies the boundary: the execute loop still terminates at "ready set empty"
(finishing remains a separate operator step), but the workflow must no longer
claim the verb does not exist. This is the minimal, grounded correction — not a
rewrite of the workflow. A broader "keep the prompt in sync with the verb
surface" drift-guard is noted as a follow-up (§8), not built here.

## 6. The live dogfood (one-time, cmux-driven)

The live run is **agent-orchestrated, not manual**: a `claude` worker is
spawned in a detached cmux pane (the mechanism this project already uses to
drive `/drain` workers), handed `/weft-execute <fixture-epic>`, and observed via
screen-capture + idle-poll. Decisions surfaced by the loop (human-escalation,
checkpoints) are relayed to the operator; every gap, rough edge, or prompt
ambiguity the run exposes is filed as a bead under this seam's epic.

The run is **documented in this seam doc** (an appended "Live run N" record:
date, fixture, observed branches, findings beads) rather than checked in as
automation. Whether to promote the cmux-drive logic into a committed,
re-runnable harness is **deferred until after the first run** — the run itself
is the evidence for that decision.

## 7. What the gate asserts

| Assertion | Branch / invariant |
|---|---|
| non-zero verb exit → test fails loudly | engine-invocation failure is never swallowed |
| `data.pass:false` then `true` after re-seal | P3 retry path |
| `data.conflicts[]` at exit 0 routed to resolve, heals | P2 conflict path (DATA, not error) |
| `data.remaining_conflicts` → `human` label added, pick not landed | P4 escalation path |
| clean picks closed via `pick land` | P1 land path |
| `shed cleanup` + `reap` idempotent; no orphan workspaces remain | workspace lifecycle |
| `shed form` returns empty wave; `resume` shows all landed-or-blocked | terminating loop / terminal state |
| every emitted envelope is well-formed `{ok,verb,data,...}` | the contract the prompt branches on |

## 8. Testing

- The seam's deliverable **is** a test, so "testing" is the integration test
  itself (§3) plus a fast non-integration unit test for the fixture builder
  (assert the epic + dep edges + pick labels are seeded as intended, so a
  fixture regression is caught without the full E2E run).
- CI: the integration test runs only under `-tags integration`; `t.Skip` when
  `jj`/`bd`/`weft` are unavailable, so the default `go test ./...` is
  unaffected. The existing `integration` job must be extended to install `jj`
  and widen its `-tags integration` path to `./internal/weave/` (§3) — without
  that, the gate skips on every CI run and provides no coverage. A passing CI
  run that actually *executed* (not skipped) the weave test is the acceptance
  signal for this deliverable.
- The live dogfood is not a CI gate; it is a recorded validation (§6).

## 9. Open sub-seams / follow-ups

- **Re-runnable cmux dogfood harness** — promote the one-time drive logic into a
  committed manual/nightly harness, *if* the first live run earns it (§6).
- **Prompt↔verb drift-guard** — a general mechanism (test or CI check) ensuring
  `execute.md` (and the agents) cannot silently fall behind the verb surface
  again, beyond the point fix in §5.
- Any engine gap the proof surfaces (§7 risk) → its own findings bead.

## 10. Decisions (ADR candidates)

- **D1 — Two-layer proof (scripted gate + live dogfood), not one.** The verb
  choreography and the prompt-driven orchestration fail differently; a single
  layer cannot cover both. The scripted gate is the regression artifact; the
  live run is the prompt proof.
- **D2 — Subprocess-binary E2E, not in-process verb driver.** End-to-end means
  the built binary against real jj+bd, exercising cobra + envelope emission as
  the prompt sees them. (Rejected: in-process `Runner`-fake driver — proves
  less; reserved for unit tests.)
- **D3 — Orchestration stays in the prompt; the test only mirrors it.**
  Rejected turning the loop into a `weft weave` Go verb — it would violate the
  engine=primitives / orchestration=prompt split (`design.md` §7).
- **D4 — One synthetic branch-coverage fixture for both layers**, rather than
  dogfooding a real backlog epic — guarantees the conflict/retry/escalation
  branches are actually hit, which an authentic epic exercises only
  incidentally.
- **D5 — Re-runnable harness deferred behind evidence**, not built upfront —
  YAGNI until the first live run shows it repays the maintenance.

## Risk / open question

The fixture must **deterministically** force a first-class jj conflict through
`weft shed integrate` from a scratch setup. Grounding confirms the engine emits
conflicts as `data.conflicts[]` DATA at exit 0 (seam 4), but the fixture must reliably
produce same-region edits that `shed integrate`'s rebase turns into a recorded
conflict. If the engine cannot be coaxed into a reproducible conflict from a
test harness, that is itself a finding — and may justify a small test-only
engine affordance (e.g. a documented way to seed a conflicting change), captured
as a follow-up bead rather than forced into this seam.
<!-- adr-capture: sha256=0198076539baabe0; session=cli; ts=2026-06-08T16:12:49Z; adrs=weft-9w5 -->
