<!--
  ~ SPDX-License-Identifier: Apache-2.0
  ~ Copyright 2026 Weft Contributors
-->

# `sketch` + `ui-phase` skills — visual direction + UI contract (ccy.10) — design

**Date:** 2026-06-20
**Bead:** weft-ccy.10 (parent epic: weft-ccy — Restore GSD Layer-A interactive/phased loop)
**Status:** design
**Port source:** `/gsd-sketch`, `/gsd-ui-phase` (GSD Core, MIT), rewritten to bead-backed state; visual companion vendored from `obra/superpowers` (MIT)

## Context

`weft-ccy` restores GSD's pre-planning + interactive surface over weft's stable
verb surface + beads. `weft-ccy.10` is the **last open child** of the epic (9/10
closed). It was split from `weft-ccy.4` (explore + spike, shipped via PR #63),
which deliberately scoped *only* the two skills whose outputs are naturally
bead-shaped. `sketch` was deferred because (a) its native output is
browser-rendered HTML mockups, which fight weft's "no `.planning/` files,
beads-is-the-brain" contract (ADR `weft-fwn`), and (b) its GSD downstream
consumer `/gsd-ui-phase` was "deliberately not ported in this round" (06-09 epic
spec §"Out of scope"). The split bundled the **ui-phase consumer question** into
this bead so the two are designed together.

**Decision (this bead):** port **both** `sketch` and `ui-phase` at **full GSD
fidelity**, completing the explore/spike/sketch pre-planning trio and giving
sketch a native consumer. This reverses the 06-09 "ui-phase not ported"
deferral; that deferral was explicitly "in this round," and that round is now
complete. The substrate-fit and consumer questions both resolve cleanly against
existing weft seams (throwaway exploration → ephemeral scratch; durable design
decisions → `bd remember` + decision beads + epic `design` field; consumer →
the already-shipped `plan-phase`).

A late refinement during this session replaced the original static-HTML sketch
mechanic with the **visual companion** from `obra/superpowers'` brainstorming
skill — a zero-dependency Node.js browser server with live click-to-select
interaction. It is **vendored into weft** (weft does not assume `dev-flow` is
installed alongside it) and **rebranded to weft** (scratch dir `.weft/`,
weft-branded UI chrome), with MIT attribution recorded in a NOTICE co-located in
the vendored directory (the files carry no per-file license headers upstream).

## Decisions made in this session

| Decision | Choice |
|---|---|
| Scope | Port **both** `sketch` and `ui-phase` (full GSD fidelity), reversing the 06-09 ui-phase deferral. |
| sketch mechanic | **Vendored visual companion** (live browser, side-by-side, click-to-select), **not** static HTML and **not** throwaway-jj. |
| sketch artifact lifecycle | Mockups are ephemeral, written to gitignored `.weft/` (or `/tmp` fallback); jj never snapshots gitignored paths in a colocated repo, so the working copy stays clean with no jj choreography. |
| sketch durable output | Chosen visual direction → `sketch`-labelled **`bd remember`** + epic `design` field (same-session availability per ADR `weft-b19`). weft's analog of GSD's `sketch-findings-*/SKILL.md`. |
| companion source | **Vendor** the zero-dep MIT server into weft's plugin tree (self-contained). Not a `dev-flow` runtime dependency. |
| companion branding | Rebrand to weft: scratch dir `.weft/`, weft-branded `frame-template.html` chrome. MIT attribution recorded in a co-located NOTICE + an UPSTREAM pin of the obra source — the vendored files carry no per-file license headers upstream. |
| ui-phase structure | **Two agents** — `weft-ui-researcher` + `weft-ui-checker` — with a **≤2-iteration** revision loop (mirrors GSD). |
| ui-phase durable output | UI contract → a **`decision` bead** (`ui-spec` label, `related` to the phase epic) **+** the phase epic `design` field. Not a `UI-SPEC.md` file. |
| consumer | The already-shipped **`plan-phase`** (ccy.3). Modified to add a UI safety gate + to pass the UI contract to the planner. |
| safety gate | **Config-less, non-blocking, skippable** nudge (weft has no `workflow.*` config system; GSD's `ui_phase`/`ui_safety_gate` toggles are dropped). |
| engine | **No Go/engine changes.** Entirely prompt-layer (skills + agents + ADR) plus vendored scripts. |

## Positioning in the per-phase rhythm

Today's per-phase rhythm:

```
discuss → plan-phase → execute --epic <phase> → verify-work → finish
```

With this work, frontend phases gain two optional upstream doors, in GSD's
order — `sketch` explores visual direction, `ui-phase` locks the UI contract,
both before planning:

```
discuss → [sketch] → ui-phase → plan-phase → execute --epic <phase> → verify-work → finish
```

`ui-phase` reads the discuss decisions from the phase epic's `design` field
exactly as `plan-phase` already does (Phase 1 of `plan-phase`). Both new doors
are **optional and standalone** — invoked explicitly, never auto-chained — with
soft handoffs (sketch's `bd remember` is read by ui-phase; ui-phase's decision
bead + `design` field is read by plan-phase).

## 1. `sketch` skill

**File:** `plugin/skills/sketch/SKILL.md` (skill-only, plugin tree; no `weft/`
host-agnostic pair, consistent with explore/spike per ADR `weft-88z`).
**Argument hint:** `[topic] [epic-id]` (loose pre-project, or epic-scoped for a
phase, mirroring `spike`).

GSD's `/gsd-sketch` produces 2–3 throwaway HTML mockup variants (CSS themes)
under `.planning/sketches/NNN/` for **visual/UI direction**, and `--wrap-up`
packages the validated decisions into a project-local skill. weft keeps the
"throwaway exploration medium, durable decisions survive" shape (the spike
pattern) but upgrades the medium to a live interactive browser companion.

**Phases:**

1. **Intake.** Frame the UI surface and how many variants to show (2–4 — the
   companion's "2–4 options max"). One light round of framing questions if the
   surface is unclear.
2. **Launch companion.** Start the vendored server
   (`scripts/start-server.sh --project-dir <repo-root>` → scratch under
   gitignored `.weft/sketch/<session>/`; background per platform). Capture the
   URL + `screen_dir` + `state_dir` from the startup JSON / `$STATE_DIR/server-info`.
   Tell the user to open the URL.
3. **Push variants + iterate.** Write each mockup variant as a **content
   fragment** (the server wraps it in the weft frame template) to a fresh,
   semantically-named file in `screen_dir`, using the companion's layout classes
   (`.split` side-by-side, `.cards`, `.mockup`, `.mock-nav`/`.mock-sidebar`/
   `.mock-content`/etc.). On each subsequent turn, read `state_dir/events`
   (JSONL of clicks/selections) plus the user's terminal text. Iterate
   (`layout-v2.html`, …) until a direction is chosen.
4. **Capture the chosen direction.** From the selection events + terminal text,
   record the chosen visual direction (layout, palette, typography, spacing) as
   a `sketch`-labelled **`bd remember`** entry (`--key sketch-<slug>`) and, when
   epic-scoped, fold it into the phase epic `design` field (same-session
   availability for `ui-phase`, per ADR `weft-b19`).
5. **Stop + hand off.** `scripts/stop-server.sh $SESSION_DIR`. Mockups in
   `.weft/` are transient (gitignored; `/tmp` sessions auto-deleted). Point
   onward: → `ui-phase` to lock the full UI contract, or → `plan-phase`.

**Degraded mode.** If `node` is not on `PATH`, fall back to writing
self-contained HTML files to `/tmp` and handing the user `file://` paths (the
original static approach), noting the degradation. The durable output
(`bd remember` + `design` field) is identical either way.

**Does NOT:** keep or commit mockups; write durable `.planning/`/sidecar state;
run the companion's persistence into a tracked path.

## 2. Vendored visual companion

GSD's sketch and ui-phase both benefit from a browser surface; rather than
reimplement one, weft vendors `obra/superpowers'` brainstorming visual
companion — a **zero-dependency Node.js server** (Node built-ins only: HTTP,
WebSocket, filesystem watch). It watches a directory for HTML files and serves
the newest to the browser, wraps content fragments in a frame template, and
records click selections to `state_dir/events` for the agent to read next turn.

**Vendored files** (≈26 KB): `server.cjs` (single-file CommonJS server),
`start-server.sh`, `stop-server.sh`, `frame-template.html`, `helper.js`.

**Vendoring decisions:**

- **Self-contained.** weft ships as its own plugin and does not assume
  `dev-flow` is installed; depending on `dev-flow`'s installed script path
  (which carries a per-release version-hash segment) is rejected as fragile.
- **Shared location** under `plugin/` so both `sketch` and `ui-phase` reference
  one copy. Exact path resolved at plan time per weft's `${CLAUDE_PLUGIN_ROOT}`
  reference conventions and the plugin-validate path rules (candidate:
  `plugin/skills/sketch/scripts/visual-companion/`, referenced by `ui-phase`
  via `${CLAUDE_PLUGIN_ROOT}/skills/sketch/scripts/visual-companion/`; or a
  top-level `plugin/scripts/visual-companion/`).
- **weft rebrand** (modifications to the MIT source, which MIT permits):
  - scratch dir constant `.superpowers/` → `.weft/` in `start-server.sh` /
    `server.cjs`;
  - user-facing chrome in `frame-template.html` (header/title) reads **weft**,
    not Superpowers/brainstorm.
- **Licensing.** The vendored files **carry no per-file license headers
  upstream** — the MIT License applies to them via the `obra/superpowers`
  repository LICENSE file. They are NOT relicensed to Apache-2.0 and do NOT
  receive weft's Apache SPDX header. A **NOTICE** co-located in the vendored
  directory records the MIT attribution; an **UPSTREAM** note pins the
  `obra/superpowers` source commit and enumerates the weft modifications
  (`.weft/` rename, chrome rebrand). weft's SPDX-header convention (Apache-2.0
  on all weft-authored source) is a contributor practice, **not** a CI-enforced
  gate — there is no SPDX/header check in `.github/workflows/`. So no exemption
  needs wiring: the rule is simply *do not add weft's Apache header to the
  vendored MIT files*.
- **`.gitignore`.** Add `.weft/`. Leave the existing `.superpowers/` entry
  intact (it covers `dev-flow`'s own brainstorming when run inside this repo).

**Runtime prerequisite.** `node` on `PATH` at sketch/ui-phase time (only when
the companion is actually used). weft CI already installs Node (for
`claude plugin validate`); the server is zero-dependency, so no `npm install`.
Degraded fallback as in §1.

## 3. `ui-phase` skill

**File:** `plugin/skills/ui-phase/SKILL.md` (skill-only, plugin tree).
**Argument hint:** `[phase-epic-id]`.

GSD's `/gsd-ui-phase` locks UI design decisions before planning, orchestrating a
`gsd-ui-researcher` and a `gsd-ui-checker` with a ≤2-iteration revision loop,
consuming sketch findings + phase context + detected design-system state, and
emitting a `{phase}-UI-SPEC.md` contract that the planner reads (guarded by a UI
safety gate in plan-phase). weft mirrors the structure with bead-native I/O.

**Phases:**

1. **Load context (from beads).** `bd show <phase-epic-id>` → mini-brief
   (`description`), `acceptance`, and `design` field (discuss decisions). Read
   any sketch finding (`bd remember` `sketch-*` / the epic `design` field). If a
   `ui-spec` decision bead already exists for this phase, offer **update / view
   / skip** (GSD's existing-UI-SPEC behaviour).
2. **Detect design-system state.** Scan the repo for `components.json`
   (shadcn), Tailwind config, existing design tokens, and the frontend
   framework (`package.json`) — informs which questions are already answered.
3. **Dispatch `weft-ui-researcher`.** Treats sketch-locked decisions as settled
   (does not re-ask them); asks only **unanswered** questions across the five
   areas — **spacing, color, typography, copywriting, registry-safety**; may
   reuse the visual companion for visual sub-questions (e.g. comparing spacing
   scales or palettes). Produces a **draft UI contract**.
4. **Dispatch `weft-ui-checker`.** Validates the draft against six dimensions —
   **Copywriting, Visuals, Color, Typography, Spacing, Registry-Safety**.
   Flagged issues re-run the researcher; **≤2 iterations**.
5. **Approve + persist.** Present the locked contract; on human approval,
   persist bead-native: a **`decision` bead** (`bd create --type=decision
   --labels ui-spec …`, then a `related` edge to the phase epic) holding the
   locked contract
   (spacing tokens, color variables, typography, copywriting, registry-safety
   notes), **and** fold the contract into the phase epic `design` field for
   same-session `plan-phase` consumption. Hand off: → `plan-phase`.

**Does NOT:** write `UI-SPEC.md`/`.planning/` files; introduce a `workflow.*`
config toggle (it is opt-in by invocation; the gate is the plan-phase nudge).

## 4. The two agents

**Files:** `plugin/agents/ui-researcher.md`, `plugin/agents/ui-checker.md`
(referenced as `weft-ui-researcher` / `weft-ui-checker`; adapted-from-GSD header
+ SPDX, mirroring the existing four agents). **Proposed models:**
`weft-ui-researcher` = `opus` (interactive design reasoning, like
`weft-planner`), `weft-ui-checker` = `sonnet` (bounded validation pass) — final
choice confirmed at plan time.

- **`weft-ui-researcher`** — reads the loaded phase context + sketch finding +
  detected design-system state; asks only unanswered questions across the five
  areas; drafts the UI contract.
- **`weft-ui-checker`** — validates a draft contract across the six dimensions
  and returns a structured pass/issues verdict that drives the revision loop.

## 5. `plan-phase` UI safety gate (consumer modification)

Modify the shipped `plugin/skills/plan-phase/SKILL.md`:

- **Phase 1 (Load phase context):** add a **non-blocking UI safety gate**.
  When the phase looks like frontend work — a design system is present in the
  repo *or* the mini-brief / existing picks mention UI/frontend/component/page/
  screen — **and** no `ui-spec` decision bead and no UI contract in the `design`
  field exist for the phase epic, prompt:
  *"This phase looks like frontend work but has no locked UI contract. Run
  `ui-phase <phase-epic-id>` first? (recommended / skip)."* The user may skip;
  the gate never blocks. There is no config toggle (weft has no `workflow.*`
  system) — the gate is always-on but skippable.
- **Phase 2 (Dispatch `weft-planner`):** include the UI contract (from the
  `design` field / `ui-spec` bead, if present) in the planner's design context,
  so generated picks reference the locked spacing tokens, color variables, and
  copywriting decisions.

These are additive prompt-layer edits; `plan-phase`'s existing behaviour on a
non-frontend phase (or a frontend phase with a contract already present) is
unchanged.

## 6. Coupling and relationship to existing skills

**Standalone doors, soft handoffs**, consistent with explore/spike:

- `sketch` and `ui-phase` are invoked explicitly; neither hard-invokes the
  other or any downstream skill.
- `sketch` *suggests* `ui-phase` (or `plan-phase`) at hand-off; its
  `bd remember` finding is *automatically* read by `ui-phase` (and surfaced by
  `bd prime` in later sessions).
- `ui-phase`'s decision bead + `design` field is *automatically* read by
  `plan-phase`; the safety gate *suggests* `ui-phase` when a frontend phase
  lacks a contract.
- `discuss` remains the HOW-shaping door; `ui-phase` is the **UI-contract**
  door, narrower and frontend-specific, run after discuss and before
  plan-phase.

## 7. GSD contrast

| GSD | weft |
|---|---|
| sketch mockups under `.planning/sketches/NNN/` | ephemeral mockups under gitignored `.weft/` (or `/tmp`); never committed |
| sketch `--wrap-up` → project-local `sketch-findings-*/SKILL.md` | chosen direction → `bd remember` + epic `design` field |
| static HTML opened manually | live visual companion (side-by-side, click-to-select, event capture) |
| `{phase}-UI-SPEC.md` design contract file | `decision` bead (`ui-spec`) + epic `design` field |
| `gsd-ui-researcher` + `gsd-ui-checker` | `weft-ui-researcher` + `weft-ui-checker` (same five areas / six dimensions, ≤2 loop) |
| `workflow.ui_phase` / `workflow.ui_safety_gate` config toggles | no config system; opt-in by invocation + always-on skippable plan-phase nudge |
| companion / agents reference `.planning/` + `STATE.md` | beads is the brain; no sidecar state |

## 8. Out of scope

- Autonomous / non-interactive UI generation — `sketch` and `ui-phase` are
  interactive doors only.
- A weft `workflow.*` configuration system — the safety gate is config-less.
- Any Go/engine change — this work is prompt-layer + vendored scripts.
- `.planning/` files or kept mockup artifacts — durable state lives in beads.
- Porting GSD command surface beyond `sketch` + `ui-phase` (seam-5 discipline:
  port additively, per need).
- Tracking upstream `obra/superpowers` companion changes automatically — the
  vendored copy is a pinned snapshot (UPSTREAM note records the commit); future
  syncs are manual.

## 9. Testing & validation

- **Plugin gates:** `claude plugin validate ./plugin --strict` and
  `. --strict` — confirm both accept the added vendored non-markdown files
  (`.cjs`/`.sh`/`.js`/`.html`); the intra-tree path-citation grep (no
  `weft/(agents|references|workflows)/` paths in `plugin/`) is unaffected (the
  vendored scripts cite no such intra-tree paths). The vendored files carry no
  per-file license headers; weft's Apache SPDX header is not added to them (no
  CI header gate exists — see §2).
- **Dogfood gate** (the ccy.4-style end-to-end run that caught real bugs):
  exercise `sketch → ui-phase → plan-phase` on a small frontend mock, verifying:
  the companion launches and serves variants (and the degraded `/tmp` fallback
  when `node` is absent); mockups land only in gitignored `.weft/` and the jj
  working copy stays clean; the chosen direction persists as `bd remember` +
  `design` field; `ui-phase` emits a `ui-spec` decision bead + `design` field
  and the researcher/checker ≤2 loop terminates; the plan-phase safety gate
  fires on a frontend phase and is skippable, and stays silent on a non-frontend
  phase; the planner receives the UI contract.
- **No engine tests** — there is no engine change. Confirmed.

## ADR impact

Expected ADR(s), captured via `/capture-adrs` after the plan phase:

- **ui-phase port reverses the deferral.** The 06-09 epic spec's
  "`/gsd-ui-phase` … deliberately not ported in this round" is now reversed;
  ui-phase ships at full fidelity. Relates to / addendums ADR `weft-cfp`
  (interaction model) and the 06-09 spec scoping.
- **Substrate fit for visual pre-planning.** Extends ADR `weft-fwn`
  (bead-native pre-planning outputs): sketch mockups are ephemeral
  (gitignored `.weft/` / `/tmp`), the chosen direction is `bd remember` +
  `design` field, and the UI contract is a `decision` bead + `design` field —
  completing the no-`.planning/` contract for the visual front of the funnel.
- **Vendored MIT companion + weft rebrand + licensing.** Records the vendoring
  decision (self-contained, not a `dev-flow` dependency), the `.weft/`/chrome
  rebrand as permitted MIT modifications, and the no-per-file-header /
  Apache-exempt handling (MIT via the upstream LICENSE) with NOTICE + UPSTREAM
  pin.
- **Config-less safety gate.** Records dropping GSD's `workflow.*` toggles in
  favour of an always-on skippable plan-phase nudge.

## Open items to ground at `writing-plans`

1. **Agent dual-tree — RESOLVED at plan time: `plugin/agents/` only.** The
   `weft/` tree is legacy and is neither shipped nor validated: the root
   `.claude-plugin/marketplace.json` declares a single plugin `source: ./plugin`
   (so `claude plugin validate . --strict` validates the *marketplace* → `./plugin`,
   **not** the `weft/` tree), the CI grep-discipline comment states "the weft/
   tree does not exist in the plugin cache," `weft/agents/weft-planner.md` is
   stale (last touched seam-9, 2026-06-07 — missing the ccy.3 phased-planner work
   that landed in `plugin/agents/planner.md` 2026-06-11), and no current skill or
   manifest references `weft/agents`. So new agents are created in
   `plugin/agents/` **only** — not dual-tree. (This corrects design-review
   round-1 finding #2, which assumed `. --strict` validates the `weft/` tree.)
2. **Final agent models** (`weft-ui-researcher`, `weft-ui-checker`).
3. **Exact vendored path** under `plugin/` per `${CLAUDE_PLUGIN_ROOT}`
   conventions + plugin-validate path rules (skill-owned `scripts/` vs
   top-level `plugin/scripts/`).
4. **Companion source commit** to pin in the UPSTREAM note, and the precise set
   of `frame-template.html` / `start-server.sh` / `server.cjs` lines touched by
   the `.weft/` + chrome rebrand.
<!-- adr-capture: sha256=58c2a45a178cf0f4; session=cli; ts=2026-06-20T15:15:47Z; adrs=weft-n8f,weft-ngf,weft-tub,weft-odp -->
