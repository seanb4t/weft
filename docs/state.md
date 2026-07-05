<!--
  ~ SPDX-License-Identifier: Apache-2.0
  ~ Copyright 2026 Weft Contributors
-->

# Weft — STATE (live position)

> Status: **living** — rewritten as work moves; the synthesis layer, NOT a source of truth. ·
> Updated: 2026-07-05
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

**Milestone:** unattended-trust hardening (roadmap §3 / §7.4) — **LANDED 2026-07-05** (epic
`weft-x38`, PR #103 merged). Executed autonomously via `/drain`: nine pick-iterations, one
bead per turn (the seven materialized picks plus two review-filed follow-ups — `.8`
doctor-never-mutates I1 coverage, `.6.1` ADR/plan reconciliation), each an implementer subagent
gated by two-stage spec + code/test review. Shipped: `internal/liveness` (inferred executor
recency), `weft doctor` (whole-warp health join — report + propose, never mutates), `reap`
executor_live wiring + foreign-workspace guard + `--dry-run`, seam-11 `finish reconcile`
re-verification, resolver oscillation cap, replan removed-pick I2 guard, and the §7.4 doctor+reap
exit-test E2E. Invariants I1–I4 all hold by test; `/review-pr` gate PASS (turn 1 caught two real
invariant inversions — I3 non-positive threshold, I4 finalize gate — fixed + re-verified). Design
trail: spec `docs/superpowers/specs/2026-07-04-unattended-trust-design.md`, plan
`docs/superpowers/plans/2026-07-04-unattended-trust.md`, ADRs `weft-jcg`/`weft-qc0`/`weft-0pq`.

**Active implementation:** none — the epic is closed, its picks and the `/drain` audit bead
(`weft-t35`) closed, warp synced. Roadmap §7 steps 1–4 are complete.

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

- **New this cycle:** the `/drain` run plus this session's review fan-outs left a pile of stale jj
  workspaces — `worktree-drain-epic-weft-x38` and ~40 `worktree-agent-*` isolation worktrees
  (Claude Code subagent leftovers). These are precisely what the freshly-shipped `weft doctor` /
  `weft reap` foreign-workspace guard exists to flag — a good first live target for the tooling.
- **Carried:** the weft-9i3 Node/happy-dom XSS test (`helper.test.mjs`) is still unrun (needs a
  one-time `npm install`); the `helper.js` fix itself shipped in 0.2.2.

## Next concrete step

**Roadmap §7 step 5 — weft weaves weft** (v1.0 exit criterion 1): run a real weft feature
end-to-end through weft's *own* loop (feature/new-project skill → `plan emit` → shed waves →
picks → `finish`), not the dev-flow meta-tooling that built v0.x. With §3 unattended-trust
landed, this is the next milestone — and the first one weft can plausibly run on itself.

**Carried (independent):** the legacy `weft/` prompt tree is **gone** — `weft-9q5` landed via
PR #107 (15 files removed, `NOTICE` repointed `weft/`→`plugin/`, seam-10 de-linked, review gate
PASS). Remaining carry: sweep the ~40 stale `worktree-*` jj workspaces this session left (see
Strays) — now a `weft doctor`/`reap` job, no longer by-hand.

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

- **Landed this cycle:** the **unattended-trust milestone** — epic `weft-x38` (§7.4) executed
  end-to-end via `/drain` (9 picks, invariants I1–I4 by test, `/review-pr` PASS) and merged as
  PR #103; §7.3 materialization + 3 ADRs (`weft-jcg`/`weft-qc0`/`weft-0pq`) preceded it. Steering
  groomed in #106; the **legacy `weft/` tree was deleted** (`weft-9q5`, PR #107). Epic + drain
  beads closed, warp synced.
- **Decisions settled:** roadmap §9 (1–5) confirmed 2026-07-04.
- **Next thread:** roadmap §7 step 5 — **weft weaves weft** (exit criterion 1). Carried: sweep the
  stale `worktree-*` jj workspaces this session left, via the newly-shipped `doctor`/`reap`. All
  fresh work off clean `main` (#107).
