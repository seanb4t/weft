<!--
  ~ SPDX-License-Identifier: Apache-2.0
  ~ Copyright 2026 Weft Contributors
-->

# Phase A — `discuss` + `verify-work` skills: Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship the two interactive gate skills — `discuss` (weft-ccy.1, port of `/gsd-discuss-phase`) and `verify-work` (weft-ccy.2, port of `/gsd-verify-work`) — in both prompt trees, plus the ADR weft-cfp addendum.

**Architecture:** Prompt-layer only (no Go changes). Each skill follows the seam-5 two-tree convention: a `weft/commands/weft-X.md` + `weft/workflows/X.md` pair (GSD-parity source tree), collapsed into `plugin/skills/X/SKILL.md` per ADR weft-88z with intra-tree citations rewritten to `${CLAUDE_PLUGIN_ROOT}`. Both skills are scoped to **any epic id** (phase sub-epic or today's one-shot epic) and persist all state to beads (no `.planning/` files). Spec: `docs/superpowers/specs/2026-06-09-layer-a-interactive-phased-loop-design.md`.

**Tech Stack:** Markdown skills (Claude Code plugin model), `bd` CLI (epic design/notes fields), existing weft verbs. Validation: `claude plugin validate --strict` (both paths), CI grep-discipline, `rg` shape checks.

**Bead mapping:** Tasks 1–2 → `weft-ccy.1`; Tasks 3–4 → `weft-ccy.2`; Task 5 → both; Task 6 → epic-level (spec "ADR impact").

---

## File Structure

```text
weft/commands/weft-discuss.md        # thin command (NEW)
weft/workflows/discuss.md            # workflow body (NEW)
weft/commands/weft-verify-work.md    # thin command (NEW)
weft/workflows/verify-work.md        # workflow body (NEW)
plugin/skills/discuss/SKILL.md       # collapsed pair (NEW)
plugin/skills/verify-work/SKILL.md   # collapsed pair (NEW)
plugin/README.md                     # skills list (MODIFY)
docs/adr/weft-cfp-*.md               # re-rendered with addendum (MODIFY, via /adr)
```

---

### Task 1: `/weft-discuss` command + workflow — port of `/gsd-discuss-phase` (weft-ccy.1)

**Files:**

- Create: `weft/commands/weft-discuss.md`
- Create: `weft/workflows/discuss.md`

- [ ] **Step 1: Fetch the source**

`deepwiki open-gsd/gsd-core` → "Full flow of /gsd-discuss-phase and workflows/discuss-phase.md: how gray areas are derived and categorized, the file-scout step, the prior-decision check, the per-area question rhythm (4 single questions, 2-3 recommended-choice options), scope-creep deferral, and the CONTEXT.md sections it writes (domain, decisions, canonical_refs, code_context, specifics, deferred)."

- [ ] **Step 2: Author the command** (`weft/commands/weft-discuss.md`)

Frontmatter:

```yaml
---
description: Shape HOW a phase gets built — adaptive gray-area questions whose locked decisions land in the epic's bead design field for the planner.
argument-hint: "[epic-id]"
---
```

SPDX + provenance (`adapted from /gsd-discuss-phase (GSD Core, MIT)`). Thin: states intent + invokes the workflow body in `weft/workflows/discuss.md`.

- [ ] **Step 3: Author the workflow** (`weft/workflows/discuss.md`)

SPDX + provenance. Structure the body as numbered sections mirroring `weft/workflows/execute.md` house style (Inputs table, methodology sections, step list):

- **Inputs:** `epic-id` (required). Guard: validate the id shape against the standing allowlist idiom (`^[a-zA-Z0-9][a-zA-Z0-9._-]*$`, see `weft-cli-validate-user-id-before-revset-or-gh-api` convention) before interpolating into any shell command; refuse on mismatch.
- **ADAPT** (GSD methodology, bead-backed):
  1. **Load the epic:** `bd show <epic-id>` — the epic's title/description/acceptance is the phase goal (replaces GSD's ROADMAP.md read). If the epic already carries a `design` field or `discuss:` notes, treat those as prior decisions and DO NOT re-ask them (GSD's prior-CONTEXT.md check).
  2. **Scout:** read the relevant source files the epic's description/acceptance implicates (host file tools) to ground options in code reality.
  3. **Derive gray areas:** phase-specific implementation decision areas (never generic categories) — e.g. library/config choices, API/CLI shape, data layout, UX copy. Present the derived list; let the user select which to discuss.
  4. **Per selected area:** up to ~4 single questions, one at a time, each offering 2–3 concrete options with a recommended choice + one-line rationale (host `AskUserQuestion` where available). Stop an area early when its decisions are locked.
  5. **Scope creep:** an idea beyond this epic's scope is captured as a deferred idea, and discussion redirects to the current epic's domain.
- **REWRITE** (beads is the brain — no files):
  - **Persist locked decisions** to the epic's design field:

    ```bash
    bd update <epic-id> --design "<structured decisions doc>"
    ```

    The design doc carries the bead-backed analog of GSD's CONTEXT.md sections, as markdown headings: `## Domain`, `## Decisions` (one locked decision per bullet: choice + rationale), `## Canonical refs` (file paths scouted), `## Specifics`, `## Deferred` (deferred ideas). If a design field already exists, merge — never silently overwrite prior locked decisions.
  - **Audit trail:** append `bd note <epic-id> "discuss: locked N decisions across M areas (<area list>)"` at session end.
  - **Consumer contract:** state explicitly that the per-phase planner (phase C) and executors read these decisions from the epic's design field; the skill's output is complete when the design field answers the HOW questions a planner would otherwise guess.
- **DROP:** `CONTEXT.md` / `.planning/` artifacts; `ROADMAP.md` reads; all GSD mode flags (`--auto`, `--batch`, `--analyze`, `--assumptions`, `--all`, `--power`, `--text`) — v1 ships the default adaptive mode only (YAGNI; note them as future overlays).
- **Stop conditions:** all selected areas discussed, or no meaningful gray areas exist (say so and exit without writing).

- [ ] **Step 4: Validate shape**

Run:

```bash
rg -q 'bd update.*--design' weft/workflows/discuss.md && rg -q 'bd show' weft/workflows/discuss.md && rg -q 'SPDX' weft/commands/weft-discuss.md weft/workflows/discuss.md && echo SHAPE-OK
! rg -i '\.planning|CONTEXT\.md|ROADMAP|STATE\.md|gsd-tools' weft/commands/weft-discuss.md weft/workflows/discuss.md && echo NO-LAYER-BC-REFS
```

Expected: `SHAPE-OK`; `NO-LAYER-BC-REFS`.

- [ ] **Step 5: Commit**

Run: `jj commit -m "docs(weft-ccy.1): /weft-discuss command + workflow — port of gsd-discuss-phase"`

---

### Task 2: `plugin/skills/discuss/SKILL.md` — collapse the pair (weft-ccy.1)

**Files:**

- Create: `plugin/skills/discuss/SKILL.md`

- [ ] **Step 1: Collapse** per ADR weft-88z: SKILL.md frontmatter = the command's `description` + `argument-hint`; body = the workflow body (drop the command's thin indirection).

