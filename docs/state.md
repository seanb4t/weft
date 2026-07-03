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

**Active implementation:** the two remaining strays (weft-9i3, weft-aff) were finished and put
up as PRs #92 / #93 this session; both await merge to cut 0.2.2.

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

## Strays (finished-or-near work, not landed — the doctor gap, found by hand)

- **weft-8r4** — merged (PR #80).
- **weft-ecj** — merged (PR #89).
- **v0.2.1** — released (PR #74 merged; 0.2.1 cut).
- **weft-aff** — finished 2026-07-03: doc fix rebased onto 0.2.1, PR #93 open, awaiting merge.
- **weft-9i3** — finished 2026-07-03: `stop-server.sh` PID-validation + ownership hardening
  implemented against the RED test; `test_stop_server.py` GREEN 4/4 (needed a detached-spawn fix
  so killed fakes reparent to init instead of lingering as zombies). PR #92 open, awaiting merge.
  The Node/happy-dom XSS test (`helper.test.mjs`) is still unrun (needs `npm install`); the
  `helper.js` fix itself is in place.

## Next concrete step

Merge PRs #92 (weft-9i3) and #93 (weft-aff); the `fix:` in #92 makes release-please propose
**0.2.2** — merge that release PR to cut it (roadmap §7.1, housekeeping-to-zero). Then §7.2
(doc-refresh beads: `design.md` + seam 01–07 headers still claim "no implementation") and §7.3:
run the unattended-trust milestone through `discuss` → plan → `plan-to-beads` to materialize the
deferred seam scope (roadmap §5) into the warp — the first open epic since `weft-ccy` closed.

## Open questions / decisions in flight

- **Roadmap §9 provisional decisions (1–5)** — pending Sean's confirm/override: milestone choice,
  legacy-tree deletion, fovea-as-v1.0-exit, materialize-now, release cadence.
- **CLAUDE.md carve-out** — done in this working copy (different sections than PR #89's Status
  hunk, so it should merge cleanly); verify at rebase time after #89 lands.
- **weft-9i3 test approach** — stdlib unittest was a sandbox workaround; keep or revisit when the
  session has network.
- **Workspace hygiene** — 7 empty `worktree-agent-*` jj workspaces + 33 `worktree-agent-*`
  bookmarks (Claude Code worktree-isolation leftovers, outside `weft reap`'s bead-linked remit).
  Sweep manually, or teach `reap`/`doctor` to flag foreign workspaces?

## Session handoff

A fresh session should read, in order: `roadmap.md` (intent) → `design.md` + `seams/` (what's
built) → this file (where we are). Then `bd ready` for the actual work queue.

- **weft-9i3 / weft-aff** are PRs #92 / #93. After merge: `jj git fetch`, rebase
  `--skip-emptied`, delete the `worktree-weft-9i3` / `worktree-weft-aff` bookmarks, forget +
  `rm -rf` those two worktrees, and let the beads close on merge. Then merge the 0.2.2 release PR.
- **Bead DB:** no mutations this session; the local Dolt server was down (`bd dolt push` refused),
  but nothing is pending to sync.
- **Two live threads:** (1) confirm the roadmap §9 provisional decisions; (2) after 0.2.2,
  start §7.2/§7.3.
