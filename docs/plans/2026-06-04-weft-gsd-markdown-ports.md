<!--
  ~ SPDX-License-Identifier: Apache-2.0
  ~ Copyright 2026 Weft Contributors
-->

# Weft GSD Markdown Ports (Seam 5) — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Author the Weft prompt source tree — `weft/{references,agents,commands,workflows}/` + a top-level `NOTICE` — by porting GSD Core's command/agent markdown into beads+jj-native reference drafts that call **only** the shipped seam 1–4 verb surfaces, adapting GSD's substrate-agnostic methodology and rewriting its `.planning/`/worktree/`gsd-tools.cjs` plumbing.

**Architecture:** This seam's deliverable is **prompt markdown, not Go code** (spec §1/§9). Each task ports one GSD artifact per the spec §3 port map: **ADAPT** the methodology sections (start from GSD's MIT text, modify for weft), **REWRITE** the plumbing to `weft`/`bd`/`jj` verbs, **DROP** Layer-B/C artifacts (`.planning/`, `ROADMAP.md`, `SUMMARY.md`, git-worktree choreography). There is no `go test` gate; each task's objective verification is a **grep-based discipline check** (spec §7): the ported prompt MUST NOT reference `.planning/`/`worktree`/`gsd-tools`/`ROADMAP`/`SUMMARY`, MUST carry an SPDX header + GSD provenance comment, and MUST call only stable verbs. Prose quality is verified by the downstream `/review-pr` gate.

**Tech Stack:** Markdown with YAML frontmatter (Claude-Code-native — the runtime-agnostic lingua franca, spec §5). No engine code. Tools the executor uses: `deepwiki` (fetch GSD source from `open-gsd/gsd-core` to adapt from), `rg` (validation), `jj` (commit).

**Spec:** `docs/seams/05-gsd-markdown-ports.md` (design-reviewer READY round 1). Porting strategy §2; port map §3; thin-orchestrator pattern §4; layout/naming §5; licensing/attribution §6; reference-draft discipline §7.

**Stable verb surface (the ONLY verbs a ported prompt may call — spec §7):**
`weft shed form|isolate|integrate|cleanup` · `weft pick seal|verify|land|redo` · `weft ws` · `weft reap` · `weft resume` · `weft plan check|emit` · `weft conflict open|finalize` · plus `bd …` and `jj …` per the agent-safety reference. The `warp-plan.json` schema the planner emits is **seam 2 §3** (`{epic:{title,description,acceptance?}, picks:[{ref,title,description,needs?,files?,priority?,labels?}]}`).

