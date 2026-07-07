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

**The harvest (the point of dogfooding):** the post-mortem
(`docs/postmortems/2026-07-07-weft-weaves-weft.md`, PR #117) is blunt — weft *wove* the code but did
**not** ship a working feature unaided. The authoring core came through clean, but defects surfaced
at every substrate seam **and in the feature itself**: `weft-uwq` (stale dist binary → `plan emit`
silently under-wired parent/blocks edges, exit-0 warp corruption; remediated by the `task install`
jj-safe stamp, PR #114), `weft-fxj` (`finish open`'s `gh pr create` omits `--head` → fails on jj's
detached HEAD), `weft-ojw` (`finish reconcile` errors on a deleted/pruned merged branch), and — the
sharpest finding — **`weft-1ve` (P1): the delivered `weft status` shipped broken.** It calls
`bd list` without `--all`, so it can never show closed/*done* work and reports a full warp as
*empty*. It passed every gate because the tests used fake `bd` fixtures and the built command was
**never run** against the real warp — so `weft-4e8` (P2) adds a real-execution smoke gate, without
which the loop certifies compilation, not correctness. `weft-p8t` (P2) adds E2E coverage for the
whole `finish` family (covers `weft-fxj`/`weft-ojw`).

**Active implementation:** none — epic `weft-j4c` closed, both picks landed on `main`, warp synced.
`weft-1ve` **fixed** (PR #118 merged); the remaining findings are open (`weft-4e8`, `weft-fxj`,
`weft-ojw`, `weft-p8t`); `weft-uwq` closed via #114.

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

**Harden the self-weave loop** — §7 step 5 is *demonstrated* (first run landed), not *done*; the
post-mortem (#117) shows weft can weave code but not yet ship a working feature unaided. Two threads,
in priority: **(a) the loop's blind spot** — `weft-1ve` (the broken `weft status`) is **fixed** (PR
#118: `--all` + a *contract-faithful* regression fake, verified against the real warp). Its root
cause generalized: the verify gap is **circular self-certification** — an executor authors the test,
the fake it drives, and the code, and the gate checks only that they agree — not a missing flag. So
`weft-4e8` (real-execution smoke gate) is now a *subset* of the deeper **`weft-y85`**: *generalize
planner + verification* so self-certifying, fixture-only validation is structurally impossible for
substrate-touching work — a discovered verification profile + per-pick validation surface + verify's
three questions, **language/repo-agnostic**, held for a **dedicated design session** (brainstorm →
spec → warp). **(b) close the loop's own ends** — `weft-fxj` (`finish open` → pass `--head`/`--base`),
`weft-ojw` (`finish reconcile` → tolerate a deleted/pruned merged branch), with `weft-p8t` E2E
coverage. (`weft-uwq` already remediated, PR #114.) Then continue dogfooding toward the remaining
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

- **Landed this cycle:** the **first weft-weaves-weft self-weave** — epic `weft-j4c` woven through
  weft's own loop and merged as PR #115 (`weft status` + `/weft:status`), plus its **post-mortem**
  (PR #117). Honest verdict: the authoring core self-hosted, but `finish` was hand-held and the
  deliverable **shipped broken** — findings `weft-uwq` (closed), `weft-fxj`, `weft-ojw`, and the new
  `weft-1ve` (status broken), `weft-4e8` (real-run verification gap), `weft-p8t` (finish E2E). Epic +
  both picks closed, warp synced.
- **Prior cycle:** unattended-trust milestone (`weft-x38`, PR #103; steering #106) and legacy
  `weft/` tree deletion (`weft-9q5`, PR #107).
- **Decisions settled:** roadmap §9 (1–5) confirmed 2026-07-04.
- **Since the post-mortem:** `weft-1ve` **fixed** (PR #118 merged) and its root cause generalized
  into design bead **`weft-y85`** (*generalize planner + verification* against circular
  self-certification; `weft-4e8` is a subset) — held for a **dedicated design session**. This
  steering refresh is PR #119.
- **Next thread:** run the `weft-y85` design session (the loop's real blind spot), and close the
  loop's own ends (`weft-fxj`/`weft-ojw`, E2E `weft-p8t`); then continue dogfooding toward the
  remaining v1.0 exits (fovea onboard; self-host). Fresh work off clean `main`.
