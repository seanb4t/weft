<!--
  ~ SPDX-License-Identifier: Apache-2.0
  ~ Copyright 2026 Weft Contributors
-->

# Phased Planner + JIT Per-Phase Planning Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Teach the `weft-planner` agent to always decompose into phases — emitting a roadmap (`phases[]`) for genuinely multi-phase work and today's single-epic pick plan otherwise — and add a `plan-phase` skill that plans each phase's picks just-in-time into its sub-epic.

**Architecture:** Purely prompt-layer. All three artifacts are markdown prompts driving Phase-B engine machinery that already ships (`internal/plan/{plan,emit}.go`, `internal/cli/plan.go`): the two-shape `GraphJSON` branch, `data.ids` echo on emit, and the `plan emit --epic <id>` re-plan upsert path. No Go changes.

**Tech Stack:** Claude Code plugin skills/agents (markdown + YAML frontmatter); `weft` CLI verbs (`plan check`, `plan emit [--dry-run] [--epic]`); `bd` (beads) for the warp; jj for VCS.

**Spec:** `docs/superpowers/specs/2026-06-10-phased-planner-jit-design.md`

**Design bead:** `weft-ccy.3`

---

## Grounding notes (verified against the repo)

- **Files touched all exist** except the one new-create: `plugin/agents/planner.md` ✓, `plugin/skills/new-project/SKILL.md` ✓, `plugin/skills/plan-phase/` is new (parent `plugin/skills/` exists; `discuss/`, `execute/`, `new-project/`, `verify-work/` are siblings).
- **No automated unit tests apply** — these are markdown prompts. The verification gate for every task is the CI's own discipline run locally: `claude plugin validate ./plugin --strict`, `claude plugin validate . --strict`, and the grep-discipline guard `grep -RnE 'weft/(agents|references|workflows)/' plugin/` returning **no matches** (use `${CLAUDE_PLUGIN_ROOT}` for any intra-plugin path reference).
- **Skills are auto-discovered.** The plugin manifest (`plugin/.claude-plugin/plugin.json`) does NOT enumerate skills, so creating `plugin/skills/plan-phase/SKILL.md` is sufficient to register it — no manifest edit.
- **Skill frontmatter contract** (from `plugin/skills/discuss/SKILL.md`): YAML block with `description:` + `argument-hint:`, then the SPDX header comment, then an `<!-- adapted from … -->` attribution comment when porting from GSD.
- **Engine contracts the prompts rely on** (do not re-implement): roadmap emit produces project-epic + phase sub-epics + inter-phase `blocks` edges with zero picks; `plan emit` create-path success envelope carries `data.ids` (`{node-key → bead-id}`); `plan emit <file> --epic <phase-id>` upserts picks into a phase sub-epic, wiring parent-child + new-pick `blocks` edges post-import.

---

## File Structure

| File | Responsibility | Action |
|---|---|---|
| `plugin/agents/planner.md` | The planning agent. Gains phase-discovery + the roadmap output shape; pick methodology unchanged (reused per-phase). | Modify |
| `plugin/skills/new-project/SKILL.md` | Greenfield entry. Phase 4 consumes either output shape; Phase 7 closing message diverges on shape. | Modify |
| `plugin/skills/plan-phase/SKILL.md` | New skill: JIT per-phase pick emission scoped to one phase sub-epic. | Create |

---

## Task 1: weft-planner — phase discovery + roadmap output shape

**Files:**

- Modify: `plugin/agents/planner.md` (Output contract section ~lines 30–69; Methodology section ~lines 71–84; Quality gate ~lines 186–203)

- [ ] **Step 1: Reframe the Output contract as two shapes**

Replace the current "## Output contract" intro sentence and the single JSON block (planner.md lines ~32–54, ending before "Field rules") with the two-shape framing. The existing pick JSON block and field rules stay; insert the roadmap shape ahead of them.

Replace:

```markdown
## Output contract

Your deliverable is a single `warp-plan.json` file conforming to seam 2 §3:
```

