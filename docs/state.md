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

**Active implementation:** none in flight beyond the strays below. This session produced
documentation (this doc pair), not code.

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

- **weft-8r4** — done + reviewed; PR #80 open since 06-22; needs merge.
- **weft-aff** — done + review-verified; sitting undescribed/unpushed in `worktree-weft-aff`;
  needs describe → push → PR.
- **weft-9i3** — interrupted mid-TDD RED (network sandbox blocked pytest; pivoted to stdlib
  unittest); ~248 lines incl. XSS/PID hardening sitting in `worktree-weft-9i3`; needs finishing.
- **weft-ecj** — PR #89 open; a concurrent session owns it (and the `claude-status` workspace) —
  do not touch.
- **v0.2.1** — release PR #74 pending since 06-12; holds the sketch/ui-phase skills, the
  SessionStart hook, and the dogfood fixes.

## Next concrete step

Land the strays and merge #74 (roadmap §7.1). Then §7.2 (doc-refresh beads) and §7.3: run the
unattended-trust milestone through `discuss` → plan → `plan-to-beads` to materialize the deferred
seam scope (roadmap §5) into the warp — that run creates the first open epic since `weft-ccy`
closed.

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

- **This pair + grooming infra land together** as bead weft-k2g's PR (steering docs, CLAUDE.md
  edits, SessionStart hook). After merge: `jj git fetch`, rebase `--skip-emptied`, delete the
  bookmark, close weft-k2g.
- **Bead DB:** no mutations this session; nothing pending for `bd dolt push`.
- **Two live threads:** (1) confirm the roadmap §9 provisional decisions; (2) start §7.1
  (land the strays, cut the release).
