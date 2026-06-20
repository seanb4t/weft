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

## Phase 1.5 — UI safety gate (non-blocking)

If this phase looks like **frontend work** — a design system is present in the
repo (`components.json`, Tailwind config, a frontend framework in
`package.json`) **or** the mini-brief / existing picks mention UI / frontend /
component / page / screen — **and** no UI contract exists for it (no `ui-spec`
`decision` bead related to the phase epic, and no UI contract in the `design`
field), prompt the human:

> "This phase looks like frontend work but has no locked UI contract. Run
> `ui-phase <phase-epic-id>` first? (recommended / skip)"

This is a **nudge, not a block** — the human may skip and plan anyway. There is
no config toggle (weft has no `workflow.*` system); the gate is always evaluated
but always skippable. On a non-frontend phase, or one that already has a UI
contract, say nothing and continue.

---

## Phase 2 — Dispatch weft-planner (picks-only, phase-scoped)

**Goal:** produce a **picks-only** `warp-plan.json` for this phase.

Dispatch the `weft-planner` agent (model `opus`, per its frontmatter) with:

- the phase mini-brief (`description`) as the spec,
- the phase `acceptance` as the observable goal to decompose backward from,
- the locked HOW decisions from the `design` field (if any),
- the locked **UI contract** (if any) — the `ui-spec` `decision` bead / `design`-field
  contract from `ui-phase` — so picks reference the locked spacing tokens, color
  variables, typography, and copywriting decisions.

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
