<!--
  ~ SPDX-License-Identifier: Apache-2.0
  ~ Copyright 2026 Weft Contributors
-->

# Weft — Roadmap (target state + path)

> Status: **roadmap** — the living steering doc: what weft is *for*, the delta to it, and the
> ordered path. Top of the doc pyramid. · Created/updated: 2026-07-05 ·
> Authors: Sean Brandt (@seanb4t) · Assisted-by: Claude Fable 5
>
> - **Live position** — "where are we right now, what's next" → [`state.md`](./state.md), the
>   session spine. This doc is the *intent*; `state.md` is the *position*; **beads is the atomic
>   work graph** (the source of truth for tasks/status/deps — never mirrored into these docs).
> - **What's built today** → [`design.md`](./design.md) (architecture overview),
>   [`seams/`](./seams/) (per-seam specs), [`adr/`](./adr/) (decisions),
>   [`plans/`](./plans/) + [`superpowers/`](./superpowers/) (increments).
>
> **The bootstrap exception (2026-07):** weft's own methodology forbids ROADMAP/STATE files — the
> warp *is* the plan (CLAUDE.md hard rules; `design.md` §3). This pair exists because weft is not
> yet ready to dogfood itself: it holds the *steering layer* (identity, target, milestone intent)
> that beads structurally does not, in the same lightweight roadmap+state cadence fovea adopted
> (fovea `docs/design/roadmap.md`, 2026-07-03). It never restates bead state, and it **retires when
> weft plans weft** (§2, exit criterion 1) — the exception is self-liquidating.
>
> The §9 decisions were adopted 2026-07-03 as recommended defaults and **confirmed by Sean
> 2026-07-04** (all five as-proposed). They are now settled intent, not provisional.

---

## 1. What weft is (identity)

**Weft is GSD's methodology on real tools.**

GSD Core's value is context engineering — spec → plan → execute → verify → ship, heavy work in
fresh subagent contexts, coordinated by durable state. Its two other layers (a homegrown markdown
tracker, homegrown git-worktree choreography) exist only because nothing better sat underneath.
Weft keeps the methodology and replaces both layers with purpose-built substrates:

- **beads is the brain** — planning, the dependency graph (the warp), task state, scheduling.
  `bd ready` *is* the scheduler.
- **jj is the loom** — workspace-per-pick isolation on a shared commit graph, conflicts as
  first-class data, change-scoped recovery.
- **The engine (`weft`, one Go binary) is deterministic plumbing** between them — it never spawns
  agents and never generates prose. The host runtime (Claude Code plugin: 12 skills, 6 agents)
  does the thinking.

The load-bearing primitive is the **bead↔change spine**: every pick carries its jj change-id as a
bead label, collapsing recovery, audit, and resume-after-compaction into one pointer.

> *beads holds the structure; jj does the weaving; the human works the merge wall.*

## 2. Target state

**v1.0 = trustworthy unattended weaving on someone else's repo.** Three exit criteria:

1. **Weft weaves weft.** A real feature of weft's own is planned and executed end-to-end through
   weft's loop (feature/new-project skill → `plan emit` → shed waves → picks → `finish`) — not
   through the dev-flow meta-tooling that built v0.x. Today the loop has been proven only as the
   scripted seam-10 CI gate plus one live dogfood; the tool has never routinely eaten its own
   cooking. When it does, this doc pair retires onto the warp.
2. **fovea onboards.** The first external repo: `weft onboard` on fovea, whose own interim
   roadmap/state cadence (fovea `docs/design/state.md`) explicitly waits on weft. That is the
   customer commitment that makes "done" externally testable.
3. **Unattended-trustworthy.** A crashed executor, a stranded workspace, or a half-finished wave
   is *detected and reported* by the engine — not discovered by archaeology (§3).

## 3. The remaining architectural move