**Deferred (out of scope — DO NOT author):** `/weft-finish` and `/weft-verify` — they pair with `weft finish open/reconcile`, which is **not built** (deferred from seam 1's ship plan); porting them now would violate §7's "call only stable verbs." The `weft install` runtime transform (§8). The ~60 other GSD commands / ~28 other agents (phase-types, doc-synth) — DROP for v1 (§3). Per-prompt section-classification worksheets (§8).

**Authoring altitude (why this plan specifies porting-specs, not full prose):** Each task gives the executor a precise port spec — target path, source GSD artifact, the section-by-section ADAPT/REWRITE/DROP disposition, the exact verb-call sequence, frontmatter, provenance, and the grep validation. The executor fetches the GSD source via deepwiki and authors the adapted prose (the prose is the executor's artifact, as code is in a code plan). Inlining full GSD-adapted prompt text in the plan would be both massive and presumptuous of the adaptation. The grep checks make each port objectively verifiable regardless.

---

## File Structure

| File | Responsibility |
|---|---|
| `NOTICE` (create) | Top-level GSD Core (MIT) attribution for ADAPTed methodology (spec §6). |
| `weft/references/jj-agent-safety.md` (create) | The jj agent profile prompts cite: `--no-pager`, `--git` diffs, change-ids not hashes, edit-markers-not-`jj resolve`, `-m` always, fetch-at-task-start. |
| `weft/references/bead-change-spine.md` (create) | The bead↔`jj-change:<id>` spine + `model:*` routing label convention. |
| `weft/references/tdd-verify-discipline.md` (create) | ADAPTed TDD / verify-as-data / fresh-context principle. |
| `weft/agents/weft-planner.md` (create) | Port of `gsd-planner` → emits `warp-plan.json` (seam 2). |
| `weft/agents/weft-executor.md` (create) | Port of `gsd-executor` → TDD + `weft pick seal` (seams 1/3). |
| `weft/agents/weft-reviewer.md` (create) | Port of `gsd-code-reviewer` → verdict-as-data (`weft pick verify`). |
| `weft/agents/weft-resolver.md` (create) | Port of `gsd-code-fixer` → marker-editing inside `weft conflict open/finalize` (seam 4). |
| `weft/commands/weft-new-project.md` + `weft/workflows/new-project.md` (create) | Port of `/gsd-new-project` → `warp-plan.json` → `weft plan emit`. |
| `weft/commands/weft-execute.md` + `weft/workflows/execute.md` (create) | Port of `/gsd-execute-phase` → the thin wave orchestrator over `weft shed`/`pick`/`conflict`. |

All paths are new (the `weft/` prompt tree does not yet exist; Task 1 creates it). Engine code under `internal/`, `cmd/` is untouched.

**Infra prerequisite (collision fix — Task 1 handles it):** the spec's `weft/` source dir collides with the conventional `weft` build binary at the repo root, and `.gitignore`'s `/weft` line would silently ignore the whole `weft/` tree (making the deliverable untracked). Resolution (keeps the spec's `weft/` location): Task 1 drops the `/weft` line from `.gitignore` (binaries stay covered by `/cmd/weft/weft` and `/dist/`); the engine binary is henceforth built to `dist/` (or `/dev/null` for compile-checks), never the repo root. This is a design-review gap in spec §5 surfaced during planning; the fix is minimal and spec-faithful.

**Discipline contract (every ported prompt, enforced by each task's grep check, spec §6/§7):**
1. SPDX header comment (`<!-- SPDX-License-Identifier: Apache-2.0 ... -->`).
2. A provenance line where ADAPTed: `adapted from gsd-<x>.md (GSD Core, MIT)`.
3. ZERO references to `.planning`, `ROADMAP`, `STATE.md`, `SUMMARY`, `PLAN.md`, `worktree`, `gsd-tools`, `REVIEW.md`, `REVIEW-FIX.md` (the deleted Layer-B/C — spec §3/§7).
4. Verb calls drawn ONLY from the stable surface above.

---

### Task 1: Foundation — `NOTICE` + `weft/references/`

**Files:**
- Modify: `.gitignore`
- Create: `NOTICE`
- Create: `weft/references/jj-agent-safety.md`
- Create: `weft/references/bead-change-spine.md`
- Create: `weft/references/tdd-verify-discipline.md`

- [ ] **Step 1: Clear the `weft/` path collision** (infra prerequisite)

In `.gitignore`, delete the line `/weft` (it would ignore the new `weft/` source tree). Leave `/cmd/weft/weft` and `/dist/` — binaries build there, never the repo root. If a stray `./weft` binary exists at the repo root, remove it (`rm -f ./weft`) so the `weft/` directory can be created.

Verify:
```
! grep -qx '/weft' .gitignore && echo GITIGNORE-FIXED
test ! -e weft || test -d weft && echo PATH-CLEAR   # weft is absent or already a dir
```
Expected: `GITIGNORE-FIXED`; `PATH-CLEAR`.

- [ ] **Step 2: Author `NOTICE`** (top-level, spec §6)

Plain-text `NOTICE` crediting GSD Core. Exact content:

```
Weft
Copyright 2026 Weft Contributors

This product includes methodology and prompt structure adapted from
GSD Core (https://github.com/open-gsd/gsd-core), licensed under the MIT
License, Copyright (c) its contributors.

Adapted prompt sections (planning methodology, executor TDD discipline,
review rigor, fix-as-guidance) under weft/agents/ and weft/commands/ carry
per-file "adapted from gsd-<x>.md" provenance notes. Rewritten sections
(beads + jj mechanics) are original work under Apache-2.0.
```

- [ ] **Step 3: Author `weft/references/jj-agent-safety.md`**

SPDX header + a reference doc the agents/workflows cite. REWRITE from scratch (beads+jj is original — no GSD source). Sections:
- `# jj agent-safety profile` intro: "every weft agent/workflow that touches VCS follows this."
- Rules (lift from `.jj/`-skill discipline already in the repo's CLAUDE.md / jj skill): always `--no-pager`; `--git` on diffs; reference **change-ids**, not commit hashes; **edit conflict markers directly, never `jj resolve`** (it hangs non-interactively); `-m` always (never the editor); `jj git fetch` at task start; recovery is change-scoped (`jj abandon` / `jj op revert`), never `jj op restore`.
- A one-line pointer that the engine verbs (`weft shed`/`pick`/`conflict`) already encapsulate the dangerous multi-step choreography — agents call the verb, not raw jj, except where they edit files/markers.

- [ ] **Step 4: Author `weft/references/bead-change-spine.md`**

SPDX header. REWRITE (original). Sections:
- The spine: a bead pins its jj change via the `jj-change:<id>` label (`weft pick seal` writes it; `integrate`/`land`/`redo`/`resume` read it).
- Identity labels: `weft-ref:<ref>` (plan↔warp join, seam 2), `phase:*`, `jj-change:<id>`.
- `model:*` routing labels (Rule 5): a bead's `model:haiku|sonnet|opus` label selects the dispatch model — the weft analog of GSD's `resolve-model` (spec §4/§8).

- [ ] **Step 5: Author `weft/references/tdd-verify-discipline.md`**

SPDX header + provenance (ADAPT from GSD `references/*.md`). Fetch GSD's references via `deepwiki open-gsd/gsd-core` ("contents of the TDD / verify / fresh-context reference docs"). ADAPT: red-green-refactor TDD, the verify gate, the fresh-context dispatch principle (orchestrator context stays clean; agents are spawned fresh). REWRITE any plumbing: the verify gate's verdict is **data** (`weft pick verify` → `{pass}` on exit 0), not a `REVIEW.md` file.

- [ ] **Step 6: Validate (the discipline gate)**

Run:
```
test -f NOTICE && grep -q "GSD Core" NOTICE && echo NOTICE-OK
rg -l 'SPDX-License-Identifier' weft/references/*.md | wc -l   # expect 3
! rg -i '\.planning|worktree|gsd-tools|ROADMAP|STATE\.md|SUMMARY|REVIEW\.md' weft/references/ NOTICE && echo NO-LAYER-BC-REFS
```
Expected: `NOTICE-OK`; `3`; `NO-LAYER-BC-REFS` (the `!`-negated rg exits 0 only when there are zero forbidden matches).

- [ ] **Step 7: Commit**

Run: `jj commit -m "docs(weft-hjx.5): .gitignore fix + NOTICE + weft/references foundation (seam 5)"`

---

### Task 2: `weft/agents/weft-planner.md` — port of `gsd-planner`

**Files:**
- Create: `weft/agents/weft-planner.md`

- [ ] **Step 1: Fetch the source**

`deepwiki open-gsd/gsd-core` → "Full section-by-section content of agents/gsd-planner.md: role, planning_context, downstream_consumer, quality_gate, the Goal-Backward methodology, and TDD-mode heuristics."

- [ ] **Step 2: Author the port** (ADAPT/REWRITE/DROP per spec §3)

YAML frontmatter (Claude-Code-native):
```yaml
---
name: weft-planner
description: Plans a spec into a warp-plan.json (the bead graph) — vertical-slice picks, file-ownership dependency reasoning, wave thinking. Emitted warp goes to `weft plan emit`.
model: opus
---
```
Then SPDX header comment + provenance (`adapted from gsd-planner.md (GSD Core, MIT)`).

- **ADAPT** (keep the reasoning): goal-backward decomposition, vertical-slice bias, file-ownership→dependency reasoning, wave thinking (cross-reference seam 2 §4's overlap policy), TDD-mode heuristics for eligible picks.
- **REWRITE** (the output contract): the agent's deliverable is a **`warp-plan.json`** conforming to seam 2 §3 — `{epic:{title,description,acceptance?}, picks:[{ref,title,description,needs?,files?,priority?,labels?}]}` — NOT a `PLAN.md`. `description` per pick IS the plan (seam 2 §3 / design §5). `needs` = explicit deps; `files` = ownership estimate that drives overlap-derived edges. Instruct: hand the file to `weft plan check` then `weft plan emit --dry-run` for the human gate.
- **DROP**: all `.planning/` reads/writes (`STATE.md`/`ROADMAP.md`/`REQUIREMENTS.md`/`CONTEXT.md`), the `PLAN.md` frontmatter/XML-task format, the `produces` PLAN.md frontmatter key.

- [ ] **Step 3: Validate**

Run:
```
rg -q 'name: weft-planner' weft/agents/weft-planner.md && rg -q 'warp-plan.json' weft/agents/weft-planner.md && rg -q 'gsd-planner' weft/agents/weft-planner.md && echo SHAPE-OK
! rg -i '\.planning|worktree|gsd-tools|ROADMAP|STATE\.md|PLAN\.md|SUMMARY' weft/agents/weft-planner.md && echo NO-LAYER-BC-REFS
```
Expected: `SHAPE-OK`; `NO-LAYER-BC-REFS`.

- [ ] **Step 4: Commit**

Run: `jj commit -m "docs(weft-hjx.5): weft-planner agent — port of gsd-planner (seam 5)"`

---

### Task 3: `weft/agents/weft-executor.md` — port of `gsd-executor`

**Files:**
- Create: `weft/agents/weft-executor.md`

- [ ] **Step 1: Fetch the source**

`deepwiki open-gsd/gsd-core` → "Full content of agents/gsd-executor.md: objective, worktree_branch_check, parallel/sequential_execution, files_to_read, success_criteria, deviation rules, TDD execution, checkpoint_protocol."

- [ ] **Step 2: Author the port**

Frontmatter:
```yaml
---
name: weft-executor
description: Executes one ready pick (bead) in its isolated workspace — TDD, atomic unit of work — then seals it. Dispatched fresh per pick by the execute workflow.
model: sonnet
---
```
SPDX + provenance (`adapted from gsd-executor.md (GSD Core, MIT)`).

- **ADAPT**: TDD discipline (cite `weft/references/tdd-verify-discipline.md`), deviation handling (auto-fix obvious bugs / add critical missing functionality), atomic-unit-of-work — the pick is the unit; its bead `description` is the plan.
- **REWRITE**: the executor runs **inside the workspace `weft shed isolate` already created** (it does not create worktrees) — cite `weft/references/jj-agent-safety.md`. Commit = `weft pick seal <bead>` (which does the jj commit + pins the `jj-change:<id>` spine label); verification = `weft pick verify <bead>`. The bead↔change spine is `weft/references/bead-change-spine.md`.
- **DROP**: `<worktree_branch_check>` git assertions (the engine owns workspace identity, seam 3); `SUMMARY.md` writes; `.planning/` reads; `gsd-tools.cjs` calls; per-plan worktree decision.

- [ ] **Step 3: Validate**

Run:
```
rg -q 'name: weft-executor' weft/agents/weft-executor.md && rg -q 'weft pick seal' weft/agents/weft-executor.md && rg -q 'gsd-executor' weft/agents/weft-executor.md && echo SHAPE-OK
! rg -i '\.planning|worktree|gsd-tools|SUMMARY|git worktree|REVIEW\.md' weft/agents/weft-executor.md && echo NO-LAYER-BC-REFS
```
Expected: `SHAPE-OK`; `NO-LAYER-BC-REFS`.

- [ ] **Step 4: Commit**

Run: `jj commit -m "docs(weft-hjx.5): weft-executor agent — port of gsd-executor (seam 5)"`

---

### Task 4: `weft/agents/weft-reviewer.md` — port of `gsd-code-reviewer`

**Files:**
- Create: `weft/agents/weft-reviewer.md`

- [ ] **Step 1: Fetch the source**

`deepwiki open-gsd/gsd-core` → "Full content of agents/gsd-code-reviewer.md: role, the bug/security/quality detection methodology, how it honors CLAUDE.md and project skills, and its REVIEW.md output format."

- [ ] **Step 2: Author the port**

Frontmatter:
```yaml
---
name: weft-reviewer
description: Reviews a pick's change for bugs, security, and quality. Returns its verdict as data (the weft pick verify gate), not a review file.
model: sonnet
---
```
SPDX + provenance (`adapted from gsd-code-reviewer.md (GSD Core, MIT)`).

- **ADAPT**: review rigor — bug detection, security (OWASP-style), code-quality; honoring `CLAUDE.md` + project skills.
- **REWRITE**: the verdict is **data** — the reviewer's output maps to `weft pick verify`'s `{pass, …}` envelope (verdict-as-data, seam 1; cite `tdd-verify-discipline.md`), NOT a `REVIEW.md` artifact. Note the §8 open question inline (this agent MAY later fold into `pick verify`'s gate) as a provenance comment, not a behavior.
- **DROP**: `REVIEW.md` file writes; `.planning/` phase-dir paths.

- [ ] **Step 3: Validate**

Run:
```
rg -q 'name: weft-reviewer' weft/agents/weft-reviewer.md && rg -q 'pick verify' weft/agents/weft-reviewer.md && rg -q 'gsd-code-reviewer' weft/agents/weft-reviewer.md && echo SHAPE-OK
! rg -i '\.planning|worktree|gsd-tools|REVIEW\.md|REVIEW-FIX' weft/agents/weft-reviewer.md && echo NO-LAYER-BC-REFS
```
Expected: `SHAPE-OK`; `NO-LAYER-BC-REFS`.

- [ ] **Step 4: Commit**

Run: `jj commit -m "docs(weft-hjx.5): weft-reviewer agent — port of gsd-code-reviewer (seam 5)"`

---

### Task 5: `weft/agents/weft-resolver.md` — port of `gsd-code-fixer`

**Files:**
- Create: `weft/agents/weft-resolver.md`

- [ ] **Step 1: Fetch the source**

`deepwiki open-gsd/gsd-core` → "Full content of agents/gsd-code-fixer.md: role, project_context, fix_strategy (intelligent fix = guidance not blind patch), rollback_strategy (per-finding), execution_flow."

- [ ] **Step 2: Author the port**

Frontmatter:
```yaml
---
name: weft-resolver
description: Resolves a first-class jj conflict in a resolution workspace — edits the conflict markers to a correct merge using the colliding picks' intent, then returns. The engine squashes.
model: sonnet
---
```
SPDX + provenance (`adapted from gsd-code-fixer.md (GSD Core, MIT)`).

- **ADAPT**: fix-as-guidance (the conflicting picks' bead descriptions are intent, not blind patches), verify-each, atomic intent.
- **REWRITE** (this is the seam-4 contract — cite `docs/seams/04-conflict-resolution.md` §5 and `jj-agent-safety.md`): the resolver is dispatched **into the resolution workspace `weft conflict open <bead>` created**; it reads the colliding beads' descriptions + the conflict markers, **edits the markers directly** to a correct merge and removes them, then returns. It MUST NOT run `jj resolve` (hangs) and MUST NOT commit — `weft conflict finalize <bead>` does the squash. Marker style is `diff` (set by `conflict open`).
- **DROP**: `REVIEW-FIX.md`; the `git checkout -- {file}` rollback (jj records conflicts in the commit — recovery is `weft pick redo` / re-open, not git checkout); `gsd-tools query commit` atomic-commit flow.

- [ ] **Step 3: Validate**

Run:
```
rg -q 'name: weft-resolver' weft/agents/weft-resolver.md && rg -q 'conflict finalize' weft/agents/weft-resolver.md && rg -q 'gsd-code-fixer' weft/agents/weft-resolver.md && echo SHAPE-OK
! rg -i '\.planning|worktree|gsd-tools|REVIEW-FIX|git checkout|jj resolve' weft/agents/weft-resolver.md && echo NO-FORBIDDEN-REFS
```
Expected: `SHAPE-OK`; `NO-FORBIDDEN-REFS` (note: `jj resolve` is forbidden in this prompt per seam-4 §5 — the resolver edits markers).

- [ ] **Step 4: Commit**

Run: `jj commit -m "docs(weft-hjx.5): weft-resolver agent — port of gsd-code-fixer (seam 5)"`

---

### Task 6: `/weft-new-project` command + workflow — port of `/gsd-new-project`

**Files:**
- Create: `weft/commands/weft-new-project.md`
- Create: `weft/workflows/new-project.md`

- [ ] **Step 1: Fetch the source**

`deepwiki open-gsd/gsd-core` → "Content and orchestration flow of the /gsd-new-project command and gsd-roadmapper: adaptive questioning, parallel research, requirement extraction, and what .planning artifacts it writes."

- [ ] **Step 2: Author the command** (`weft/commands/weft-new-project.md`)

Frontmatter:
```yaml
---
description: Plan a new project into the warp — adaptive questions, research, then emit a warp-plan.json the human approves.
argument-hint: "[project description]"
---
```
SPDX + provenance. Thin: the command states intent + invokes the workflow body in `weft/workflows/new-project.md`.

- [ ] **Step 3: Author the workflow** (`weft/workflows/new-project.md`, the thin orchestrator §4)

SPDX + provenance (`adapted from /gsd-new-project + gsd-roadmapper`).
- **ADAPT**: the adaptive-questioning + parallel-research + requirement-extraction flow.
- **REWRITE** (the thin-orchestrator mapping, spec §4): dispatch the `weft-planner` agent (model from its frontmatter / `model:*` convention) to produce `warp-plan.json`; then `weft plan check <file>`; then `weft plan emit <file> --dry-run` as the **human approval gate**; on approval `weft plan emit <file>`. Host-runtime dispatch (Claude Code `Agent`, etc.) is unchanged-pattern and stays with the host (spec §5, design §7).
- **DROP**: writing `ROADMAP.md`/`STATE.md`/`PROJECT.md`/`REQUIREMENTS.md`; `gsd-tools.cjs init`/`state`.

- [ ] **Step 4: Validate**

Run:
```
rg -q 'weft plan emit' weft/workflows/new-project.md && rg -q 'weft-planner' weft/workflows/new-project.md && rg -q 'SPDX' weft/commands/weft-new-project.md && echo SHAPE-OK
! rg -i '\.planning|worktree|gsd-tools|ROADMAP|STATE\.md|PROJECT\.md|REQUIREMENTS\.md' weft/commands/weft-new-project.md weft/workflows/new-project.md && echo NO-LAYER-BC-REFS
```
Expected: `SHAPE-OK`; `NO-LAYER-BC-REFS`.

- [ ] **Step 5: Commit**

Run: `jj commit -m "docs(weft-hjx.5): /weft-new-project command + workflow — port of gsd-new-project (seam 5)"`

---

### Task 7: `/weft-execute` command + workflow — port of `/gsd-execute-phase`

**Files:**
- Create: `weft/commands/weft-execute.md`
- Create: `weft/workflows/execute.md`

- [ ] **Step 1: Fetch the source**

`deepwiki open-gsd/gsd-core` → "Orchestration flow of /gsd-execute-phase and the execute-phase workflow: wave formation, parallel dispatch, the verify gate, worktree-safety.cjs choreography, and how it integrates/merges results."

- [ ] **Step 2: Author the command** (`weft/commands/weft-execute.md`)

Frontmatter:
```yaml
---
description: Weave a wave of ready picks — form the shed, isolate, dispatch executors, integrate, resolve conflicts, land.
argument-hint: "[epic-id]"
---
```
SPDX + provenance. Thin: invokes the workflow body.

- [ ] **Step 3: Author the workflow** (`weft/workflows/execute.md`, the thin orchestrator §4)

SPDX + provenance (`adapted from /gsd-execute-phase + execute-phase workflow`).
- **ADAPT**: wave orchestration, verify-gate methodology, fresh-context per-pick dispatch.
- **REWRITE** (the §4 tool-layer mapping — this is the capstone orchestrator; call ONLY stable verbs):
  1. `weft shed form --epic <id>` → the ready wave.
  2. `weft shed isolate <bead>...` → per-pick workspaces.
  3. dispatch a fresh `weft-executor` per pick (model via `model:*` label) into its workspace; each ends with `weft pick seal`.
  4. `weft pick verify <bead>` per pick (dispatch `weft-reviewer` where deeper review is wanted); verdict is data.
  5. `weft shed integrate <bead>...` → dep-ordered stack; conflicts surface as `{bead,change}` data.
  6. for each conflicted bead: `weft conflict open <bead>` → dispatch `weft-resolver` → `weft conflict finalize <bead>` (escalates via the `human` label if unresolved).
  7. `weft pick land <bead>` per conflict-free pick.
  8. `weft shed cleanup <bead>...` (and `weft reap` for orphans).
  9. `weft resume --epic <id>` to project state between waves; loop until the epic's ready set is empty.
- **DROP**: `worktree-safety.cjs`, all `git worktree` choreography, `gsd-tools.cjs`, `.planning/` state, `SUMMARY.md`.

- [ ] **Step 4: Validate**

Run:
```
for v in 'shed form' 'shed isolate' 'pick seal' 'pick verify' 'shed integrate' 'conflict open' 'conflict finalize' 'pick land' 'shed cleanup' 'resume'; do rg -q "weft $v" weft/workflows/execute.md || echo "MISSING: weft $v"; done; echo VERB-CHECK-DONE
! rg -i '\.planning|worktree|gsd-tools|SUMMARY|weft finish' weft/commands/weft-execute.md weft/workflows/execute.md && echo NO-FORBIDDEN-REFS
```
Expected: `VERB-CHECK-DONE` with no `MISSING:` lines; `NO-FORBIDDEN-REFS` (note `weft finish` is forbidden here — it is not a stable verb).

- [ ] **Step 5: Full discipline sweep + commit**

Run (whole tree must be clean):
```
! rg -i '\.planning|worktree|gsd-tools|ROADMAP|STATE\.md|SUMMARY|REVIEW\.md|REVIEW-FIX|weft finish' weft/ && echo TREE-CLEAN
rg -L 'SPDX-License-Identifier' weft/ ; echo "(any file above lacks an SPDX header — should be none)"
go build -o /dev/null ./cmd/weft   # sanity: prompt-only seam must not have touched engine code (-o /dev/null avoids the weft/ path collision)
```
Expected: `TREE-CLEAN`; no files missing SPDX; build still succeeds.

Run: `jj commit -m "docs(weft-hjx.5): /weft-execute command + workflow — port of gsd-execute-phase (seam 5)"`

---

## Done criteria

- `weft/{references,agents,commands,workflows}/` + `NOTICE` exist; every prompt file carries an SPDX header and (where ADAPTed) a `gsd-<x>.md (GSD Core, MIT)` provenance line; `NOTICE` credits GSD Core.
- Discipline sweep clean: **zero** references to `.planning`/`ROADMAP`/`STATE.md`/`SUMMARY`/`PLAN.md`/`worktree`/`gsd-tools`/`REVIEW.md`/`REVIEW-FIX` anywhere under `weft/`.
- Every ported prompt calls **only** the stable seam 1–4 verb surface; `weft-execute`'s workflow exercises the full weave loop (`shed form/isolate/integrate/cleanup` · `pick seal/verify/land` · `conflict open/finalize` · `resume`); `weft-new-project` drives `plan check`/`emit`; the resolver edits markers and never runs `jj resolve` or commits.
- `weft-planner` emits a seam-2 §3 `warp-plan.json` (no `PLAN.md`); `weft-executor` seals via `weft pick seal`; `weft-reviewer` returns verdict-as-data.
- `go build -o /dev/null ./cmd/weft` still succeeds (no engine code touched); `.gitignore` no longer ignores `weft/`.

## Out of scope (follow-on / §8 sub-seams)

- `/weft-finish` + `/weft-verify` — blocked on the unbuilt `weft finish open/reconcile` (seam-1 ship plan); port once that verb lands.
- The `weft install` runtime transform (frontmatter/command-syntax/placement per host); whether it is a `weft` verb or separate tool.
- Folding `weft-reviewer` into `pick verify` vs keeping it a distinct agent (§8).
- The ~60 other GSD commands / ~28 other agents (phase-type `ui`/`ai`/`spec`, doc-synthesis) — DROP for v1.

<!-- adr-capture: sha256=ff38e987a3b0bdd5; session=cli; ts=2026-06-04T21:05:49Z; adrs= -->