with:

````markdown
## Output contract

Your deliverable is a single `warp-plan.json` file conforming to seam 2 §3. It
takes **one of two shapes**, chosen by the phase-discovery test (§ Methodology 0).
A plan carries **`phases` or `picks`, never both** — `weft plan check` rejects a
plan that mixes them.

### Shape A — roadmap (multi-phase)

Emitted when the work has 2+ genuine ship-and-reshape inflections. The project
epic decomposes into phase sub-epics with inter-phase `needs` edges and **no
picks** — each phase's picks are planned just-in-time later (the `plan-phase`
skill), after that phase's `discuss`.

```json
{
  "epic": { "title": "…", "description": "…", "acceptance": "…" },
  "phases": [
    {
      "ref":         "phase-1",
      "title":       "…",
      "description": "Phase goal + this phase's slice of the research digest + the requirement IDs it covers. This IS the mini-brief plan-phase reads back via `bd show` in a later session — beads is the brain, so carry forward everything the per-phase planner will need.",
      "acceptance":  "Phase success criteria — becomes the verify-work fallback target.",
      "needs":       []
    },
    {
      "ref":         "phase-2",
      "title":       "…",
      "description": "…",
      "acceptance":  "…",
      "needs":       ["phase-1"]
    }
  ]
}
```

Roadmap field rules:

- **`description`** — the phase mini-brief. Because picks are planned in a later
  session, this field MUST carry forward the phase goal, the relevant slice of
  the research digest, and the requirement IDs that phase delivers. There is no
  CONTEXT.md; the bead is the only memory.
- **`acceptance`** — the phase's observable success criteria. `verify-work`
  (ccy.2) uses it as the UAT target, so make it checkable, not aspirational.
- **`needs`** — inter-phase dependencies only. Each becomes a `blocks` edge that
  gates `bd ready` transitively (a later phase's picks stay unready until the
  earlier phase closes). Do not encode pick-level ordering here.

### Shape B — single-epic picks (single-phase / degenerate)

Emitted when the work is one cohesive milestone. This is **today's exact shape**
— the one-shot flow survives behaviorally as the degenerate case, not a separate
mode.
````

- [ ] **Step 2: Add the phase-discovery methodology step (run first)**

Insert a new "### 0. Phase discovery (run first)" immediately after the "## Methodology" heading (planner.md line ~71), before "### 1. Goal-Backward decomposition":

```markdown
### 0. Phase discovery (run first)

Before decomposing, decide the plan's **shape**. Run Goal-Backward and wave
reasoning (§§1–4) far enough to see the work's structure, then apply the test:

> A phase boundary exists exactly where you would genuinely want to **ship and
> get human feedback before committing to the next chunk's HOW.**

That ship-and-reshape inflection is the only thing the per-phase discuss/verify
gate buys. Phases add *gates*, not *scheduling* — wave decomposition already
orders work within one epic without them.

- **2+ genuine inflections** → emit **Shape A (roadmap)**: one phase per cluster
  of waves; inter-cluster dependencies become phase `needs` edges; author each
  phase's mini-brief + acceptance; emit **no picks**.
- **One cohesive milestone** → emit **Shape B (single-epic picks)** — today's
  shape.

**Resolve residual ambiguity toward fewer phases.** The failure modes are
asymmetric: under-phasing loses a mid-point checkpoint but still ships and
verifies (cheap, recoverable); over-phasing manufactures PR boundaries and
forced gates that are annoying to unwind. A phase must *earn* its boundary by a
real inflection — do not split a cohesive epic just because it is large (waves
handle size). The human sees the roadmap at the `--dry-run` gate and can collapse
an over-split; that is a backstop, not license to over-phase.
```

- [ ] **Step 3: Extend the quality gate for the roadmap shape**

In the "## Quality gate (self-check before handing off)" list (planner.md ~lines 188–202), add these bullets after the existing TDD-eligible bullet:

