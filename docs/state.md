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

**Milestone:** housekeeping-to-zero + re-encoding the plan (roadmap §7.1–7.3). The 2026-07-03 deep
review (this cadence's founding audit, session `efb0aae7`) established: engine shipped and
spec-complete, doc corpus claiming otherwise, warp with zero forward plan, three stranded picks,
three weeks of release drift.

**Active implementation:** none pending — **§7.2 honest docs landed** (bead `weft-a01`; PR #98
design corpus + PR #99 steering, both merged): `design.md` + all 11 seam headers now read
shipped-status, `design.md` §9 demoted to history. Housekeeping-to-zero (roadmap §7.1) is **done**
(0.2.2 released, PR #95). **§7.3 is next and now unblocked** — roadmap §9 confirmed by Sean
2026-07-04 (see "Open questions").

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

**§7.3 encode the plan** — now unblocked (roadmap §9 confirmed 2026-07-04). Run the
unattended-trust milestone (roadmap §3: `weft doctor`, `executor_live` liveness,
resolver-oscillation guard, replan removed-pick supersede, seam-11 `finish reconcile` re-verify)
through `discuss` → `writing-plans` → `plan-to-beads` to materialize the deferred seam scope into
the warp — the first open epic since `weft-ccy` closed. Approved parallel task: **delete the legacy
`weft/` tree** (roadmap §9 #2 / §6 "Remove") — tracked as its own bead.

## Open questions / decisions in flight

- **Roadmap §9 decisions (1–5)** — **confirmed by Sean 2026-07-04** (all five as-proposed):
  milestone = unattended-trust hardening; delete legacy `weft/` tree; fovea-onboard = v1.0 exit;
  materialize deferred scope now; release cadence = milestone-or-2-weeks. No longer in flight.
- **weft-9i3 test approach** — resolved: kept the stdlib-`unittest` PID test (GREEN); the vendored
  script stays unhardened-for-zombies (production has none). The Node/happy-dom XSS test still
  needs a one-time `npm install` to run — outstanding follow-up.
- **Workspace hygiene** — the three `worktree-weft-*` picks were forgotten + removed this cycle.
  Still ~7 empty `worktree-agent-*` jj workspaces + `worktree-agent-*` bookmarks (Claude Code
  worktree-isolation leftovers, outside `weft reap`'s bead-linked remit). Sweep manually, or teach
  `reap`/`doctor` to flag foreign workspaces?

## Session handoff

A fresh session should read, in order: `roadmap.md` (intent) → `design.md` + `seams/` (what's
built) → this file (where we are). Then `bd ready` for the actual work queue.

- **Landed this cycle:** §7.2 honest docs (PR #98 design corpus + PR #99 steering; bead `weft-a01`
  closed); earlier — weft-9i3 (#92), weft-aff (#93), **0.2.2** (#95). Bead DB synced.
- **Decisions settled:** roadmap §9 (1–5) confirmed 2026-07-04 — the milestone question is closed.
- **Next thread:** §7.3 — materialize the unattended-trust milestone into the warp (`discuss` →
  plan → `plan-to-beads`), plus the approved legacy-`weft/`-tree deletion. Both are fresh work off
  clean `main`.
