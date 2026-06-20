<!--
  ~ SPDX-License-Identifier: Apache-2.0
  ~ Copyright 2026 Weft Contributors
-->

# `sketch` + `ui-phase` Skills Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add the two remaining GSD pre-planning skills — `sketch` (throwaway visual/UI direction via a vendored browser companion → bead-native chosen-direction) and `ui-phase` (lock a UI contract via a researcher/checker agent pair → `decision` bead + epic `design` field) — and wire `plan-phase` to consume the contract behind a config-less UI safety gate.

**Architecture:** Purely prompt-layer + vendored scripts; no engine change. `sketch` and `ui-phase` are standalone, optional doors in the per-phase rhythm (`discuss → [sketch] → ui-phase → plan-phase → execute → verify-work → finish`). Outputs are bead-native — no `.planning/` files. The visual companion is `obra/superpowers`' zero-dependency Node server, **vendored** into weft (self-contained; not a `dev-flow` dependency) and **rebranded** to weft (`.weft/` scratch dir + chrome), MIT attribution preserved.

**Tech Stack:** Claude Code plugin skills + agents (markdown + YAML frontmatter); `bd` (`bd remember`, `bd update --design`, `bd create --type=decision`, `bd dep add --type related`, `bd note`); the vendored Node companion (`server.cjs` + shell + HTML/JS, Node built-ins only, `node` on `PATH`); the Task tool for the two new agents.

**Spec:** `docs/superpowers/specs/2026-06-20-sketch-ui-phase-design.md`

**Design bead:** `weft-ccy.10`

---

## Grounding notes (verified against the repo at plan time)

- **Files touched.** New creates: `plugin/skills/sketch/SKILL.md`,
  `plugin/skills/ui-phase/SKILL.md`, `plugin/agents/ui-researcher.md`,
  `plugin/agents/ui-checker.md`, `plugin/skills/sketch/scripts/visual-companion/*`
  (vendored). Modify: `plugin/skills/plan-phase/SKILL.md`, `.gitignore`. Parent
  `plugin/skills/` and `plugin/agents/` both exist (siblings: `discuss`,
  `execute`, `explore`, `feature`, `new-project`, `onboard`, `phase-driver`,
  `plan-phase`, `spike`, `verify-work`; agents `planner`, `reviewer`, `executor`,
  `resolver`).
- **Skills/agents auto-discovered** — `plugin/.claude-plugin/plugin.json` does
  not enumerate them; creating the files registers them. README index is
  decoupled (newest skills are absent from it); this plan does not touch
  `plugin/README.md`.
- **Agents are `plugin/agents/` ONLY — NOT dual-tree (resolved open item #1).**
  The `weft/` tree is legacy: root `.claude-plugin/marketplace.json` declares a
  single plugin `source: ./plugin`, so `claude plugin validate . --strict`
  validates the *marketplace → ./plugin*, not `weft/`; the CI grep-discipline
  comment states "the weft/ tree does not exist in the plugin cache";
  `weft/agents/weft-planner.md` is stale (seam-9, 2026-06-07) vs
  `plugin/agents/planner.md` (ccy.3, 2026-06-11); nothing references
  `weft/agents`. Do **not** create `weft/agents/weft-*.md`.
- **Agent reference idiom (verified live):** skills dispatch agents by the path
  `${CLAUDE_PLUGIN_ROOT}/agents/<name>.md` (e.g. `execute`/`new-project`/`plan-phase`),
  and the agent's `name:` frontmatter carries the `weft-` prefix (`weft-planner`,
  `weft-reviewer`). New agents: files `plugin/agents/ui-researcher.md` +
  `ui-checker.md`, `name: weft-ui-researcher` / `weft-ui-checker`.
