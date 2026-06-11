<!--
  ~ SPDX-License-Identifier: Apache-2.0
  ~ Copyright 2026 Weft Contributors
-->

# `onboard` Skill Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add an `onboard` skill that makes an existing non-weft repo weft-ready (`bd init` + one codebase-mapping pass seeding `bd remember` memories, then route to `feature`/`new-project`), and bundle the surgical `feature` precondition fix that closes the routing triad.

**Architecture:** Purely prompt-layer. `onboard` is a thin composition: `bd init` → one `Explore` recon pass over GSD's four axes → seed `bd remember` entries (surfaced by the existing beads `bd prime` hook) → handoff. No Go change.

**Tech Stack:** Claude Code plugin skills (markdown + YAML frontmatter); `bd` (beads — `bd init`, `bd remember`, `bd prime`); the `Explore` subagent; jj for VCS.

**Spec:** `docs/superpowers/specs/2026-06-11-onboard-skill-design.md`

**Design bead:** `weft-ccy.7`

---

## Grounding notes (verified against the repo)

- **Files touched:** `plugin/skills/onboard/SKILL.md` is the one new-create (parent `plugin/skills/` exists; siblings `discuss/`, `execute/`, `feature/`, `new-project/`, `plan-phase/`, `verify-work/`). `plugin/skills/feature/SKILL.md` exists; its Phase 0 weft-managed sentence is at line 34.
- **`bd init` flags confirmed** (`bd init --help`): `--non-interactive` (skip prompts; auto in CI/non-TTY) and `-p/--prefix` (default: current directory name). A fresh repo has no prior `.beads/config.yaml`, so `bd init` creates a clean local-only DB — the "bd init fails if config.yaml declares a remote" gotcha does not apply to onboarding an unmanaged repo.
- **`bd remember` confirmed** (`bd remember --help`): `bd remember "<text>" [--key <key>]`; memories are "injected at prime time (bd prime)". Verified live: this session's startup surfaced "Persistent Memories (N)" from `bd remember`. So seeded memories surface every future session with no new hook.
- **No automated unit tests apply** — markdown prompts. Per-task gate is the CI discipline run locally: `claude plugin validate ./plugin --strict`, `claude plugin validate . --strict`, and `grep -RnE 'weft/(agents|references|workflows)/' plugin/` returning no matches (use `${CLAUDE_PLUGIN_ROOT}` for any intra-plugin path).
- **Skills auto-discovered** — `plugin/.claude-plugin/plugin.json` does not enumerate skills; creating `plugin/skills/onboard/SKILL.md` registers it.

---

## File Structure

| File | Responsibility | Action |
|---|---|---|
| `plugin/skills/onboard/SKILL.md` | The third door: precondition → `bd init` → one Explore map pass → seed `bd remember` → route to `feature`/`new-project`. | Create |
| `plugin/skills/feature/SKILL.md` | Surgical Phase 0 fix: delete the empty-warp→`new-project` branch so weft-managed = `.beads/`-present. | Modify |

---

## Task 1: onboard skill

**Files:**

- Create: `plugin/skills/onboard/SKILL.md`

- [ ] **Step 1: Create the skill file**

Create `plugin/skills/onboard/SKILL.md` with exactly this content:

````markdown
---
description: Make an existing non-weft repo weft-ready — bd init, one codebase-mapping pass seeding bead memories, then route to feature or new-project. No planning of its own.
argument-hint: ""
---

<!--
  ~ SPDX-License-Identifier: Apache-2.0
  ~ Copyright 2026 Weft Contributors
-->

<!-- adapted from /gsd-map-codebase (GSD Core, MIT), compressed to one pass + bead-backed memories -->

# onboard workflow

The third door: make an existing repo that has never run weft **weft-ready**. Runs
`bd init`, maps the codebase in one pass, seeds the map as `bd remember` memories (which
the beads `bd prime` hook surfaces every future session), then hands off to `feature` or
`new-project`. It does no planning of its own — it creates no epic or picks.

Use `feature` for incremental work on an already-managed repo; use `new-project` for a
brand-new project.

---

## Phase 0 — Precondition (idempotency)

Check whether this repo already has a **local** beads database:

```
test -d .beads && echo managed || echo unmanaged
```

Gate on the `.beads/` directory's presence — **not** `bd list`, which silently falls back
to the global shared beads DB when no local `.beads/` exists and would make an unmanaged
repo look "managed." If `.beads/` is present, the repo is already weft-managed — do not
re-onboard. Tell the user and point them onward: `feature` (incremental work on existing
code) or `new-project` (greenfield build). Exit.

