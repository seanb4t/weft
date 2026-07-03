<!--
  ~ SPDX-License-Identifier: Apache-2.0
  ~ Copyright 2026 Weft Contributors
-->

# Weft — STATE (live position)

> Status: **living** — rewritten as work moves; the synthesis layer, NOT a source of truth. ·
> Updated: 2026-07-03
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

**Active implementation:** none. Housekeeping-to-zero (roadmap §7.1) is **done** — every stray
landed and **0.2.2 is released** (PR #95). The warp has no open picks; next is §7.2/§7.3.

**Recently established (2026-07-03):**

- Deep review of design / warp / code / VCS: design→code coherence confirmed excellent; the one
  structural gap is whole-warp health — per-epic `resume` exists, whole-warp `doctor` does not
  (roadmap §3).
- This roadmap+state cadence adopted, mirroring fovea's, as the bootstrap exception to the
  no-ROADMAP hard rule.
- README staleness fixed (#88, merged); CLAUDE.md fix in flight (#89, owned by a concurrent
  session); `design.md` + seam 01–07 headers still stale and **untracked — needs a bead**
  (roadmap §6 "Fix").
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

Housekeeping-to-zero is done. Next is **§7.2 honest docs** — file a bead to refresh `design.md`
(demote its §9 five-seam roadmap to history) and the seam 01–07 headers that still claim "no
implementation exists", now contradicted by shipped **0.2.2** — then **§7.3 encode the plan**:
run the unattended-trust milestone (roadmap §3) through `discuss` → plan → `plan-to-beads` to
materialize the deferred seam scope (roadmap §5) into the warp — the first open epic since
`weft-ccy` closed.

## Open questions / decisions in flight

- **Roadmap §9 provisional decisions (1–5)** — pending Sean's confirm/override: milestone choice,
  legacy-tree deletion, fovea-as-v1.0-exit, materialize-now, release cadence.
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

- **All landed:** weft-9i3 (#92) + weft-aff (#93) merged, beads closed, worktrees cleaned;
  **0.2.2 released** (#95); the state-refresh (#94) merged. Warp is clean, no open picks.
- **Bead DB:** weft-9i3 + weft-aff were closed locally, but the local Dolt server was down
  (`bd dolt push` refused) — **those two closes are unsynced.** Run `bd dolt start && bd dolt push`
  once the server is up so the warp isn't stranded on this machine.
- **Two live threads:** (1) confirm the roadmap §9 provisional decisions; (2) start §7.2 (honest
  docs) → §7.3 (encode the unattended-trust plan into the warp).
