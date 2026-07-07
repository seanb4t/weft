<!--
  ~ SPDX-License-Identifier: Apache-2.0
  ~ Copyright 2026 Weft Contributors
-->

# Weft — STATE (live position)

> Status: **living** — rewritten as work moves; the synthesis layer, NOT a source of truth. ·
> Updated: 2026-07-07
>
> **What this is:** the live "where are we, what's next, why". Read
> [`roadmap.md`](./roadmap.md) first for intent, then this file for position, then **beads** for
> the actual work graph.
>
> **The one rule (anti-staleness):** this file holds ONLY what `bd` cannot compute. It references
> beads by ID; it **never** restates issue status, counts, dependencies, or blockers — those are
> live in beads (`bd ready`, `bd blocked`, `bd stats`, `bd show <id>`). **If a `bd` query can
> produce it, it does not belong here.**
>
> **Bootstrap exception:** see the roadmap header — this cadence retires when weft plans weft
> (roadmap §2, exit criterion 1).

---

## Current focus

**Milestone:** **weft weaves weft** — first self-weave (roadmap §7 step 5 / v1.0 exit criterion 1)
— **first run LANDED 2026-07-07** (epic `weft-j4c`, PR #115 merged). weft planned and wove a real
feature *through its own loop*, not the dev-flow meta-tooling that built v0.x: `feature` skill
(rough seed → one intake round → `weft-planner` → `plan emit`) → `shed form`/`isolate` → per-pick
`weft-executor` (TDD) → `pick verify`/`seal` → `shed integrate` → `pick land` → `finish open` →
post-merge reconcile. Shipped feature: the **`weft status`** warp-progress readout — a Go
subcommand (`internal/cli/status.go`; whole-warp overview + per-epic drill; text + `--json`;
**beads-only**, no steering-doc coupling) plus the `/weft:status` plugin skill that narrates it.
Two waves, zero conflicts, both verify gates green.

**The harvest (the point of dogfooding):** the planning/weaving logic came through clean on the
first pass; **three defects surfaced at substrate *seams***, filed as `weft-uwq` (stale dist
binary on PATH → `plan emit` silently under-wired parent/blocks edges — an exit-0 warp
corruption), `weft-fxj` (`finish open`'s `gh pr create` omits `--head` → fails on jj's detached
HEAD), and `weft-ojw` (`finish reconcile` errors when the merged head branch is deleted/pruned).
All three cluster at tooling boundaries — artifact-currency, jj↔gh, post-merge — that unit tests
don't exercise; the feature code itself carried none. `weft-uwq` remediation already in flight
(`task install` jj-safe version stamp, PR #114 landed).

**Active implementation:** none — epic `weft-j4c` closed, both picks landed on `main` (112f2488),
warp synced. The three seam findings are open.

**Prior milestone:** unattended-trust hardening (§3 / §7.4) landed 2026-07-05 (epic `weft-x38`,
PR #103; steering groomed in #106) — `weft doctor`/`reap`/`internal/liveness`, invariants I1–I4 by
test. Roadmap §7 steps 1–4 complete; git history + #103 carry the detail.

**Recently established (2026-07-03):**

- Deep review of design / warp / code / VCS: design→code coherence confirmed excellent; the one
  structural gap is whole-warp health — per-epic `resume` exists, whole-warp `doctor` does not
  (roadmap §3).
- This roadmap+state cadence adopted, mirroring fovea's, as the bootstrap exception to the
  no-ROADMAP hard rule.
- README staleness fixed (#88, merged); CLAUDE.md carve-out landed (bootstrap-exception +
  "Steering docs" section now in CLAUDE.md); `design.md` + all 11 seam headers refreshed to
  shipped status via `weft-a01` / PR #98 (roadmap §6 "Fix" — the honest-docs pass swept all
  eleven seams, not just 01–07, for corpus coherence).
- Grooming infrastructure added (bead weft-k2g): CLAUDE.md gained the hard-rule carve-out
  plus a "Steering docs" section with the agent-owned grooming protocol, and a repo-local
  SessionStart hook (`.claude/hooks/session-start-steering` + `.claude/settings.json`) now
  surfaces this pair each session and warns when `state.md` is >14 days stale.

## Strays — the by-hand doctor gap is now shipped (2026-07-05)

The founding audit's manual bead×workspace×change join is now the shipped `weft doctor`
(whole-warp health, report + propose) plus `reap`'s foreign-workspace guard — exactly the §3
capability. The old stray-pick roster (weft-8r4 #80, weft-ecj #89, weft-aff #93, weft-9i3 #92;
v0.2.1 #74, v0.2.2 #95) is retired from this doc; git history holds it.

- **jj hygiene swept clean (2026-07-06):** cleared the accumulated review/drain subagent-isolation
  residue — 67 orphan `worktree-agent-*` bookmarks + their empty changes, the stale
  `worktree-drain-epic-weft-x38` liveness change (a superseded pre-squash-merge draft of #103), and
  assorted empty scratch WCs. Live workspaces are now `default` + `claude-status` only; `jj log`
  heads are just `main` + the two workspace `@`s. The by-hand sweep (empty+undescribed safety
  filter → `jj bookmark delete` + `jj abandon`, non-empty/described changes verified redundant
  before abandon) is precisely what `weft doctor` should *flag* and a future `doctor`-fed cleanup
  verb should *automate* — `reap`'s foreign-workspace guard deliberately won't touch these.
- **Carried:** the weft-9i3 Node/happy-dom XSS test (`helper.test.mjs`) is still unrun (needs a
  one-time `npm install`); the `helper.js` fix itself shipped in 0.2.2.

## Next concrete step

**Harden the self-weave loop** — §7 step 5 is now *demonstrated* (first run landed), not *done*.
The next thread is to fix the three seam findings the run exposed — `weft-uwq` (binary-currency
guard / `weft doctor` self-staleness check), `weft-fxj` (`finish open` → pass `--head`/`--base` to
`gh`), `weft-ojw` (`finish reconcile` → tolerate a deleted/pruned merged branch) — so the next
self-weave runs end-to-end without hand-holding. Then continue dogfooding toward the remaining
v1.0 exits (fovea onboard; self-host that retires this steering pair). `bd ready` for the queue.

**Carried (independent):** the legacy `weft/` prompt tree is **gone** (`weft-9q5`, PR #107). jj
workspace hygiene: `weft reap` now actively collects stale empty workspaces (it reaped the
`claude-status` WC this session under an unscoped invocation — recreate if a statusline needs it);
prefer `weft reap --epic <id>` to stay scoped.

## Open questions / decisions in flight

- **Roadmap §9 decisions (1–5)** — **confirmed by Sean 2026-07-04** (all five as-proposed):
  milestone = unattended-trust hardening; delete legacy `weft/` tree; fovea-onboard = v1.0 exit;
  materialize deferred scope now; release cadence = milestone-or-2-weeks. No longer in flight.
- **weft-9i3 test approach** — resolved: kept the stdlib-`unittest` PID test (GREEN); the vendored
  script stays unhardened-for-zombies (production has none). The Node/happy-dom XSS test still
  needs a one-time `npm install` to run — outstanding follow-up.
- **Workspace hygiene** — **shipped:** `doctor` now *flags* foreign workspaces
  (`worktree-agent-*` / `drain-epic-*` leftovers) as a `foreign` finding, and `reap` has the guard
  so it never forgets them (the design round found today's reap would — a live-session hazard).
  Both landed in `weft-x38` (#103); the remaining leftovers are a run-the-tool sweep, not an open
  question.

## Session handoff

A fresh session should read, in order: `roadmap.md` (intent) → `design.md` + `seams/` (what's
built) → this file (where we are). Then `bd ready` for the actual work queue.

- **Landed this cycle:** the **first weft-weaves-weft self-weave** — epic `weft-j4c` planned and
  woven through weft's own loop and merged as PR #115 (`weft status` subcommand + `/weft:status`
  skill). The run harvested three seam findings (`weft-uwq`, `weft-fxj`, `weft-ojw`). Epic + both
  picks closed, warp synced.
- **Prior cycle:** unattended-trust milestone (`weft-x38`, PR #103; steering #106) and legacy
  `weft/` tree deletion (`weft-9q5`, PR #107).
- **Decisions settled:** roadmap §9 (1–5) confirmed 2026-07-04.
- **Next thread:** harden the self-weave loop — fix `weft-uwq`/`weft-fxj`/`weft-ojw`, then continue
  dogfooding toward the remaining v1.0 exits (fovea onboard; self-host). Fresh work off clean
  `main` (112f2488, #115).
