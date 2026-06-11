<!--
  ~ SPDX-License-Identifier: Apache-2.0
  ~ Copyright 2026 Weft Contributors
-->

# Phased planner + JIT per-phase planning (ccy.3, Phase C) — design

**Date:** 2026-06-10
**Bead:** `weft-ccy.3` (child of epic `weft-ccy`)
**Status:** Approved by Sean (brainstorming session); pending design-review gate
**Refines:** `docs/superpowers/specs/2026-06-09-layer-a-interactive-phased-loop-design.md` (master spec §1, §2, §8 Phase C)
**Depends on:** `weft-ccy.5` (engine enablers — DONE)

## Context

The master spec (2026-06-09) restores the GSD Layer-A interactive/phased loop
additively. Phase B (`weft-ccy.5`) shipped the engine enablers; this spec covers
**Phase C**: the prompt-layer changes that make the planner *always decompose
into phases*, emit a roadmap when the work is genuinely multi-phase, and plan
each phase's picks just-in-time.

### What Phase B already shipped (the machinery this design drives)

Grounded by reading `internal/plan/{plan,emit}.go` and `internal/cli/plan.go`:

- **Two emission shapes already exist in `GraphJSON`.** When `WarpPlan.Phases`
  is non-empty, emit produces *project epic → phase sub-epics → inter-phase
  `blocks` edges, **zero picks*** (the roadmap). When `Picks` is non-empty, emit
  produces today's single-epic shape. `Validate` enforces "a plan carries phases
  or picks, not both."
- **`plan emit` echoes created ids.** The create-path success envelope carries
  `data.ids` (`{node-key → bead-id}`), so a skill reads the project-epic id and
  every phase-sub-epic id straight from the emit output — no query-around.
- **`plan emit --epic <id>` is the re-plan upsert path**, and Phase B completed
  the §8 sub-seam: it applies new-pick `blocks` edges and wires parent-child
  links post-import (positional `ids` from `bd import --json`). Per-phase pick
  emission *is* this path targeted at a phase sub-epic — **no new emission mode**.

**Consequence: ccy.3 is purely prompt-layer.** No Go changes. The risk surface
is prompt design, not engine code. The bead's original "prompt-layer only" claim
is now accurate (Phase B paid the engine cost).

### Grounding traces (recorded as `bd note` on `weft-ccy.3`)

- `bd ready` respects epic-blocking **transitively**: children of a phase epic
  that is itself blocked (via an inter-phase `blocks` edge) are excluded from
  `bd ready` until the blocker closes. Inter-phase edges therefore gate
  execution phase-by-phase with no per-pick edges required (empirically verified
  2026-06-10).
- `bd import` ignores the `parent` field; weft wires parentage post-import via
  `bd dep add --type parent-child` using positional ids. The re-plan `--epic`
  path already does this.
- context7 has no entry for `bd` (niche tool); grounding is via codebase probe +
  empirical bead memories rather than upstream docs (degraded mode, noted for
  the reviewer).

## Decisions made in this session

| Decision | Choice |
|---|---|
| Per-phase planning unit | **Standalone `plan-phase` skill** — the composable unit the Phase-E driver later reuses; also satisfies the bead's `phases → picks` acceptance within ccy.3. |
| Cross-session context carry | **Per-phase sub-epic beads** — each phase's slice of the research digest + requirement IDs is folded into that phase sub-epic's description at roadmap emit; `plan-phase` reads it back via `bd show <phase>`. |
| Phase-discovery rule | **Crisp ship-and-reshape test, no bias; residual ambiguity → fewer phases; human approval gate as backstop** (see §2). |

## 1. Shape & scope

Three prompt artifacts, all driving Phase-B machinery:

| Artifact | Change |
|---|---|
| `plugin/agents/planner.md` (`weft-planner`) | Add phase-discovery (§2) and **two output shapes** (§3): roadmap (`phases[]`) when multi-phase, picks when single-phase. Add the phase context-carry rule. |
| `plugin/skills/new-project/SKILL.md` | Phase 4–7 branch on roadmap vs single-epic; the closing message diverges for multi-phase (§4). |
| `plugin/skills/plan-phase/SKILL.md` (**new**) | JIT per-phase pick emission scoped to one phase sub-epic (§5). |