Only proceed when `.beads/` is absent (the repo has never run weft).

---

## Phase 1 — bd init

Initialise beads for this repo:

```
bd init --non-interactive -p <prefix>
```

Choose `<prefix>` from the repository directory name (a short, lowercase slug); if the
directory name is ambiguous or unsuitable, ask the user for a prefix. `bd init` creates a
local-only beads DB under `.beads/` (no Dolt remote — the user wires sync later). Confirm
`.beads/` now exists before proceeding.

---

## Phase 2 — Codebase map (one Explore pass)

Dispatch **one** `Explore` subagent to map the existing code in a single pass. Instruct it
to investigate four axes and return a digest with one short section per axis (a few bullet
points each — not exhaustive prose):

- **Stack & integrations** — languages, runtimes, frameworks, key dependencies, external
  services/APIs, build/run/test commands.
- **Architecture & structure** — top-level layout, layers/boundaries, entry points, where
  the important code lives.
- **Conventions & testing** — code style, naming, error-handling patterns, the test
  framework and how tests are organised/run.
- **Concerns** — notable tech debt, known fragile areas, or hazards a contributor should
  know.

Hold the returned digest in context for Phase 3. Never dispatch more than one recon pass.

---

## Phase 3 — Seed bead-backed memory

Persist the digest as durable `bd remember` memories — one per axis, with stable keys — so
the beads `bd prime` hook surfaces them every future session (beads is the brain; no
CONTEXT.md, no `.planning/` files):

```
bd remember "<stack & integrations digest>"     --key weft-map-stack
bd remember "<architecture & structure digest>"  --key weft-map-arch
bd remember "<conventions & testing digest>"      --key weft-map-conventions
bd remember "<concerns digest>"                    --key weft-map-concerns
```

Then seed one orientation memory so future sessions know how to drive weft:

```
bd remember "weft repo: incremental work on existing code -> /weft-feature; greenfield build -> /weft-new-project; vocab — warp (the bead dependency graph/plan), weft (the woven work), pick (one bead -> one jj change), shed (one parallel wave)." --key weft-orientation
```

Keep each memory concise (a few lines); they are orientation, not full documentation.

---

## Phase 4 — Hand off

The repo is now weft-ready: `.beads/` is initialised and the codebase map + orientation are
seeded as memories. onboard plans nothing itself. Present both exits and let the user pick:

- **`feature`** — incremental work on the existing code (mints one epic + picks; minutes).
- **`new-project`** — a greenfield/first build planned from scratch.

Closing note: weft's VCS verbs (`execute`/`shed`/`pick`) require a colocated jj repo. If
this repo is not yet jj-colocated, set that up via the `jj-init` skill before running them.
(VCS setup is out of scope for onboard.)

---

## What this workflow does NOT do