- [ ] **Step 2: Rewrite intra-tree citations** (per the standing `weft-plugin-authoring-rewrite-intra-tree-path-citations` gotcha): `weft/references/<x>.md` → `${CLAUDE_PLUGIN_ROOT}/references/<x>.md`; `weft/agents/weft-<role>.md` → `${CLAUDE_PLUGIN_ROOT}/agents/<role>.md` (keep bare `weft-<role>` dispatch names in prose); `weft/workflows/<x>.md` self-references become section references within the SKILL.

- [ ] **Step 3: Validate**

Run:

```bash
! rg -nE 'weft/(agents|references|workflows)/' plugin/skills/discuss/ && echo DISCIPLINE-OK
claude plugin validate ./plugin --strict && claude plugin validate . --strict && echo VALIDATE-OK
```

Expected: `DISCIPLINE-OK`; `VALIDATE-OK` (both gates — the marketplace-path strict gate catches what the plugin-path gate misses).

- [ ] **Step 4: Commit**

Run: `jj commit -m "docs(weft-ccy.1): plugin discuss skill — collapsed pair per ADR weft-88z"`

---

### Task 3: `/weft-verify-work` command + workflow — port of `/gsd-verify-work` (weft-ccy.2)

**Files:**

- Create: `weft/commands/weft-verify-work.md`
- Create: `weft/workflows/verify-work.md`

- [ ] **Step 1: Fetch the source**

`deepwiki open-gsd/gsd-core` → "Full flow of /gsd-verify-work and workflows/verify-work.md: deliverable extraction from SUMMARY.md, the per-item y/n loop (empty = pass), UAT.md progress/resume, severity inference from user wording, parallel debug-agent diagnosis, gap-closure planning, and the end states (execute --gaps-only + re-verify; all-pass completion marking)."