## 2. Phase-discovery heuristic

The planner already runs Goal-Backward decomposition into dependency-ordered
**waves** within one epic (dogfood-proven: 22 picks / 42 edges / 7 waves, correct
`bd ready` gating). Phase discovery is a layer on top of that, not a replacement.

**The test (no bias either way — phasing is a property of the work):**

> A phase boundary exists exactly where you would genuinely want to **ship and
> get human feedback before committing to the next chunk's HOW.**

That ship-and-reshape inflection is the *only* thing the per-phase discuss/verify
gate buys. If the whole project's picks can be planned confidently up front, it
is **single-phase** — the wave decomposition already supplies dependency
ordering **without** gates. Phases add *gates*, not *scheduling*.

- **Test passes (2+ genuine ship-and-reshape inflections)** → emit a **roadmap**:
  one phase per cluster of waves, inter-cluster dependencies become phase `needs`
  edges. No picks.
- **Test fails (one cohesive milestone)** → emit **single-epic picks** — today's
  exact shape. This is the **degenerate case, not a mode**: the one-shot flow
  survives behaviorally.

**Residual ambiguity resolves to fewer phases**, because the failure modes are
asymmetric:

- *Under-phased* (one big epic): loses a mid-point checkpoint, but the work still
  ships and is verified at the end. Cheap, recoverable.
- *Over-phased*: each phase sub-epic is one PR with its own finish cycle —
  manufactured PR boundaries and forced gates that are structurally annoying to
  unwind. Harder to undo.

**The human approval gate is the backstop, not the planner's thumb.**
new-project Phase 6 shows the roadmap as a `--dry-run` before anything
materializes; the human sees "3-phase roadmap" and can collapse it. This is a
*safety net* for the rare residual case, deliberately not the primary mechanism:
if the planner routinely over-phased and leaned on the human to collapse, the
gate would turn adversarial and erode trust.

**Coverage of the phased path does not depend on the planner over-triggering.**
The worry "the feature won't fire / won't get tested" is addressed by the
**phase-F dogfood validation gate** (master spec §8) — one deliberately
multi-phase project proves the path. Normal-use phasing stays honest.

## 3. weft-planner — roadmap output shape

The agent gains a decision step (run §2 after wave reasoning) and a second output
shape. When multi-phase, each `phases[]` entry carries:

- `ref`, `title` — identity + name.
- `description` — **the phase mini-brief**: the phase goal, *that phase's slice
  of the research digest*, and its requirement IDs. This is the context-carry
  mechanism (Decision B): it persists into the phase sub-epic bead so a
  later-session `plan-phase` consumes it via `bd show <phase>`. Beads is the
  brain — there is no CONTEXT.md.
- `acceptance` — phase success criteria. Becomes the `verify-work` (ccy.2)
  fallback target (master spec §4 chain: phase acceptance → picks' acceptance →
  phase goal).
- `needs` — inter-phase dependencies → `blocks` edges → transitive `bd ready`
  gating.

The pick-authoring methodology (Goal-Backward, vertical-slice, file-ownership,
TDD heuristics) is **unchanged** — it is *reused* at `plan-phase` time against a
single phase. The roadmap shape authors phase-level briefs and acceptance only;
it never authors picks.

The agent's existing quality gate and validation chain (`plan check`,
`emit --dry-run`) apply to the roadmap shape unchanged — both shapes validate and
preview through the same verbs.

## 4. new-project — branch + closing message

Phases 1–3 (adaptive Q&A, parallel research, requirement extraction) are
**unchanged**. Phase 4 dispatches `weft-planner`, which now returns a roadmap
**or** an epic+picks plan. Phases 5–6 (`plan check`, `emit --dry-run` human gate)
are **unchanged** — both shapes flow through the same verbs.

Phase 7 emits; the **closing message diverges** on the shape (read from
`data.ids` in the emit envelope):

