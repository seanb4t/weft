<!--
  ~ SPDX-License-Identifier: Apache-2.0
  ~ Copyright 2026 Weft Contributors
-->

# `feature` Skill Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a lightweight `feature` skill — weft's analog of GSD's `/gsd-quick` — that takes a feature description against an existing weft-managed repo and produces an approved single epic + picks in minutes, plus a `new-project` routing edit that points incremental requests to it.

**Architecture:** Purely prompt-layer. `feature` is a thin composition of pieces already shipped: `bd create --type epic` (mint), an adaptive `Explore` recon pass, the adaptive real `discuss` skill (ccy.1), the `weft-planner` Shape B output (ccy.3), and the `plan emit --epic` re-plan path (ccy.3/ccy.5). No Go changes.

**Tech Stack:** Claude Code plugin skills (markdown + YAML frontmatter); `weft` CLI verbs (`plan check`, `plan emit --epic [--dry-run]`); `bd` (beads); jj for VCS.

**Spec:** `docs/superpowers/specs/2026-06-11-feature-skill-design.md`

**Design bead:** `weft-ccy.6`

---

## Grounding notes (verified against the repo)

- **Files touched:** `plugin/skills/feature/SKILL.md` is the one new-create (parent `plugin/skills/` exists; siblings `discuss/`, `execute/`, `new-project/`, `plan-phase/`, `verify-work/`). `plugin/skills/new-project/SKILL.md` exists; its `## Phase 1 — Adaptive questioning` heading (the routing-edit anchor) is at line 22, immediately after the intro + `---`.
- **No automated unit tests apply** — markdown prompts. The per-task verification gate is the CI discipline run locally: `claude plugin validate ./plugin --strict`, `claude plugin validate . --strict`, and `grep -RnE 'weft/(agents|references|workflows)/' plugin/` returning **no matches** (use `${CLAUDE_PLUGIN_ROOT}` for any intra-plugin path).
- **Skills are auto-discovered** — `plugin/.claude-plugin/plugin.json` does not enumerate skills, so creating `plugin/skills/feature/SKILL.md` registers it; no manifest edit.
- **Skill frontmatter contract** (from `discuss`/`plan-phase`): YAML `description:` + `argument-hint:`, then the SPDX header comment, then an `<!-- adapted from … -->` attribution comment.
- **Engine/skill contracts reused (do not re-implement):** `discuss` takes `[epic-id]` and persists locked HOW to the epic `design` field; `weft-planner` emits Shape B (single-epic `picks[]`); `plan emit --epic <id>` upserts picks into an existing epic (all-creates on an empty epic — `BuildReplan` classifies every pick as created when the epic has no `weft-ref` children; pinned by `TestPlanEmitReplanEmptyListCreatesAll`), ignoring the warp-plan's `epic` block; `bd create --type epic` mints a bare epic.

---

## File Structure

| File | Responsibility | Action |
|---|---|---|
| `plugin/skills/feature/SKILL.md` | The lightweight front door: precondition → brief Q&A + mint epic → adaptive recon → adaptive discuss → planner Shape B → `plan emit --epic` → suggest `execute`. | Create |
| `plugin/skills/new-project/SKILL.md` | Add a `## Phase 0` routing check: incremental work against an existing weft-managed repo → point to `/weft-feature`. | Modify |

---

## Task 1: feature skill

**Files:**

- Create: `plugin/skills/feature/SKILL.md`

- [ ] **Step 1: Create the skill file**

Create `plugin/skills/feature/SKILL.md` with exactly this content:

````markdown
---
description: Lightweight front door for incremental work on an existing weft-managed repo — brief Q&A, adaptive recon/discuss, one epic + picks against existing code, in minutes.
argument-hint: "[feature description]"
---

<!--
  ~ SPDX-License-Identifier: Apache-2.0
  ~ Copyright 2026 Weft Contributors
-->

<!-- adapted from /gsd-quick (GSD Core, MIT), rewritten to bead-backed state -->

# feature workflow

weft's analog of GSD's `/gsd-quick`: the lightweight entry for incremental work on a
codebase weft already manages. Mints one feature epic, adaptively shapes HOW and recons
the existing code **only when warranted**, then plans and emits a single epic + picks
against the current source — in minutes. Beads is the brain; there is no research
fan-out, no phase discovery, and no sidecar files.

