---
description: Lock a phase's UI contract before planning — load phase context + sketch findings, detect the design system, run the ui-researcher/ui-checker loop, persist the contract as a ui-spec decision bead + epic design field. Frontend phases only.
argument-hint: "[phase-epic-id]"
---

<!--
  ~ SPDX-License-Identifier: Apache-2.0
  ~ Copyright 2026 Weft Contributors
-->

<!-- adapted from /gsd-ui-phase (GSD Core, MIT), rewritten to bead-backed state -->

# ui-phase workflow

weft's analog of `/gsd-ui-phase`: lock the **UI contract** for a frontend phase
*before* `plan-phase`, so generated picks reference consistent spacing tokens,
color variables, typography, and copy. Runs after `discuss` (and any `sketch`),
before `plan-phase`, in the per-phase rhythm. beads is the brain — the contract
is a `decision` bead + the epic `design` field, not a `UI-SPEC.md` file.

> Optional door, frontend phases only. It does not generate UI code — it locks
> the design contract the planner consumes.

---

## Phase 1 — Load context (from beads)

```
bd show <phase-epic-id>
```

Read the phase sub-epic's `description` (mini-brief), `acceptance`, and `design`
field (locked HOW decisions from `discuss`). Pull any **sketch finding** for this
surface (`bd memories sketch` / the `design`-field direction). If a `ui-spec`
`decision` bead already exists for this phase, offer **update / view / skip**.

An empty `design` field is valid input — proceed from the mini-brief +
acceptance.

---

## Phase 2 — Detect design-system state

Scan the repo for the current UI substrate so the researcher asks only what is
genuinely open:

- `components.json` (shadcn), Tailwind config, existing design-token files;
- the frontend framework (`package.json` dependencies).

Summarise what already exists; settled choices are not re-asked.

---

## Phase 3 — Dispatch `weft-ui-researcher`

Dispatch the `weft-ui-researcher` agent (model `opus`, per its frontmatter; see
`${CLAUDE_PLUGIN_ROOT}/agents/ui-researcher.md`) with: the phase context from
Phase 1, the sketch finding (settled — not to be re-asked), and the detected
design-system state from Phase 2. It asks only **unanswered** questions across
the five areas (spacing / color / typography / copywriting / registry-safety)
and returns a **draft UI contract**. For visual sub-questions (comparing spacing
scales or palettes), it MAY use the `sketch` companion
(`${CLAUDE_PLUGIN_ROOT}/skills/sketch/scripts/visual-companion/`).

---

## Phase 4 — Dispatch `weft-ui-checker` (revision loop, ≤2)

Dispatch the `weft-ui-checker` agent (model `sonnet`; see
`${CLAUDE_PLUGIN_ROOT}/agents/ui-checker.md`) with the draft contract + phase
context. It returns a verdict whose first line is `VERDICT: PASS` or
`VERDICT: ISSUES`. On `ISSUES`, re-run `weft-ui-researcher` on the flagged items
and re-check — **at most two iterations**. After the second iteration, proceed
with the best draft and note any residual gaps for the human gate.

---

## Phase 5 — Approve + persist

Present the locked UI contract to the human. On approval, persist bead-native:

```
ID=$(bd create --type=decision --labels ui-spec \
      --title "UI contract: <phase>" \
      --description "<the locked contract: spacing tokens, color variables, typography, copywriting, registry-safety>" \
      --json | jq -r .id)
bd dep add <phase-epic-id> "$ID" --type related
bd update <phase-epic-id> --design "<existing design + the locked UI contract>"
```

(`--type related`, not the default blocking edge — an epic can only be *blocked*
by another epic.) The `design`-field copy gives `plan-phase` the contract the
same session (ADR `weft-b19`); the `decision` bead is the durable, query-able
record.

End with: **"UI contract locked — run `plan-phase <phase-epic-id>`."**

---

## What this workflow does NOT do

- It does not write `UI-SPEC.md` / `.planning/` files — the contract is a
  `decision` bead + the epic `design` field.
- It does not introduce a `workflow.*` config toggle — it is opt-in by
  invocation; `plan-phase`'s safety gate is the nudge.
- It does not generate UI code or run the planner — that is `plan-phase` /
  `execute`.