- **Agent models (resolved open item #2):** `weft-ui-researcher` `model: opus`
  (interactive design reasoning, like `weft-planner`); `weft-ui-checker`
  `model: sonnet` (bounded validation pass).
- **Vendor path (resolved open item #3):**
  `plugin/skills/sketch/scripts/visual-companion/` (a `scripts/` dir under the
  skill, mirroring how `dev-flow`'s brainstorming skill ships the same server and
  passes `claude plugin validate --strict`). `ui-phase` references it via
  `${CLAUDE_PLUGIN_ROOT}/skills/sketch/scripts/visual-companion/`.
- **Companion source (resolved open item #4):** vendor the five files from the
  in-environment `dev-flow` brainstorming copy (resolved by `find`, version-hash
  agnostic); they originate from `obra/superpowers` (MIT). NOTICE attributes
  `obra/superpowers`; UPSTREAM records both the proximate `dev-flow` copy and the
  `obra/superpowers` commit (implementer pins the current upstream `main` commit
  at vendor time).
- **Companion rebrand sites (verified in the source):**
  `start-server.sh` (`.superpowers/brainstorm/` path + `--project-dir` help text +
  `/tmp/brainstorm-` fallback), `server.cjs` (`/tmp/brainstorm` default + inline
  `Brainstorm Companion` HTML titles), `frame-template.html` (`<title>Superpowers
  Brainstorming</title>` line 5 + `<h1>…Superpowers Brainstorming…</h1>` line 199),
  `stop-server.sh` (comments). Internal `BRAINSTORM_*` env var names are **not**
  rebranded (not user-facing; renaming them only widens upstream drift — Sean's
  rebrand scope is dir + visible chrome).
- **`.gitignore`:** already has `.superpowers/`, `tmp/`, `.tmp/`; add `.weft/`.
  Leave `.superpowers/` (covers `dev-flow` brainstorming run in this repo).
- **plan-phase shape (verified):** `## Phase 1 — Load phase context` (reads
  `bd show`, `description`/`acceptance`/`design`), then `## Phase 2 — Dispatch
  weft-planner`. The gate inserts between them; the planner-context extension
  augments Phase 2. The existing "empty design field is valid input" language is
  the template for the non-blocking tone.
- **bd flags (verified live, via explore/spike + prior grounding):** `bd remember
  "…" --key <key>`; `bd update <id> --design "…"`; `bd create --type=decision
  --labels <l> --title … --description …/--stdin --json` (`.id`); `bd dep add
  <a> <b> --type related` (epics can only be *blocked* by epics, so ADR/spec
  traceability edges use `--type related`); `bd note <id>`. `--labels` is plural.
- **No automated unit tests apply** to markdown prompts / vendored scripts. The
  per-task gate is the CI discipline run locally: `claude plugin validate ./plugin
  --strict`, `claude plugin validate . --strict`, and
  `grep -RnE 'weft/(agents|references|workflows)/' plugin/` returning no matches.
  Task 1 adds a one-shot companion smoke test; Task 6 is the end-to-end dogfood.

---

## File Structure

| File | Responsibility | Action |
|---|---|---|
| `plugin/skills/sketch/scripts/visual-companion/{server.cjs,start-server.sh,stop-server.sh,frame-template.html,helper.js}` | Vendored zero-dep browser companion (serves mockup fragments, captures click selections), rebranded to weft. | Create (vendor) |
| `plugin/skills/sketch/scripts/visual-companion/NOTICE` | MIT attribution for the vendored companion. | Create |
| `plugin/skills/sketch/scripts/visual-companion/UPSTREAM.md` | Provenance: proximate `dev-flow` copy + `obra/superpowers` commit + enumerated weft modifications. | Create |
| `.gitignore` | Ignore the companion's transient scratch dir. | Modify (add `.weft/`) |
| `plugin/skills/sketch/SKILL.md` | `sketch` skill — visual direction via the companion → `bd remember` + epic `design` field. | Create |
| `plugin/agents/ui-researcher.md` | `weft-ui-researcher` — drafts the UI contract, asking only unanswered design questions. | Create |
| `plugin/agents/ui-checker.md` | `weft-ui-checker` — validates a draft contract across six dimensions. | Create |
| `plugin/skills/ui-phase/SKILL.md` | `ui-phase` skill — researcher/checker loop → `ui-spec` `decision` bead + epic `design` field. | Create |
| `plugin/skills/plan-phase/SKILL.md` | Add a config-less UI safety gate + pass the UI contract to the planner. | Modify |

---

## Task 1: Vendor + rebrand the visual companion

**Files:**

- Create: `plugin/skills/sketch/scripts/visual-companion/{server.cjs,start-server.sh,stop-server.sh,frame-template.html,helper.js}`
- Create: `plugin/skills/sketch/scripts/visual-companion/NOTICE`, `…/UPSTREAM.md`
- Modify: `.gitignore`

- [ ] **Step 1: Locate the source copy (version-hash agnostic)**

```bash
SRC="$(dirname "$(find "$HOME/.claude/plugins/cache" -path '*dev-flow*/skills/brainstorming/scripts/server.cjs' 2>/dev/null | head -1)")"
test -n "$SRC" && ls "$SRC"/{server.cjs,start-server.sh,stop-server.sh,frame-template.html,helper.js}
```

Expected: the five files listed. If `$SRC` is empty (dev-flow not installed locally), fetch the five files from `https://github.com/obra/superpowers` at `skills/brainstorming/scripts/` on `main` instead, and record that commit in Step 6.

- [ ] **Step 2: Copy the five files verbatim into the vendor dir**

```bash
DEST="plugin/skills/sketch/scripts/visual-companion"
mkdir -p "$DEST"
cp "$SRC"/{server.cjs,start-server.sh,stop-server.sh,frame-template.html,helper.js} "$DEST/"
chmod +x "$DEST/start-server.sh" "$DEST/stop-server.sh"
```

Do **not** add weft's Apache SPDX header to these files — they keep their MIT
headers (there is no CI header gate; see Step 6).

- [ ] **Step 3: Rebrand the scratch dir `.superpowers/brainstorm` → `.weft/sketch` and `/tmp/brainstorm` → `/tmp/weft-sketch`**

Apply these exact string replacements (paths are user-facing scratch; internal
`BRAINSTORM_*` env var names are intentionally left unchanged):

In `plugin/skills/sketch/scripts/visual-companion/start-server.sh`:

```
#   --project-dir <path>  Store session files under <path>/.superpowers/brainstorm/
→  #   --project-dir <path>  Store session files under <path>/.weft/sketch/

  SESSION_DIR="${PROJECT_DIR}/.superpowers/brainstorm/${SESSION_ID}"
→  SESSION_DIR="${PROJECT_DIR}/.weft/sketch/${SESSION_ID}"

  SESSION_DIR="/tmp/brainstorm-${SESSION_ID}"
→  SESSION_DIR="/tmp/weft-sketch-${SESSION_ID}"
```

In `plugin/skills/sketch/scripts/visual-companion/server.cjs`:

```
const SESSION_DIR = process.env.BRAINSTORM_DIR || '/tmp/brainstorm';
→  const SESSION_DIR = process.env.BRAINSTORM_DIR || '/tmp/weft-sketch';
```

In `plugin/skills/sketch/scripts/visual-companion/stop-server.sh`:

```
# under /tmp (ephemeral). Persistent directories (.superpowers/) are
→  # under /tmp (ephemeral). Persistent directories (.weft/) are
```

- [ ] **Step 4: Rebrand the visible chrome → weft**

In `plugin/skills/sketch/scripts/visual-companion/frame-template.html`:

```
  <title>Superpowers Brainstorming</title>
→  <title>weft · sketch</title>

    <h1><a href="https://github.com/obra/superpowers" style="color: inherit; text-decoration: none;">Superpowers Brainstorming</a></h1>
→  <h1>weft · sketch</h1>
```

In `plugin/skills/sketch/scripts/visual-companion/server.cjs` (the inline
no-content placeholder page, two occurrences of `Brainstorm Companion`):

```
<head><meta charset="utf-8"><title>Brainstorm Companion</title>
→  <head><meta charset="utf-8"><title>weft · sketch</title>

<body><h1>Brainstorm Companion</h1>
→  <body><h1>weft · sketch</h1>
```

- [ ] **Step 5: Add `.weft/` to `.gitignore`**

Add the line `.weft/` to `.gitignore` (immediately after the existing
`.superpowers/` entry). Leave `.superpowers/` in place.

- [ ] **Step 6: Write NOTICE + UPSTREAM provenance**

Create `plugin/skills/sketch/scripts/visual-companion/NOTICE`:

```
weft vendors the visual companion server in this directory from
Superpowers (https://github.com/obra/superpowers), used under the MIT License.

  MIT License — Copyright (c) Jesse Vincent and Superpowers contributors.

The vendored files (server.cjs, start-server.sh, stop-server.sh,
frame-template.html, helper.js) retain their original MIT license headers and
are NOT relicensed. weft itself is Apache-2.0; this directory is the sole MIT
exception. See UPSTREAM.md for the source commit and the list of weft
modifications.
```

Create `plugin/skills/sketch/scripts/visual-companion/UPSTREAM.md`:

```markdown
# Vendored: Superpowers visual companion

- **Upstream:** https://github.com/obra/superpowers — `skills/brainstorming/scripts/`
- **Upstream commit:** <pin the obra/superpowers main commit SHA at vendor time>
- **Proximate source:** the dev-flow plugin's brainstorming/scripts/ copy present
  in this environment (`<record the dev-flow plugin version>`).
- **License:** MIT (retained; see NOTICE).

## weft modifications

- Scratch dir rebranded: `.superpowers/brainstorm/` → `.weft/sketch/`,
  `/tmp/brainstorm[-id]` → `/tmp/weft-sketch[-id]` (start-server.sh, server.cjs,
  stop-server.sh).
- Visible chrome rebranded to `weft · sketch` (frame-template.html title + h1;
  server.cjs placeholder page title + h1).
- Internal `BRAINSTORM_*` env var names left unchanged (not user-facing).
- No functional/protocol changes.
```

- [ ] **Step 7: Smoke-test the vendored server**

```bash
cd plugin/skills/sketch/scripts/visual-companion
node --check server.cjs && echo "server.cjs parses OK"   # syntax check
# Functional smoke: launch, serve a fragment, confirm scratch + event plumbing, stop
OUT="$(./start-server.sh --project-dir "$(jj root)" 2>&1)"; echo "$OUT"
SDIR="$(printf '%s' "$OUT" | sed -n 's/.*"state_dir":"\([^"]*\)".*/\1/p')"
SCREEN="$(printf '%s' "$OUT" | sed -n 's/.*"screen_dir":"\([^"]*\)".*/\1/p')"
printf '<h2>smoke</h2>' > "$SCREEN/smoke.html"
test -d "$(jj root)/.weft/sketch" && echo "scratch under .weft/sketch OK"
./stop-server.sh "$(dirname "$SDIR")" 2>&1 | tail -1
```

Expected: startup JSON with `port`/`url`/`screen_dir`/`state_dir`; the scratch
dir is created under `.weft/sketch/` (NOT `.superpowers/`); stop succeeds. If
`node` is absent, record the degraded-mode dependency and skip the functional
legs (the sketch skill's fallback covers it).

- [ ] **Step 8: Validate + confirm working copy stays clean**

```bash
claude plugin validate ./plugin --strict && claude plugin validate . --strict
grep -RnE 'weft/(agents|references|workflows)/' plugin/ && echo "FAIL: stale path" || echo "grep-discipline OK"
jj --no-pager status   # .weft/ must NOT appear (gitignored); only vendored files + .gitignore tracked
```

Expected: both validates pass; grep-discipline OK; `jj status` shows the new
vendored files + `.gitignore`, and **no** `.weft/` entries.

- [ ] **Step 9: Commit**

```bash
jj commit -m "feat(weft-ccy.10): vendor visual companion (MIT), rebranded to weft

Vendored obra/superpowers brainstorming server into
plugin/skills/sketch/scripts/visual-companion/ (MIT retained, NOTICE+UPSTREAM);
scratch dir + chrome rebranded to weft; .gitignore gains .weft/.

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

## Task 2: `sketch` skill

**Files:**

- Create: `plugin/skills/sketch/SKILL.md`

- [ ] **Step 1: Create the skill file**

Create `plugin/skills/sketch/SKILL.md` with exactly this content:

````markdown
---
description: Throwaway visual/UI direction — explore 2–4 HTML mockup variants in a live browser companion (side-by-side, click-to-select), capture the chosen direction as bead-native state. Pre-planning; hands off to ui-phase/plan-phase.
argument-hint: "[topic] [epic-id]"
---

<!--
  ~ SPDX-License-Identifier: Apache-2.0
  ~ Copyright 2026 Weft Contributors
-->

<!-- adapted from /gsd-sketch (GSD Core, MIT); visual companion vendored from obra/superpowers (MIT) -->

# sketch workflow

weft's analog of `/gsd-sketch`: throwaway exploration of **visual/UI direction**
before a frontend phase is planned. Mockups are disposable — they live only in
the gitignored `.weft/sketch/` scratch dir (or `/tmp`), never committed. The
durable output is the **chosen direction**, recorded bead-native as a
`bd remember` finding (+ the epic `design` field), the weft analog of GSD's
`sketch-findings` skill. No `.planning/` files.

Mockups render in a **live browser companion** (vendored, zero-dependency Node
server) supporting side-by-side comparison and click-to-select.

> `sketch` is distinct from `explore` (WHAT to build) and `discuss` (HOW to build
> it). It shapes how a UI surface should *look*. It is an optional door, run
> before `ui-phase`.
>
> Runtime: the companion needs `node` on `PATH`. Without it, fall back to static
> HTML in `/tmp` (Phase 2 degraded path).

---

## Phase 1 — Frame the surface

Treat `[topic]` as the UI surface to explore (a screen, component, or flow). If
unclear, one light round of framing: which surface, its key elements, and how
many variants to show (**2–4**). State the variants to the user before
generating them.

---

## Phase 2 — Launch the visual companion

```
COMP="${CLAUDE_PLUGIN_ROOT}/skills/sketch/scripts/visual-companion"
command -v node >/dev/null || echo "degraded: no node — using static HTML fallback"
"$COMP/start-server.sh" --project-dir "$(jj root)"
```

Launch the server **in the background** (it must survive across turns — use the
Bash tool's `run_in_background` on platforms that reap detached processes, then
read `$STATE_DIR/server-info` next turn). Capture `url`, `screen_dir`, and
`state_dir` from the startup JSON. Tell the user to open the URL. The scratch
dir lands under the gitignored `.weft/sketch/<session>/` — never committed.

**Degraded (no `node`):** write each variant as a self-contained HTML file under
`/tmp/weft-sketch-<id>/` and give the user `file://` paths to open. Then skip to
Phase 4 (capture from the user's terminal reply).

---

## Phase 3 — Push variants + iterate

Write each variant as a **content fragment** (no `<html>`/`<head>` — the server
wraps it in the weft frame template) to a fresh, semantically-named file in
`screen_dir`, using the **Write tool** (never `cat`/heredoc). Companion classes:

- `.split` — side-by-side mockups; `.cards` — labelled design cards.
- `.mockup` / `.mockup-header` / `.mockup-body` — a framed preview.
- Wireframe blocks: `.mock-nav`, `.mock-sidebar`, `.mock-content`,
  `.mock-button`, `.mock-input`, `.placeholder`.
- `.options` (add `data-multiselect` to allow multiple) with
  `data-choice` + `onclick="toggleSelect(this)"` for A/B/C choices.

Show **2–4 options max** per screen and state the question on each ("Which layout
reads more clearly?"). End your turn: remind the user of the URL, summarise
what's on screen, and ask them to click a choice and/or reply in the terminal.

Next turn: read `$STATE_DIR/events` (JSONL of clicks; absent → no browser
interaction) **and** the user's terminal text. Iterate with a new file
(`layout-v2.html`) if feedback changes the screen. When moving back to the
terminal, push a `waiting.html` fragment to clear stale content.

---

## Phase 4 — Capture the chosen direction

From the selection events + terminal text, record the chosen direction —
**layout, color palette, typography, spacing**. Persist bead-native:

```
bd remember "Sketch — <surface>: chosen direction — layout: <…>; palette: <…>; typography: <…>; spacing: <…>." --key sketch-<slug>
```

If `[epic-id]` was given, also fold it into the phase epic so `ui-phase` /
`plan-phase` see it the same session (the `design`-field handoff, ADR `weft-b19`):

```
bd update <epic-id> --design "<existing design + the chosen sketch direction>"
```

Do not invent decisions the exploration did not produce. No `.planning/` files;
no committed mockups.

---

## Phase 5 — Stop + hand off

```
"$COMP/stop-server.sh" "$SESSION_DIR"
```

Mockups in `.weft/sketch/` are transient (gitignored; `/tmp` sessions are
auto-deleted). Point onward:

- Lock the full UI contract → `ui-phase <epic-id>`.
- Or go straight to planning → `plan-phase <epic-id>`.

---

## What this workflow does NOT do

- It does not keep or commit mockups — they are throwaway (gitignored `.weft/`
  or `/tmp`).
- It does not write `.planning/` or sidecar durable state — the durable output
  is `bd remember` + the epic `design` field.
- It does not run a build or framework — variants are self-contained HTML the
  companion serves.
- It does not lock a UI contract — that is `ui-phase`. `sketch` only explores
  visual direction.
````

- [ ] **Step 2: Validate**

```bash
claude plugin validate ./plugin --strict && claude plugin validate . --strict
grep -RnE 'weft/(agents|references|workflows)/' plugin/ && echo "FAIL" || echo "grep-discipline OK"
```

Expected: both validates pass; grep-discipline OK.

- [ ] **Step 3: Commit**

```bash
jj commit -m "feat(weft-ccy.10): sketch skill — visual direction via companion

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

## Task 3: `weft-ui-researcher` + `weft-ui-checker` agents

**Files:**

- Create: `plugin/agents/ui-researcher.md`
- Create: `plugin/agents/ui-checker.md`

- [ ] **Step 1: Create `plugin/agents/ui-researcher.md`**

Create the file with exactly this content:

````markdown
---
name: weft-ui-researcher
description: Drafts a UI contract for one phase — reads phase context + sketch findings + detected design-system state, asks only UNANSWERED design questions across spacing/color/typography/copywriting/registry-safety, emits a structured contract draft. Dispatched by the ui-phase skill.
model: opus
---

<!--
  ~ SPDX-License-Identifier: Apache-2.0
  ~ Copyright 2026 Weft Contributors
-->

<!-- adapted from gsd-ui-researcher.md (GSD Core, MIT), rewritten to bead-backed state -->

# weft-ui-researcher

You are weft's UI research agent. Given one phase's context, you produce a
**draft UI contract** — the locked design decisions a planner needs so generated
picks reference consistent tokens. You write no files; your output is the draft
contract text, returned to the `ui-phase` skill (which persists it as a
`decision` bead + the epic `design` field). beads is the brain — no `.planning/`
files, no `UI-SPEC.md`.

## Inputs (provided in your prompt)

- The phase epic's `description` (mini-brief), `acceptance`, and `design` field
  (locked HOW decisions from `discuss`).
- Any **sketch finding** (`bd remember` `sketch-*` content / `design`-field
  direction): layout, palette, typography, spacing already chosen. **Treat these
  as settled — do not re-ask them.**
- Detected **design-system state**: presence of `components.json` (shadcn),
  Tailwind config, existing design tokens, the frontend framework.

## Method

Ask only **UNANSWERED** questions across these five areas, one focused round each
where a real gap exists (skip an area fully answered by the sketch finding or the
existing design system):

1. **Spacing** — scale/rhythm, density.
2. **Color** — palette, semantic roles, dark/light, contrast.
3. **Typography** — families, scale, weights.
4. **Copywriting** — voice/tone, key labels, empty/error states.
5. **Registry-safety** — reuse existing components/tokens vs introduce new;
   avoid duplicating or forking the design system.

Prefer recommended-choice questions (2–3 options) like `discuss`. Do not
re-litigate decisions already locked by the sketch finding or design system.

## Output contract

Return a single **draft UI contract** with one labelled section per area
(Spacing / Color / Typography / Copywriting / Registry-safety), each stating the
**locked decision** (concrete tokens/values where they exist) and citing its
source (sketch finding, existing design system, or this session's answer). This
draft is what `weft-ui-checker` validates and what `ui-phase` persists.
````

- [ ] **Step 2: Create `plugin/agents/ui-checker.md`**

Create the file with exactly this content:

````markdown
---
name: weft-ui-checker
description: Validates a draft UI contract across six dimensions (copywriting, visuals, color, typography, spacing, registry-safety) and returns a structured PASS/ISSUES verdict that drives the ui-phase revision loop. Dispatched by the ui-phase skill.
model: sonnet
---

<!--
  ~ SPDX-License-Identifier: Apache-2.0
  ~ Copyright 2026 Weft Contributors
-->

<!-- adapted from gsd-ui-checker.md (GSD Core, MIT), rewritten to bead-backed state -->

# weft-ui-checker

You are weft's UI validation agent. Given a **draft UI contract** (from
`weft-ui-researcher`) and the phase context, you check the draft for
completeness and internal consistency and return a verdict. You write no files
and ask the user nothing — you are a bounded, read-only validation pass.

## Validate across six dimensions

1. **Copywriting** — voice consistent; key labels and empty/error states defined.
2. **Visuals** — layout/hierarchy coherent; no unspecified surfaces.
3. **Color** — palette complete; semantic roles assigned; contrast adequate.
4. **Typography** — families/scale/weights specified and consistent.
5. **Spacing** — scale defined and applied consistently.
6. **Registry-safety** — reuses existing components/tokens; no accidental
   duplication or fork of the design system; new additions justified.

## Output contract

Return a verdict whose **first line** is exactly `VERDICT: PASS` or
`VERDICT: ISSUES`. On `ISSUES`, follow with a terse, per-dimension list of the
specific gaps to fix (each actionable). The `ui-phase` skill re-runs
`weft-ui-researcher` on the flagged items, at most twice.
````

- [ ] **Step 3: Validate**

```bash
claude plugin validate ./plugin --strict && claude plugin validate . --strict
```

Expected: both pass (the two new agents are discovered).

- [ ] **Step 4: Commit**

```bash
jj commit -m "feat(weft-ccy.10): weft-ui-researcher + weft-ui-checker agents

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

## Task 4: `ui-phase` skill

**Files:**

- Create: `plugin/skills/ui-phase/SKILL.md`

- [ ] **Step 1: Create the skill file**

Create `plugin/skills/ui-phase/SKILL.md` with exactly this content:

````markdown
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
````

- [ ] **Step 2: Validate**

```bash
claude plugin validate ./plugin --strict && claude plugin validate . --strict
grep -RnE 'weft/(agents|references|workflows)/' plugin/ && echo "FAIL" || echo "grep-discipline OK"
```

Expected: both validates pass; grep-discipline OK.

- [ ] **Step 3: Commit**

```bash
jj commit -m "feat(weft-ccy.10): ui-phase skill — lock UI contract before planning

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

## Task 5: `plan-phase` UI safety gate + planner context

**Files:**

- Modify: `plugin/skills/plan-phase/SKILL.md`

- [ ] **Step 1: Insert the UI safety gate after Phase 1**

In `plugin/skills/plan-phase/SKILL.md`, the Phase 1 section ends with the
"empty or absent `design` field is valid input" paragraph, followed by a `---`
separator, then `## Phase 2 — Dispatch weft-planner`. Insert this new section
**between that `---` and `## Phase 2`** (final order: Phase 1 → `---` → Phase 1.5
→ `---` → Phase 2), verbatim:

````markdown
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
````

- [ ] **Step 2: Extend the Phase 2 planner dispatch to pass the UI contract**

In `## Phase 2 — Dispatch weft-planner`, the bulleted dispatch context currently
ends with:

```
- the locked HOW decisions from the `design` field (if any).
```

Add one bullet immediately after it:

```
- the locked **UI contract** (if any) — the `ui-spec` `decision` bead / `design`-field
  contract from `ui-phase` — so picks reference the locked spacing tokens, color
  variables, typography, and copywriting decisions.
```

- [ ] **Step 3: Validate**

```bash
claude plugin validate ./plugin --strict && claude plugin validate . --strict
grep -RnE 'weft/(agents|references|workflows)/' plugin/ && echo "FAIL" || echo "grep-discipline OK"
```

Expected: both validates pass; grep-discipline OK.

- [ ] **Step 4: Commit**

```bash
jj commit -m "feat(weft-ccy.10): plan-phase UI safety gate + planner UI context

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

## Task 6: Validation (dogfood gate)

End-to-end exercise of the new surface on a throwaway frontend scenario, mirroring
the ccy.4 dogfood that caught real bugs. This task creates and then deletes its
own test beads; it commits nothing but the evidence captured in the bead note.

- [ ] **Step 1: Plugin gates (whole tree)**

```bash
claude plugin validate ./plugin --strict
claude plugin validate . --strict
grep -RnE 'weft/(agents|references|workflows)/' plugin/ && echo "FAIL" || echo "grep-discipline OK"
# Provenance tokens must be filled (no unresolved <… at vendor time> / <record …> in NOTICE/UPSTREAM):
grep -REn '<[^>]*(at vendor time|record )[^>]*>' plugin/skills/sketch/scripts/visual-companion/ && echo "FAIL: unfilled provenance tokens" || echo "provenance filled OK"
```

Expected: both validates pass; grep-discipline OK; provenance filled OK (Task 1
Step 6's UPSTREAM commit SHA + dev-flow version were pinned, not left as `<…>`).

- [ ] **Step 2: Companion + `.weft/` containment**

```bash
COMP="plugin/skills/sketch/scripts/visual-companion"
OUT="$("$COMP/start-server.sh" --project-dir "$(jj root)" 2>&1)"; echo "$OUT"
SDIR="$(printf '%s' "$OUT" | sed -n 's/.*"state_dir":"\([^"]*\)".*/\1/p')"
SCREEN="$(printf '%s' "$OUT" | sed -n 's/.*"screen_dir":"\([^"]*\)".*/\1/p')"
printf '<div class="split"><div class="mockup">A</div><div class="mockup">B</div></div>' > "$SCREEN/variants.html"
echo "$SCREEN" | grep -q '/.weft/sketch/' && echo "scratch under .weft/sketch OK"
"$COMP/stop-server.sh" "$(dirname "$SDIR")" 2>&1 | tail -1
jj --no-pager status | grep -q '\.weft/' && echo "FAIL: .weft tracked" || echo ".weft gitignored OK"
```

Expected: server serves the fragment; scratch is under `.weft/sketch/`; stop
succeeds; `jj status` shows no `.weft/` (gitignored). (If `node` is absent,
record the degraded path instead and verify the sketch skill's `/tmp` fallback
wording.)

- [ ] **Step 3: Dry-run the bead-native persistence (sketch + ui-phase shapes)**

```bash
# sketch finding shape
bd remember "Sketch — dogfood surface: chosen direction — layout: single-column; palette: neutral; typography: system; spacing: 8px scale." --key sketch-dogfood
bd memories sketch | grep -q sketch-dogfood && echo "sketch finding OK"

# ui-phase contract shape (throwaway epic + decision bead + related edge)
EP=$(bd create --type=epic --title "dogfood ui-phase epic" --description "throwaway" --json | jq -r .id)
DEC=$(bd create --type=decision --labels ui-spec --title "UI contract: dogfood" --description "spacing 8px; color neutral; type system; copy plain; reuse shadcn." --json | jq -r .id)
bd dep add "$EP" "$DEC" --type related && echo "related edge OK"
bd update "$EP" --design "UI contract: spacing 8px; reuse shadcn." && echo "design-field OK"
bd show "$EP" | grep -qi 'ui contract' && echo "design-field readback OK"
```

Expected: each `OK` prints; the `--type related` edge is accepted (the default
blocking edge would error "epics can only block other epics").

- [ ] **Step 4: Confirm the plan-phase gate logic reads coherently**

Re-read `plugin/skills/plan-phase/SKILL.md` Phase 1.5: confirm the gate fires on
the frontend signals, is explicitly skippable, and stays silent when a `ui-spec`
bead / `design`-field contract is present. (Prompt-layer logic check — no
automated assertion.)

- [ ] **Step 5: Clean up dogfood beads + record evidence**

```bash
bd delete "$DEC" "$EP" --force
bd forget sketch-dogfood
bd note weft-ccy.10 "dogfood <date>: plugin validate strict x2 + grep-discipline clean; companion serves fragments, scratch contained to gitignored .weft/sketch (working copy clean); sketch finding (bd remember) + ui-phase contract (decision bead ui-spec + related edge + design-field) round-trip; --type related required (default blocking edge errors on epic<-decision); plan-phase Phase 1.5 gate skippable + silent when contract present. <record any bug caught+fixed, ccy.4-style>."
```

Expected: dogfood beads removed; evidence note on `weft-ccy.10`. Record any bug
caught and fixed inline (the ccy.4 dogfood caught a missing `--title`; expect
similar surprises and fix them in the owning task before this gate passes).

---

## Notes for the implementer

- **Order matters:** Task 1 (companion) precedes Task 2 (sketch references it);
  Task 3 (agents) precedes Task 4 (ui-phase dispatches them). Task 5 (plan-phase)
  is independent. Task 6 needs all.
- **No engine/Go changes** — if you find yourself editing `internal/` or `cmd/`,
  stop; this work is prompt-layer + the vendored MIT scripts only.
- **jj agent-safety:** `--no-pager` on every `jj` command; `--git` on diffs.
- **Do not add weft's Apache SPDX header** to the vendored
  `visual-companion/` files — they keep their MIT headers (NOTICE + UPSTREAM
  carry the attribution).
<!-- adr-capture: sha256=24fc1eacf1f530b5; session=cli; ts=2026-06-20T15:15:47Z; adrs=weft-n8f,weft-ngf,weft-tub,weft-odp -->