Use `new-project` instead for a brand-new project (it does the full Q&A + research +
roadmap). Use `onboard` to make an unmanaged repo weft-ready first.

---

## Phase 0 — Precondition

Confirm the repo is weft-managed before doing anything else:

```
bd list --json
```

The repo is weft-managed when `.beads/` is present **and** `bd list --json` returns a
non-empty warp (at least one existing epic/issue). If `.beads/` is absent or the warp is
empty, stop and tell the user this repo is not weft-managed yet — point them to
`onboard` (to make it weft-ready) or `new-project` (for a greenfield build) — and exit.

---

## Phase 1 — Brief Q&A → mint the feature epic

Treat the `[feature description]` as the seed. Ask only the follow-up questions that are
genuinely ambiguous — one light round. Do **not** run new-project's "dream extraction";
this is a small, bounded feature. Stop asking once you can state, with confidence:

1. The feature's observable goal — what a user or operator can do once it ships.
2. Its acceptance criteria — checkable conditions for done.
3. What is explicitly out of scope.

Then mint the feature epic directly in beads (it is just a bead — the warp-plan path
cannot emit an epic with no picks):

```
bd create --type epic --title "<feature title>" \
  --description "<observable goal + scope>" \
  --acceptance "<checkable acceptance criteria>" \
  --json
```

Capture the returned epic id from the `--json` output's flat top-level `.id`. All later
phases target this id.

---

## Phase 2 — Adaptive recon (at most one Explore pass)

Decide whether planning this feature needs grounding in the existing code that the
planner cannot cheaply infer (unfamiliar subsystem, non-obvious file ownership,
integration points that must be matched). State your call in one line so the user can
override it.

- **Needs grounding** → dispatch **one** `Explore` subagent scoped to the relevant area.
  Ask it for a recon digest: existing patterns, the files this feature will touch and who
  owns them, integration points, and local conventions. Hold the digest in context for
  the planner. Never dispatch more than one recon pass.
- **Obvious** (e.g. "add a `--version` flag") → skip recon.

---

## Phase 3 — Adaptive discuss

Decide whether the feature has genuine HOW gray areas — library choices, config format,
storage backend, naming/convention decisions a planner would otherwise guess. State your
call in one line.

- **Has gray areas** → invoke the `discuss` skill against the feature epic:
  `discuss <epic-id>`. It settles the gray areas through structured questions and
  persists the locked decisions to the epic's bead `design` field (beads is the brain;
  no CONTEXT.md). The planner reads them in Phase 4.
- **Obvious** → skip discuss.

Recon (Phase 2) precedes discuss deliberately: HOW is shaped better once the code is
known.

Default bias for Phases 2–3: **skip both for obvious features** — that is what keeps this
a minutes-long flow. Escalate only on genuine need.

---

## Phase 4 — Plan (weft-planner, Shape B picks-only)

Dispatch the `weft-planner` agent (model `opus`, per its frontmatter) scoped to the
feature epic. Pass it:

- the epic's goal + acceptance (`bd show <epic-id> --json`),
- the recon digest from Phase 2 (if any),
- the locked HOW decisions from the epic `design` field (if discuss ran).

Instruct the planner to emit **Shape B (single-epic picks)** — this is one cohesive
feature, so it is single-phase by construction; no phase discovery. Its `warp-plan.json`
will carry an `epic` block, but `plan emit --epic` ignores it (only `picks[]` are
processed on the re-plan path), so the planner need not match the minted epic's title.

---

## Phase 5 — Validate + human approval gate

```
weft plan check warp-plan.json
```

Inspect `data.valid`. If `false`, present `data.issues`, return to Phase 4 for a revised
plan, and re-run. Do not proceed with an invalid plan.

```
weft plan emit warp-plan.json --epic <epic-id> --dry-run
```

This computes the upsert diff in-memory against the feature epic without mutating state
(the re-plan dry-run does not call bd) and prints the preview — on a freshly-minted epic,
every pick is a **create**. Present the preview in full as the approval gate and wait for
explicit go-ahead. If the user requests changes, return to Phase 4 (or Phase 3 if the HOW
shifted) and re-run Phases 4–5.

---

## Phase 6 — Materialise + hand off

On approval:

```
weft plan emit warp-plan.json --epic <epic-id>
```

The picks land parented to the feature epic. Confirm the feature is planned and end with:
**"feature planned — run `execute --epic <epic-id>`."** `bd ready` will surface the
feature's first ready picks.