```markdown
- [ ] Shape chosen by the §0 ship-and-reshape test, not by project size. If
      Shape A, every phase reflects a genuine ship-and-reshape inflection.
- [ ] (Shape A only) Every phase `description` carries forward its goal +
      research slice + requirement IDs (the mini-brief a later `plan-phase`
      reads). Every phase `acceptance` is checkable. `needs` encodes inter-phase
      dependencies only.
- [ ] The plan carries `phases` OR `picks`, never both.
```

- [ ] **Step 4: Validate the plugin tree**

Run:

```
claude plugin validate ./plugin --strict
claude plugin validate . --strict
grep -RnE 'weft/(agents|references|workflows)/' plugin/ ; echo "grep-exit=$?"
```

Expected: both `validate` runs report success (exit 0); the grep prints **no path lines** and `grep-exit=1` (no matches). If the grep prints any line, replace that intra-tree path with `${CLAUDE_PLUGIN_ROOT}/…` and re-run.

- [ ] **Step 5: Commit**

Commit using VCS-appropriate commands per `references/vcs-preamble.md`. Message: `feat(weft-ccy.3): weft-planner phase discovery + roadmap output shape`.

---

## Task 2: new-project — consume both shapes, diverge the closing message

**Files:**

- Modify: `plugin/skills/new-project/SKILL.md` (Phase 4 ~lines 90–108; Phase 7 ~lines 155–171)