- [ ] **Step 2: Author the command** (`weft/commands/weft-verify-work.md`)

Frontmatter:

```yaml
---
description: Interactive UAT over an epic's deliverables — walk each one y/n, diagnose failures, file fix picks under the epic.
argument-hint: "[epic-id]"
---
```

SPDX + provenance (`adapted from /gsd-verify-work (GSD Core, MIT)`). Thin: invokes `weft/workflows/verify-work.md`.

- [ ] **Step 3: Author the workflow** (`weft/workflows/verify-work.md`)

SPDX + provenance. Same house structure as Task 1 Step 3.

- **Inputs:** `epic-id` (required); same id-shape guard as discuss.
- **ADAPT** (GSD methodology, bead-backed):
  1. **Enumerate deliverables** via the spec §4 fallback chain: phase epic acceptance criteria → closed picks' acceptance criteria (`bd show` each closed child) → epic goal/description as last resort. Each deliverable is phrased as a user-observable "what should happen" checkpoint.
  2. **Per-item loop:** present the checkpoint, ask whether reality matches. `yes`/`y`/empty = pass → next. Any other response is recorded verbatim as an issue.
  3. **Severity inference** from the user's wording: crash/data-loss language → P1 (blocker); "doesn't work" → P2 (major); "looks off"/cosmetic → P3.
  4. **Diagnosis:** for each recorded issue, dispatch a fresh read-only diagnosis agent (host `Agent` tool, parallel across issues) to find the root cause; fold each root cause into the issue record.
  5. **Resume:** after every verdict, append `bd note <epic-id> "verify-work: <deliverable> — pass|FAIL(<severity>): <user words>"` so an interrupted session resumes by reading the notes and skipping already-passed items.
- **REWRITE** (beads + verbs, no files):
  - **File fix picks** for each diagnosed failure:

    ```bash
    bd create --parent <epic-id> --type=bug --priority=<severity-mapped> \
      --title "UAT fix: <deliverable>" \
      --description "<user words + diagnosed root cause>" \
      --acceptance "<the failed checkpoint, restated as the pass condition>" \
      --labels "uat-fix"
    ```

  - **End states:** if fix picks were filed → suggest `/weft-execute <epic-id>` (the skill takes a positional epic-id; the fix picks are the ready set) then re-run `/weft-verify-work <epic-id>`. If all deliverables passed → `bd note <epic-id> "verify-work: all N deliverables passed"` and suggest `weft finish open <epic-id>` (the stable surface is `finish open`/`finish reconcile` — bare `weft finish` is not a verb; this skill is the human gate before shipping and does not close the epic itself).
  - **Complement, never replace:** state explicitly that the machine `weft pick verify` gate (verdict-as-data, exit 0) already ran per pick during execute; this skill is the human UAT layer on top (spec §4, ADR weft-cfp rationale).
- **DROP:** `SUMMARY.md` extraction, `UAT.md` / `.planning/` state, ROADMAP/STATE completion marking, the gsd-planner/plan-checker gap-closure iteration loop (weft files fix picks directly; execute's own verify gate covers fix quality), `--gaps-only` flag (the fix picks ARE the epic's ready set).

- [ ] **Step 4: Validate shape**

Run:

```bash
rg -q 'bd create --parent' weft/workflows/verify-work.md && rg -q 'pick verify' weft/workflows/verify-work.md && rg -q 'uat-fix' weft/workflows/verify-work.md && rg -q 'SPDX' weft/commands/weft-verify-work.md weft/workflows/verify-work.md && echo SHAPE-OK
! rg -i '\.planning|UAT\.md|SUMMARY|ROADMAP|STATE\.md|gaps-only|gsd-tools' weft/commands/weft-verify-work.md weft/workflows/verify-work.md && echo NO-LAYER-BC-REFS
```

Expected: `SHAPE-OK`; `NO-LAYER-BC-REFS`.

- [ ] **Step 5: Commit**

Run: `jj commit -m "docs(weft-ccy.2): /weft-verify-work command + workflow — port of gsd-verify-work"`

---

### Task 4: `plugin/skills/verify-work/SKILL.md` — collapse the pair (weft-ccy.2)

**Files:**

- Create: `plugin/skills/verify-work/SKILL.md`