> **✓ Landed 2026-07-05** (epic `weft-x38`, PR #103). `weft doctor`, `executor_live` liveness,
> `reap`'s foreign-workspace guard, resolver oscillation cap, replan removed-pick I2 guard, and the
> seam-11 `finish reconcile` re-verification all shipped; invariants I1–I4 hold by test. Retained as
> the rationale record — the move from *choreography that works when watched* to *choreography that
> reports when unwatched* is done.

The 2026-07-03 deep review found the engine complete against its spec, with one structural gap:
**weft has per-epic projection (`resume`) but no whole-warp health check.** Three
finished-or-nearly-finished picks sat stranded in workspaces for 11 days — invisible to
`bd ready` (correctly: `in_progress` isn't ready), invisible to `bd blocked` (nothing blocks
them), visible only by manually joining bead state against jj workspace state. That join is
exactly what the deferred scope owns:

- **`weft doctor`** — the whole-warp join: `in_progress` beads × workspace state × change state ×
  PR state → strays, orphans, divergence (seam 7 deferred scope).
- **`executor_live` liveness** (seam 3 §8) — a crashed executor currently over-retains its
  workspace; `reap` cannot tell dead from busy.
- **Resolver oscillation guard** (seam 4 §8), **replan removed-pick supersede** (seam 2 §8),
  **`finish reconcile` merge-commit-branch re-verification** against the forest + parked-escalated
  shape (seam 11 §8) — the remaining "unattended goes wrong" edges.

The primitive vocabulary (spine, overlap forest, conflict-as-data) is done and does not change.
The move is from *choreography that works when watched* to *choreography that reports when
unwatched*.

## 4. What's there (assets to build on — do not rebuild)

- **Engine, all 11 seams shipped:** `plan check|emit` (first-emit, replan, phased/JIT roadmap,
  field-drop preflight), `shed form|isolate|integrate|cleanup`, `pick seal|verify|land|redo`,
  `conflict open|finalize` (with escalation), `finish open|reconcile` (both merge styles),
  `resume`, `reap`, `ws`, `install`, `version`. ~12.4k LOC, injectable Runner, cobra+toml only,
  uniform `--json` envelope + `--pick` extraction, typed exit codes.
- **Overlap-aware integration** — file-overlap forest of linear sub-stacks (seam 11), not one
  serial stack.
- **Plugin** — 12 skills covering the whole Layer-A loop (explore, new-project, onboard, feature,
  discuss, plan-phase, phase-driver, sketch, ui-phase, spike, execute, verify-work), 6 agents,
  SessionStart orientation hook.
- **Release pipeline** — release-please + GoReleaser, single `vX.Y.Z` tag versions binary and
  plugin in lockstep, strict plugin validation in CI.
- **Proof discipline** — integration-gated weave-loop E2E in CI against pinned `bd`+`jj`;
  25 adversarial per-PR review epics with findings driven to closure; 31 machine-captured ADRs;
  committed self-hosting verify gate (`.weft/config.toml`).

## 5. What's not there (the delta to target)

Ordered by how directly it serves the target:

- **The §3 unattended-trust set** — **✓ landed 2026-07-05** (`weft-x38` / #103): `doctor`,
  `executor_live`, oscillation guard, replan supersede, seam-11 reconcile re-verification.
- **Self-dogfood:** the meta-loop (exit criterion 1), plus the deferred re-runnable dogfood
  harness and the prompt↔verb drift guard (seam 10 §8).
- **Onboard-readiness for fovea:** whatever `weft onboard` hits on a real repo that isn't weft —
  unknown-unknowns are the point of exit criterion 2.
- **A warp-encoded plan** — **✓ done** (§7.3): the §3 scope was materialized into epic `weft-x38`
  and drained to completion (#103). The live deltas are now self-dogfood (exit criterion 1) and
  fovea onboard-readiness (exit criterion 2).
- **Pulled-by-need infra:** golangci-lint; multi-arch/signing/Homebrew (seam 8 §8); config schema
  refinements (`**` globs, per-workspace conflict-marker style); the warp-plan JSON Schema
  (deferred until a third-party authoring surface exists, seam 9); non-Claude host runtimes
  (seam 7 §8).

## 6. What needs to go away (fix · demote · remove)

**Fix — the doc corpus lies about the project (highest priority):**

- "No implementation exists yet" survives in `design.md:10` and the seam 01–07 headers — eight
  documents contradicted by the shipped engine *and by later seams* (seam 8 says "Seam 7 shipped
  `weft install`" while seam 7's header denies it). Refresh every header to shipped-status.
- `design.md` §9 still describes a five-seam roadmap; eleven exist. §9 gets demoted to history,
  superseded by this doc.
- CLAUDE.md's "There is no ROADMAP.md / STATE.md" hard rule needs the bootstrap-exception
  carve-out (coordinate with in-flight PR #89, which touches the same file).

**Demote:**

- **dev-flow as weft's own build loop.** It built v0.x well, but every loop it runs is a loop
  weft's skills didn't. Exit criterion 1 replaces it for weft's own feature work; dev-flow remains
  for meta-work on the loop itself.

**Remove:**

- **The legacy `weft/` prompt tree** — 15 tracked files, unshipped, unvalidated, CI-fenced,
  undocumented at top level; a newcomer cannot tell it from `plugin/`. **✓ confirmed: delete**
  (Sean 2026-07-04; git history preserves it; `plugin/` is the product). See §9.

**Process fix:**

- **The solving-a-bead stranding mode:** a session that ends at the leave-`in_progress` handoff
  (ADR `fhsk-hj3` pattern) strands finished work with no PR and no signal. Interim mitigation: the
  SessionStart hook flags `in_progress` beads whose workspaces have gone stale. Real fix:
  `doctor` (§3).

## 7. Sequencing (the path, not a schedule)

1. **Land + release.** Merge the strays (PR #80; describe/push/PR `weft-aff`; finish `weft-9i3`
   from its RED test; PR #89), then cut v0.2.1 (PR #74). Housekeeping debt to zero.
2. **Honest docs.** This pair lands; `design.md` + seam headers refreshed; CLAUDE.md carve-out;
   legacy-tree decision enacted.
3. **✓ Encode the plan.** Materialized §5's deferred scope into epic `weft-x38` (the first open
   epic since `weft-ccy` closed); drained in step 4.
4. **✓ Unattended-trust milestone (§3).** Landed 2026-07-05 (epic `weft-x38`, PR #103; 9 picks,
   invariants I1–I4 by test). **← steps 1–4 done; step 5 is next.**
5. **Weft weaves weft.** First real self-hosted feature through the full loop; fix what it breaks;
   retire this doc pair onto the warp.
6. **Onboard fovea.** Exit criterion 2; fovea's interim cadence retires too.
7. **Pulled-by-need infra** (§5, last bullet) — only as real usage demands.

## 8. Non-goals (explicit cuts)

- **Not an agent runtime.** The engine never spawns agents, never generates prose (design.md §7).
  No LLM SDK in `go.mod` — that is a feature, not a gap.
- **Not a git tool.** jj (colocated) is the substrate; no mutating-git compatibility mode, ever.
- **Not GSD-compatible.** Clean-room reimplementation of the methodology; no `.planning/` import,
  no gsd-core interop.
- **Not markdown-planned** — beyond this bootstrap pair. The warp is the plan; these docs never
  hold task state and they retire at exit criterion 1. No SUMMARY.md, ever.
- **Not multi-host yet.** Claude Code is the only target runtime until a real need appears
  (seam 7 deferred).

## 9. Decisions (confirmed 2026-07-04)

Adopted 2026-07-03 as recommended defaults to make this roadmap actionable; **confirmed by Sean
2026-07-04** (all five as-proposed, asked 2026-07-03). The rationale is kept as the decision record.

1. **Next milestone = unattended-trust hardening (§7.4)** — over adoption/DX-first or
   new-capability-first. Rationale: it is the review's one structural gap, and both dogfood exits
   (§2.1, §2.2) depend on it. **✓ confirmed 2026-07-04.**
2. **Legacy `weft/` tree: delete** rather than document (§6). **✓ confirmed 2026-07-04.**
3. **v1.0 exit = fovea onboarded** (§2) — makes "done" externally testable rather than
   self-declared. **✓ confirmed 2026-07-04.**
4. **Materialize deferred seam scope into the warp now** (§7.3), even where priority is P4 —
   parked-and-visible beats prose-and-forgotten. **✓ confirmed 2026-07-04.**
5. **Release cadence: cut on milestone completion or two weeks, whichever comes first** — the
   three-week v0.2.1 drift (features + security-adjacent fixes sitting unreleased) should not
   recur. **✓ confirmed 2026-07-04.**
