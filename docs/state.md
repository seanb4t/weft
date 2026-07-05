<!--
  ~ SPDX-License-Identifier: Apache-2.0
  ~ Copyright 2026 Weft Contributors
-->

# Weft — STATE (live position)

> Status: **living** — rewritten as work moves; the synthesis layer, NOT a source of truth. ·
> Updated: 2026-07-04
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

**Milestone:** unattended-trust hardening (roadmap §3 / §7.4). **§7.3 "encode the plan" is
done** — the milestone was designed and materialized into the warp 2026-07-04 (session
`38459612`): design bead `weft-x38` ran brainstorming → design-review (READY r2) →
writing-plans → plan-review (READY r2) → capture-adrs (3 ADRs: `weft-jcg` liveness-inferred,
`weft-qc0` doctor-never-mutates, `weft-0pq` replan-can't-drop-woven-work) → plan-to-beads,
which promoted `weft-x38` into the milestone epic with seven child picks. Spec:
`docs/superpowers/specs/2026-07-04-unattended-trust-design.md`; plan:
`docs/superpowers/plans/2026-07-04-unattended-trust.md`.

**Active implementation:** none yet — the epic's picks are fresh; `bd ready` is the queue
(wave shape: liveness + three independent guards first, then doctor + reap wiring, then the
roadmap §7.4 exit-test E2E).

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

## Strays — cleared 2026-07-03 (the doctor gap, worked by hand)

Every finished-or-near pick the founding audit found stranded is now landed; kept one cycle as
the record of what §3's `weft doctor` should surface automatically instead of by-hand joins.

- **weft-8r4** (#80), **weft-ecj** (#89) — merged; **v0.2.1** cut (#74).
- **weft-aff** (#93) — spec §2 licensing correction, merged into 0.2.2.
- **weft-9i3** (#92) — `stop-server.sh` PID-validation + ownership hardening + `helper.js` XSS
  fix, merged into **0.2.2**. `test_stop_server.py` GREEN 4/4 (needed a detached-spawn fix so
  killed fakes reparent to init instead of lingering as zombies — see engram gotcha). One
  follow-up: the Node/happy-dom XSS test (`helper.test.mjs`) is still unrun (needs `npm install`);
  the `helper.js` fix itself shipped.

## Next concrete step

**Execute the unattended-trust epic (`weft-x38`)** — merge the §7.3 docs PR (spec + plan +
ADRs + this refresh), then work the warp: `bd ready --parent weft-x38` and drain the waves
(each pick's description carries its plan reference; read the plan task verbatim before
starting). Milestone exit test = roadmap §7.4: kill an executor mid-pick and strand a
workspace — one `weft doctor` run surfaces both. Approved parallel task: **delete the legacy
`weft/` tree** (roadmap §9 #2 / §6 "Remove") — tracked as its own bead.

## Open questions / decisions in flight

- **Roadmap §9 decisions (1–5)** — **confirmed by Sean 2026-07-04** (all five as-proposed):
  milestone = unattended-trust hardening; delete legacy `weft/` tree; fovea-onboard = v1.0 exit;
  materialize deferred scope now; release cadence = milestone-or-2-weeks. No longer in flight.
- **weft-9i3 test approach** — resolved: kept the stdlib-`unittest` PID test (GREEN); the vendored
  script stays unhardened-for-zombies (production has none). The Node/happy-dom XSS test still
  needs a one-time `npm install` to run — outstanding follow-up.
- **Workspace hygiene** — resolved by the §7.3 design: `doctor` *flags* foreign workspaces
  (`worktree-agent-*` leftovers) as a `foreign` finding for manual sweep; `reap` gains a guard so
  it never forgets them (the design round found today's reap would — a live-session hazard).
  Until those picks land, sweep manually.

## Session handoff

A fresh session should read, in order: `roadmap.md` (intent) → `design.md` + `seams/` (what's
built) → this file (where we are). Then `bd ready` for the actual work queue.

- **Landed this cycle:** §7.3 encode-the-plan — unattended-trust milestone designed
  (spec + plan, both reviewer-READY), 3 ADRs captured, epic `weft-x38` + seven picks + edges
  materialized and synced. Earlier: §7.2 honest docs (PR #98 + #99; `weft-a01`), 0.2.2 (#95).
- **Decisions settled:** roadmap §9 (1–5) confirmed 2026-07-04; §7.3 design decisions recorded
  as ADRs `weft-jcg` / `weft-qc0` / `weft-0pq` and in the spec's Decisions section.
- **Next thread:** merge the §7.3 docs PR, then execute the epic's wave 1 (`bd ready --parent
  weft-x38`), plus the approved legacy-`weft/`-tree deletion. Both are fresh work off clean
  `main`.