- [ ] **Step 1: Collapse** exactly as Task 2 Step 1 (frontmatter from the Task 3 command; body from `weft/workflows/verify-work.md`).

- [ ] **Step 2: Rewrite intra-tree citations** exactly as Task 2 Step 2.

- [ ] **Step 3: Validate**

Run:

```bash
! rg -nE 'weft/(agents|references|workflows)/' plugin/skills/verify-work/ && echo DISCIPLINE-OK
claude plugin validate ./plugin --strict && claude plugin validate . --strict && echo VALIDATE-OK
```

Expected: `DISCIPLINE-OK`; `VALIDATE-OK`.

- [ ] **Step 4: Commit**

Run: `jj commit -m "docs(weft-ccy.2): plugin verify-work skill — collapsed pair per ADR weft-88z"`

---

### Task 5: `plugin/README.md` skills list + full-tree sweep

**Files:**

- Modify: `plugin/README.md` (the `## Skills` section)

- [ ] **Step 1: Add the two skills** to the `## Skills` list, matching the existing entry style:

```markdown
- **`/weft:discuss <epic-id>`** — shape HOW a phase gets built: adaptive
  gray-area questions whose locked decisions land in the epic's bead design
  field for the planner to consume.
- **`/weft:verify-work <epic-id>`** — interactive UAT: walk the epic's
  deliverables y/n one at a time, diagnose failures, and file fix picks
  under the epic.
```

- [ ] **Step 2: Full discipline sweep**

Run:

```bash
! rg -nE 'weft/(agents|references|workflows)/' plugin/ && echo TREE-DISCIPLINE-OK
! rg -i '\.planning|gsd-tools|ROADMAP|STATE\.md|SUMMARY|CONTEXT\.md|UAT\.md' weft/commands/weft-discuss.md weft/workflows/discuss.md weft/commands/weft-verify-work.md weft/workflows/verify-work.md && echo TREE-CLEAN
claude plugin validate ./plugin --strict && claude plugin validate . --strict && echo VALIDATE-OK
go build -o /dev/null ./cmd/weft
```

Expected: `TREE-DISCIPLINE-OK`; `TREE-CLEAN`; `VALIDATE-OK`; build succeeds (prompt-only phase must not touch engine code).

- [ ] **Step 3: Commit**

Run: `jj commit -m "docs(weft-ccy): plugin README — list discuss + verify-work skills"`

---

### Task 6: ADR weft-cfp addendum — phasing is auto-discovered

**Files:**

- Modify: `docs/adr/weft-cfp-v1-interaction-model-front-loaded-spine-phased-interactive-g.md` (rendered — never edit manually; bd is the source of truth)

- [ ] **Step 1: File the addendum** via the `dev-flow:adr` skill, addendum mode, target `weft-cfp`, with this text:

> **Addendum (2026-06-09, spec `docs/superpowers/specs/2026-06-09-layer-a-interactive-phased-loop-design.md`):** the consequence "an opt-in `--phased`/`--interactive` mode can coexist with the current one-shot path" is superseded. Phasing is **auto-discovered** by the planner — there is no flag. The one-shot path survives behaviorally as the **single-phase degenerate case** (project epic = phase epic, picks planned immediately, today's epic+picks emission shape), not as a separate mode. Pick-level planning for multi-phase projects is just-in-time per phase, after that phase's `discuss`.

- [ ] **Step 2: Verify the render** — the ADR file shows the addendum and still carries its `adr-render: source=bd:weft-cfp` header.

Run: `rg -q 'Addendum' docs/adr/weft-cfp-*.md && echo ADDENDUM-RENDERED`
Expected: `ADDENDUM-RENDERED`.

- [ ] **Step 3: Commit**

Run: `jj commit -m "docs(weft-cfp): ADR addendum — auto-discovered phasing; one-shot = single-phase degenerate case"`

---

## Done criteria

- All four `weft/` files + both `plugin/skills/*/SKILL.md` exist with SPDX + GSD provenance; every shape check and the full-tree sweep pass.
- `claude plugin validate ./plugin --strict` AND `claude plugin validate . --strict` pass.
- ADR weft-cfp carries the addendum.
- `bd close weft-ccy.1 weft-ccy.2` (after review); `bd dolt push`.

## Out of scope (later phases per the spec)

- Planner consumption of the design field (phase C); GSD mode flags / `/gsd-ui-phase`; the phase-driver auto loop (phase E); engine changes (phase B).