- **single-phase:** "warp emitted — run `execute`." *(today's message, verbatim)*
- **multi-phase:** "roadmap emitted — run `discuss` on phase 1, then
  `plan-phase`." The message names the phase-1 sub-epic id.

This is the deliberate "more interactive by default" change the epic exists for
(master spec §1).

## 5. plan-phase skill (new)

`plugin/skills/plan-phase/SKILL.md`. Mirrors new-project Phases 4–7 scoped to a
**single phase sub-epic**. Argument: a phase epic id.

1. **Load phase context.** `bd show <phase-id>` (the mini-brief from §3) +
   the phase epic's `design` field/notes (the HOW decisions `discuss` (ccy.1)
   persisted). Beads is the brain; no sidecar files. An **empty or absent
   `design` field is a valid input**, not an error — if `discuss` was skipped,
   `plan-phase` plans from the phase mini-brief alone.
2. **Dispatch `weft-planner` picks-only**, targeting that phase's acceptance,
   consuming the loaded context. Output: a picks-only `warp-plan.json`.
3. **Validate + gate.** `plan check` → `plan emit picks.json --epic <phase-id>
   --dry-run` (human approval gate) → on approval, `plan emit picks.json
   --epic <phase-id>`.
4. **Result.** Picks land parented to the phase epic. First run on an empty phase
   is the re-plan **all-creates** path (no existing `weft-ref` children to match);
   a revision re-run upserts. Closing message: "phase planned — run
   `execute --epic <phase-id>`."

The full per-phase rhythm (`discuss → plan-phase → execute --epic <phase> →
verify-work → finish`) is then a composition of these standalone units. The
Phase-E driver sequences them; ccy.3 only delivers the units.

### Sequencing note

Inter-phase `blocks` edges gate **execution** (`bd ready`), not planning. A phase
epic closes when its picks close (the `finishing-a-development-branch` backstop),
releasing the next phase. The JIT rhythm plans phase N's picks after phase N's
`discuss`, which naturally follows phase N-1 shipping — so phases are planned in
order, though nothing structurally forbids planning ahead.

## 6. Testing & validation

Prompt-layer only:

- **Plugin gates:** `claude plugin validate` strict (both manifest paths) +
  intra-tree path-citation rewrite grep, per existing skill discipline.
- **Manual dogfood (the real coverage):** a deliberately multi-phase request
  driven through `new-project` → roadmap emit (verify project epic + phase
  sub-epics + inter-phase edges land; `bd ready` returns only phase-1-eligible
  work) → `plan-phase` on phase 1 (verify picks land parented to the phase epic).
  A request passes the §2 ship-and-reshape test when it has genuine
  reshape-before-next-chunk inflections — e.g. *"build a CLI tool that (1) ships
  a working read-only query path, then (2) adds mutation/write support, then
  (3) adds a remote-sync mode"*: each stage is independently shippable and you'd
  want stage 1 in users' hands before fixing stage 2's HOW. The phase-F bead's
  acceptance criteria should pin a concrete stimulus of this shape.
- **Degenerate path:** covered by the existing single-phase `new-project` flow
  continuing to work unchanged (one cohesive request → epic + picks → "run
  `execute`").

No Go unit tests are added (no Go changes); Phase B's emit/replan tests already
cover the machinery.

## 7. ADR impact

The **`weft-cfp` addendum is already filed** (2026-06-09): it supersedes the
opt-in-`--phased`-mode consequence with auto-discovered phasing + the
single-phase degenerate case. No further ADR is needed for that decision.

One planner-policy ADR remains to capture post-plan via `/capture-adrs` (Phase B
filed `weft-4hq`/`weft-0pf` for the *engine* enablers):

- **JIT per-phase planning via re-plan `--epic`** — per-phase picks are emitted
  just-in-time through the existing upsert path, with each phase sub-epic bead
  carrying its own planning context (the ship-and-reshape phase-discovery test
  and the per-phase context-carry mechanism are the recorded decisions).

## Out of scope

- The Phase-E **driver** skill (sequences the rhythm) — ccy.3 delivers the
  composable units only.
- `feature` / `onboard` skills (Phase D).
- Any engine change — Phase B is complete; ccy.3 adds none.
- Autonomous multi-phase weave without human gates (driver auto-mode, Phase E).