**Depends on:** Task 1 (the planner must be able to produce a roadmap before new-project's branch is meaningful).

- [ ] **Step 1: Note the two possible planner outputs in Phase 4**

In "## Phase 4 — Dispatch weft-planner", append this paragraph after the existing sentence ending "see `${CLAUDE_PLUGIN_ROOT}/agents/planner.md`).":

```markdown
The agent applies phase discovery (planner §0) and returns **one of two shapes**:
a **roadmap** (`phases[]`, no picks) for genuinely multi-phase work, or a
**single-epic pick plan** (`picks[]`) for one cohesive milestone. Both shapes
flow through Phases 5–6 unchanged — `weft plan check` and `weft plan emit
--dry-run` validate and preview either shape. Do not ask the planner to "pick a
mode"; the shape is a property of the work it discovered.
```

- [ ] **Step 2: Diverge the closing message on shape (Phase 7)**

In "## Phase 7 — Materialise the warp: weft plan emit", replace the final paragraph (currently): 

```markdown
Confirm to the user that the warp is live and that `bd ready` will list the
first set of ready picks (a shed is woven from them with `weft shed form` when
execution begins).
```

with:

```markdown
The `weft plan emit` success envelope echoes `data.ids` (`{node-key →
bead-id}`) — read the project-epic id and any phase-sub-epic ids straight from
it; do not query bd to rediscover them. The closing message **diverges on the
emitted shape**:

- **Single-epic pick plan:** confirm the warp is live and that `bd ready` lists
  the first ready picks (a shed is woven with `weft shed form` when execution
  begins). End with: **"warp emitted — run `execute`."** (today's flow).
- **Roadmap:** confirm the project epic + phase sub-epics + inter-phase edges
  are live, and that `bd ready` currently surfaces only phase-1-eligible work
  (later phases are gated by their inter-phase `blocks` edges until earlier
  phases close). End with: **"roadmap emitted — run `discuss` on phase 1
  (`<phase-1 id from data.ids>`), then `plan-phase` on it."** This is the
  deliberately-more-interactive default the phased loop exists for.
```

- [ ] **Step 3: Validate the plugin tree**

Run:

```
claude plugin validate ./plugin --strict
claude plugin validate . --strict
grep -RnE 'weft/(agents|references|workflows)/' plugin/ ; echo "grep-exit=$?"
```

Expected: both `validate` runs exit 0; grep prints no lines, `grep-exit=1`.

- [ ] **Step 4: Commit**

Commit per `references/vcs-preamble.md`. Message: `feat(weft-ccy.3): new-project consumes roadmap shape, diverges closing message`.

---

## Task 3: plan-phase skill — JIT per-phase pick emission

**Files:**

- Create: `plugin/skills/plan-phase/SKILL.md`

**Depends on:** Task 2 (a phase sub-epic to target only exists after a roadmap is emitted; plan-phase reuses the planner's unchanged pick methodology from Task 1).

- [ ] **Step 1: Create the skill file**

Create `plugin/skills/plan-phase/SKILL.md` with exactly this content:

````markdown
---
description: Plan one phase's picks just-in-time into its sub-epic — phase-scoped planner dispatch, approval gate, then emit into the phase via re-plan.
argument-hint: "[phase-epic-id]"
---

<!--
  ~ SPDX-License-Identifier: Apache-2.0
  ~ Copyright 2026 Weft Contributors
-->

<!-- adapted from /gsd-new-project (GSD Core, MIT), scoped to one phase -->

# plan-phase workflow

Just-in-time pick planning for one phase sub-epic of a roadmap (emitted by
`new-project`'s multi-phase path). Loads the phase's mini-brief and any locked
HOW decisions from beads, dispatches `weft-planner` scoped to that phase,
gates on human approval, then materialises the picks **into the phase sub-epic**
via the re-plan upsert path. Beads is the brain — no sidecar files.

Run this after `discuss` on the phase, as part of the per-phase rhythm
(`discuss → plan-phase → execute --epic <phase> → verify-work → finish`).

---

## Phase 1 — Load phase context

**Goal:** assemble everything the per-phase planner needs, entirely from beads.

```
bd show <phase-epic-id>
```

Read from the phase sub-epic:

- **`description`** — the phase mini-brief authored at roadmap time: the phase
  goal, its slice of the research digest, and the requirement IDs it covers.
- **`acceptance`** — the phase success criteria; this is the planner's target.
- **`design` field / notes** — the locked HOW decisions `discuss` (the
  `discuss` skill) persisted for this phase.

An **empty or absent `design` field is valid input**, not an error: if `discuss`
was skipped, plan the phase from its mini-brief and acceptance alone.

---

## Phase 2 — Dispatch weft-planner (picks-only, phase-scoped)

**Goal:** produce a **picks-only** `warp-plan.json` for this phase.

Dispatch the `weft-planner` agent (model `opus`, per its frontmatter) with:

- the phase mini-brief (`description`) as the spec,
- the phase `acceptance` as the observable goal to decompose backward from,
- the locked HOW decisions from the `design` field (if any).

Instruct the planner to emit **Shape B (single-epic picks)** — this is one
phase's cohesive slice, so it is single-phase by construction. The planner's
pick methodology (Goal-Backward, vertical-slice, file-ownership, wave thinking,
TDD heuristics) applies unchanged. The epic block of its output is ignored on
emit (the `--epic` flag targets the existing phase sub-epic); only `picks[]`
matters.

---

## Phase 3 — Validate: weft plan check

```
weft plan check warp-plan.json
```

Inspect `data.valid`. If `false`, present `data.issues`, return to Phase 2 for a
revised plan, and re-run. Do not proceed with an invalid plan.

---

## Phase 4 — Human approval gate: weft plan emit --epic --dry-run

```
weft plan emit warp-plan.json --epic <phase-epic-id> --dry-run
```

This computes the upsert diff in-memory against the phase sub-epic without
mutating state (the re-plan dry-run does not call bd) and prints the preview —
picks to be created under the phase, plus any `blocks` edges among them. On a
freshly-roadmapped phase (no existing picks), every pick is a **create**.
Present the preview in full as the approval gate and wait for explicit go-ahead.

If the human requests changes, return to Phase 2 (or to `discuss` if the HOW
shifted) and re-run Phases 3–4.

---

## Phase 5 — Materialise: weft plan emit --epic

On approval:

```
weft plan emit warp-plan.json --epic <phase-epic-id>
```

The engine upserts the picks into the phase sub-epic: parents each to the phase
(`bd dep add --type parent-child`, wired post-import) and applies authored
`blocks` edges. The picks are now live under the phase; `warp-plan.json` is a
transient artifact.

Confirm the phase is planned and end with: **"phase planned — run `execute
--epic <phase-epic-id>`."** `bd ready` will surface this phase's first ready
picks (it returns them only if no unclosed earlier phase blocks this one).

---

## What this workflow does NOT do

- It does not emit a roadmap or create phase sub-epics — `new-project` does that.
  `plan-phase` only fills an existing phase with picks.
- It does not run the 4-axis research fan-out — research happened at roadmap
  time and was carried forward in the phase mini-brief.
- It does not execute or verify picks — that is `execute` and `verify-work`.
- It does not write planning-state markdown; beads is the source of truth.
````

- [ ] **Step 2: Validate the plugin tree (registers the new skill)**

Run:

```
claude plugin validate ./plugin --strict
claude plugin validate . --strict
grep -RnE 'weft/(agents|references|workflows)/' plugin/ ; echo "grep-exit=$?"
```

Expected: both `validate` runs exit 0 and the new `plan-phase` skill is accepted (auto-discovered — no manifest edit needed); grep prints no lines, `grep-exit=1`.

- [ ] **Step 3: Commit**

Commit per `references/vcs-preamble.md`. Message: `feat(weft-ccy.3): plan-phase skill — JIT per-phase pick emission`.

---

## Task 4: End-to-end validation (manual dogfood gate)

**Files:** none modified — this task is the acceptance gate proving the rhythm.

**Depends on:** Tasks 1–3.

This is the manual validation the spec §6 names (the automated CI gates are the
per-task `plugin validate` runs). It exercises both shapes. Drive it
interactively in a scratch bd workspace or a throwaway branch so the dogfood
beads do not pollute the real warp.

- [ ] **Step 1: Multi-phase path — roadmap emit**

Drive `new-project` with a deliberately multi-phase stimulus (spec §6 example):
*"build a CLI tool that (1) ships a working read-only query path, then (2) adds
mutation/write support, then (3) adds a remote-sync mode."*

Expected: the planner emits a **roadmap** (`phases[]`, no picks). After approval +
`plan emit`, verify:

```
bd show <project-epic-id>            # 3 phase sub-epics as children
bd ready                            # returns only phase-1-eligible work (none yet — no picks)
```

Expected: project epic + 3 phase sub-epics + inter-phase `blocks` edges live; the
closing message reads "roadmap emitted — run `discuss` on phase 1 …".

- [ ] **Step 2: JIT path — plan-phase on phase 1**

Run `plan-phase <phase-1-id>`. Approve the dry-run. After `emit --epic`, verify:

```
bd show <phase-1-id>                 # picks now parented under phase 1
bd ready                             # phase-1 picks now ready; phase-2/3 still gated
```

Expected: phase-1 picks land under the phase sub-epic; phase-2/3 picks (none yet)
remain gated by the inter-phase edges.

- [ ] **Step 3: Single-phase degenerate path**

Drive `new-project` with a cohesive single-milestone stimulus (e.g. *"add a
`weft doctor` subcommand that checks jj + bd are installed and prints versions"*).

Expected: the planner emits **Shape B** (epic + picks, no `phases[]`); emission
produces today's exact shape; the closing message reads "warp emitted — run
`execute`." This confirms the degenerate case is behaviorally identical to the
pre-ccy.3 one-shot flow.

- [ ] **Step 4: Record the dogfood result**

Append a note to the design bead recording the outcome:

```
bd note weft-ccy.3 "dogfood: multi-phase roadmap + JIT plan-phase + single-phase degenerate all verified — <date>"
```
<!-- adr-capture: sha256=cc23021dbe851200; session=cli; ts=2026-06-11T02:22:39Z; adrs=weft-mvh,weft-p27 -->