---

## What this workflow does NOT do

- It does not onboard an unmanaged repo — that is `onboard`; `feature` only routes to it.
- It does not run ecosystem research (new-project's 4-agent fan-out) — out of scope.
- It does not discover phases or emit a roadmap — `feature` is always single-epic.
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

Expected: both `validate` runs exit 0 and accept the new `feature` skill (auto-discovered — no manifest edit); grep prints no lines and `grep-exit=1`.

- [ ] **Step 3: Commit**

Commit per `references/vcs-preamble.md`. Message: `feat(weft-ccy.6): feature skill — lightweight incremental-work front door`.

---

## Task 2: new-project routing to feature

**Files:**

- Modify: `plugin/skills/new-project/SKILL.md` (insert a `## Phase 0` before `## Phase 1 — Adaptive questioning`, line 22)

**Depends on:** Task 1 (the routing pointer should target a skill that exists).

- [ ] **Step 1: Insert the routing section**

In `plugin/skills/new-project/SKILL.md`, replace this heading line:

```markdown
## Phase 1 — Adaptive questioning
```

with the routing section followed by the original heading:

```markdown
## Phase 0 — Route incremental work to feature

Before any questioning, check whether this is actually a new project. If the repo is
already weft-managed — `.beads/` is present and `bd list --json` returns a non-empty warp
— and the request is **incremental work against the existing code** ("add X", "change Y",
"fix Z") rather than building a new project, do **not** run the full greenfield flow.
Point the user to `/weft-feature` (the lightweight front door: brief Q&A, no research
fan-out, one epic + picks against existing code, in minutes) and stop.

Proceed into Phase 1 only for genuine greenfield work — a brand-new project or a new
milestone in an empty/unmanaged tree.

## Phase 1 — Adaptive questioning
```

- [ ] **Step 2: Validate the plugin tree**

Run:

```
claude plugin validate ./plugin --strict
claude plugin validate . --strict
grep -RnE 'weft/(agents|references|workflows)/' plugin/ ; echo "grep-exit=$?"
```

Expected: both `validate` runs exit 0; grep prints no lines, `grep-exit=1`.

- [ ] **Step 3: Commit**

Commit per `references/vcs-preamble.md`. Message: `feat(weft-ccy.6): new-project routes incremental requests to feature`.

---

## Task 3: End-to-end validation (manual dogfood gate)

**Files:** none modified — this is the acceptance gate.

**Depends on:** Tasks 1–2.

The spec §6 names this the real coverage (the automated gates are the per-task `plugin validate` runs). Drive it interactively in a scratch bd workspace or throwaway branch so dogfood beads do not pollute the real warp.

- [ ] **Step 1: feature on the canonical missing case**

Run `feature` with the description that originally dead-ended: **"add a `weft doctor` subcommand that checks jj + bd are installed and prints versions"** against this (weft-managed) repo.

Expected: Phase 0 passes (repo is weft-managed); a brief Q&A mints one feature epic via `bd create --type epic`; recon and discuss are **skipped or run only if genuinely warranted** (the skill states its call); the planner emits Shape B picks-only; `plan emit --epic <epic>` lands picks parented to the feature epic. Verify:

```
bd show <feature-epic-id> --json    # picks parented under the feature epic
bd ready                            # feature picks now ready
```

Confirm the whole flow took minutes (not ~28) and ran no 4-agent research fan-out.

- [ ] **Step 2: new-project routing**

Run `new-project` with the same incremental request ("add a `weft doctor` subcommand") against this weft-managed repo.

Expected: Phase 0 detects incremental-work-against-existing-repo and points to
`/weft-feature` without running the greenfield questioning.

- [ ] **Step 3: precondition routing to onboard**

Run `feature` against a directory with no `.beads/` (or an empty warp).

Expected: Phase 0 stops and points the user to `onboard` / `new-project`; no epic is minted.

- [ ] **Step 4: Record the dogfood result**

```
bd note weft-ccy.6 "dogfood: feature mints epic + Shape-B picks against existing code in minutes; new-project routes incremental->feature; feature routes unmanaged->onboard — all verified — <date>"
```
<!-- adr-capture: sha256=0a915fc220e4efb0; session=cli; ts=2026-06-11T13:13:41Z; adrs=weft-yup -->