- It does not plan — no epic, no picks. That is `feature` / `new-project`.
- It does not run the four-axis map as four separate agents (GSD's model) — one Explore
  pass, deliberately compressed.
- It does not write `.planning/` files or CLAUDE.md prose — the map lives in `bd remember`
  memories, surfaced by `bd prime`.
- It does not set up jj colocation or a Dolt remote — `jj-init` and later sync wiring own
  those.
````

- [ ] **Step 2: Validate the plugin tree (registers the new skill)**

Run:

```
claude plugin validate ./plugin --strict
claude plugin validate . --strict
grep -RnE 'weft/(agents|references|workflows)/' plugin/ ; echo "grep-exit=$?"
```

Expected: both `validate` runs exit 0 and accept the new `onboard` skill; grep prints no lines and `grep-exit=1`.

- [ ] **Step 3: Commit**

Commit per `references/vcs-preamble.md`. Message: `feat(weft-ccy.7): onboard skill — make an existing repo weft-ready`.

---

## Task 2: feature precondition fix (close the routing triad)

**Files:**

- Modify: `plugin/skills/feature/SKILL.md` (Phase 0 weft-managed paragraph, line ~34)

**Depends on:** Task 1 (the routing triad is only coherent once `onboard` exists).

The shipped Phase 0 has two exit branches: `.beads/` absent → `onboard`; `.beads/`-present-but-empty-warp → `new-project`. Delete the second branch so weft-managed = `.beads/`-present (a freshly-onboarded repo has existing *code* — `feature` is right for it even with an empty warp; `feature` mints its own epic).

- [ ] **Step 1: Replace the Phase 0 weft-managed paragraph**

In `plugin/skills/feature/SKILL.md`, replace this paragraph:

```markdown
The repo is weft-managed when `.beads/` is present **and** `bd list --json` returns a
non-empty warp (at least one existing epic/issue). If `.beads/` is absent, the repo is
not weft-managed yet — point the user to `onboard` (to make it weft-ready) and exit. If
`.beads/` is present but the warp is empty, the repo is weft-ready but has nothing to
build incremental work on — point the user to `new-project` (for a greenfield build) and
exit.
```

with:

```markdown
The repo is weft-managed when `.beads/` is present. If `.beads/` is absent, the repo is
not weft-managed yet — point the user to `onboard` (to make it weft-ready) and exit.
Otherwise proceed: `feature` does incremental work on the existing **code**, so an
initialised repo qualifies even when its warp is empty (a freshly-onboarded repo) —
`feature` mints its own epic below. (For a greenfield build with no existing code to
extend, the user can choose `new-project` directly.)
```

- [ ] **Step 2: Verify the `bd list --json` precondition command still reads naturally**

The Phase 0 command block (`bd list --json`) above the paragraph is unchanged — it is now used only to detect `.beads/` presence / readability, not warp emptiness. Confirm by reading the surrounding lines that no other Phase 0 sentence still references "non-empty warp" or routes an empty warp to `new-project`:

```
rg -n "non-empty warp|warp is empty|nothing to build" plugin/skills/feature/SKILL.md ; echo "exit=$?"
```

Expected: no matches (`exit=1`). If any remain, remove that stale routing sentence.

- [ ] **Step 3: Validate the plugin tree**

Run:

```
claude plugin validate ./plugin --strict
claude plugin validate . --strict
grep -RnE 'weft/(agents|references|workflows)/' plugin/ ; echo "grep-exit=$?"
```

Expected: both `validate` runs exit 0; grep prints no lines, `grep-exit=1`.

- [ ] **Step 4: Commit**

Commit per `references/vcs-preamble.md`. Message: `fix(weft-ccy.7): feature accepts any .beads-present repo (close routing triad)`.

---

## Task 3: End-to-end validation (manual dogfood gate)

**Files:** none modified — this is the acceptance gate.

**Depends on:** Tasks 1–2.

Drive interactively in a throwaway non-weft scratch dir so onboarding does not touch the real warp.

- [ ] **Step 1: onboard a fresh non-weft repo**

Create a scratch dir with some real-ish code (e.g. `mkdir /tmp/onboard-dogfood && cd /tmp/onboard-dogfood && git init && printf 'package main\nfunc main(){}\n' > main.go`). Run `onboard`.

Expected: Phase 0 sees no `.beads/` and proceeds; `bd init --non-interactive -p onboard-dogfood` creates `.beads/`; one Explore pass runs and returns a four-axis digest; Phase 3 seeds the five `bd remember` entries. Verify:

```
cd /tmp/onboard-dogfood
ls .beads
bd memories 2>&1 | rg -i "weft-map-stack|weft-map-arch|weft-map-conventions|weft-map-concerns|weft-orientation"
```

Expected: `.beads/` exists; all five memory keys present.

- [ ] **Step 2: confirm bd prime surfaces the seeded memories**

```
cd /tmp/onboard-dogfood
bd prime 2>&1 | rg -i "weft-orientation|weft-map"
```

Expected: the seeded memories appear in `bd prime` output (proving future sessions are oriented).

- [ ] **Step 3: confirm feature now accepts the onboarded repo**

Run `feature` against `/tmp/onboard-dogfood` (which has `.beads/` + an empty warp).

Expected: with the Task-2 fix, `feature` Phase 0 proceeds (does NOT bounce to `new-project`) and goes on to mint a feature epic — proving the routing triad is coherent.

- [ ] **Step 4: confirm idempotency**

Run `onboard` again against `/tmp/onboard-dogfood` (now weft-managed).

Expected: Phase 0 detects `.beads/` present and exits with the "already weft-managed → feature/new-project" hint; it does not re-init or re-map.

- [ ] **Step 5: Record the dogfood result**

```
bd note weft-ccy.7 "dogfood: onboard bd-init + 1 Explore map + 5 bd remember seeds surfaced by bd prime; feature accepts onboarded empty-warp repo; onboard idempotent — all verified — <date>"
```
<!-- adr-capture: sha256=1e4081eb28afdb39; session=cli; ts=2026-06-11T23:47:09Z; adrs=weft-7nr -->
