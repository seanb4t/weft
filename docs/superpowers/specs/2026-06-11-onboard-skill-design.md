<!--
  ~ SPDX-License-Identifier: Apache-2.0
  ~ Copyright 2026 Weft Contributors
-->

# `onboard` skill — make an existing repo weft-ready (ccy.7) — design

**Date:** 2026-06-11
**Bead:** `weft-ccy.7` (child of epic `weft-ccy`, Phase D)
**Status:** Approved by Sean (brainstorming session); pending design-review gate
**Refines:** `docs/superpowers/specs/2026-06-09-layer-a-interactive-phased-loop-design.md` (master spec §6)
**Depends on:** `weft-ccy.6` (feature — DONE; this refines its precondition)

## Context

The 2026-06 dogfood named three gaps; this closes the third: **no path for a repo
that has never run weft.** `onboard` is the minimal door that makes an existing,
unmanaged repo weft-ready, then hands off to `feature` or `new-project`. It does no
planning of its own.

### What GSD does today (grounding via deepwiki on `open-gsd/gsd-core`)

GSD's analogue is **`/gsd-map-codebase`**: four parallel `gsd-codebase-mapper` agents —
**Tech** (stack + integrations), **Architecture** (patterns + structure), **Quality**
(conventions + testing), **Concerns** (tech debt) — emitting **seven**
`.planning/codebase/*.md` files (STACK, ARCHITECTURE, CONVENTIONS, CONCERNS, STRUCTURE,
TESTING, INTEGRATIONS). Subsequent commands (`new-project`, `plan-phase`, `execute`)
read those files to stay conventions-aware. GSD also has a `--fast` single-mapper mode.

### How weft translates it (the two invariants)

1. **beads is the brain.** weft has no `.planning/`. GSD's seven map files become
   `bd remember` entries — and `bd remember` memories are **already injected at session
   start by the beads `bd prime` hook** (verified live: this session's startup surfaced
   "Persistent Memories (9)"). So onboard needs no CLAUDE.md prose and no new hook to
   make the map discoverable — it rides the existing prime path.
2. **Deliberately compressed.** GSD's four parallel mappers become **one** `Explore`
   pass covering the four axes (master spec §6).

## Decisions made in this session

| Decision | Choice |
|---|---|
| Mapping mechanism | **One `Explore` subagent** over GSD's four axes (tech+integrations / architecture+structure / conventions+testing / concerns) → one structured digest. |
| Seed target | **`bd remember` entries** (per-axis) **+ a `weft-orientation` memory**; surfacing rides the existing beads `bd prime` SessionStart hook. No CLAUDE.md prose, no new weft hook. |
| Routing-triad fix | **Relax `feature`'s precondition to `.beads/`-present** — delete its empty-warp→`new-project` branch — so an onboarded repo isn't bounced to the wrong door. |
| Weft SessionStart hook | **Out of scope — separate follow-up bead.** A weft-owned hook earns its keep only for *dynamic* surfacing (live warp status) or beads-plugin-independence; neither is needed for minimal-v1 onboard. |
| jj colocation | **Out of scope** (master spec §6 scopes onboard to bd + map + seed). A closing note points to `jj-init` for VCS readiness. |

## 1. Shape & scope

Two prompt artifacts; purely prompt-layer, no engine/Go change:

| File | Change |
|---|---|
| `plugin/skills/onboard/SKILL.md` (**new**) | `bd init` → one-pass codebase map → seed `bd remember` + weft-orientation → route to `feature`/`new-project`. |
| `plugin/skills/new-project/SKILL.md` is untouched; `plugin/skills/feature/SKILL.md` (**modify**) | Relax Phase 0 precondition to `.beads/`-present. |

A separate follow-up bead is filed for the weft SessionStart hook (not built here).

## 2. The flow (`/weft-onboard`)

1. **Precondition / idempotency.** Confirm the repo is *not* already weft-managed: if
   `.beads/` is present, say so and point to `feature` (incremental) or `new-project`
   (greenfield), then exit. (The inverse of `feature`'s Phase 0.)
2. **`bd init`.** Run `bd init --non-interactive -p <prefix>` (prefix derived from the
   repo directory name, or asked if ambiguous). Creates the local beads DB. Minimal v1:
   local-only, no Dolt remote — the user wires sync later. Confirm `.beads/` now exists.
3. **Codebase map — one `Explore` pass.** Dispatch a single `Explore` subagent covering
   the four GSD axes in one pass: **tech stack + integrations**, **architecture +
   structure**, **conventions + testing**, **concerns / tech-debt**. It returns a
   structured digest (held in context). Never more than one pass.
4. **Seed bead-backed memory.** Persist the digest as discrete `bd remember` entries,
   one per axis with stable keys — `--key weft-map-stack`, `weft-map-arch`,
   `weft-map-conventions`, `weft-map-concerns` — **plus a `weft-orientation` memory**
   (`--key weft-orientation`): "weft repo — incremental work → `/weft-feature`; greenfield
   → `/weft-new-project`; vocab warp/weft/pick/shed." The beads `bd prime` hook injects
   all of these every future session. (Memories are durable knowledge, not warp issues —
   so this does not populate `bd list`; see §3.)
5. **Hand off.** The repo is now weft-ready (`.beads/` + seeded memories). Present both
   exits — `feature` (incremental work on the existing code) or `new-project`
   (greenfield / first build) — and let the user pick. onboard plans nothing itself.

   Closing note: for weft's VCS verbs (`execute`/`shed`/`pick`, which require a colocated
   jj repo), ensure jj is colocated — see the `jj-init` skill. (Out of scope for onboard.)

## 3. The routing-triad fix (`feature` precondition)

`feature`'s Phase 0 (shipped in ccy.6) has **two** exit branches: `.beads/` absent →
`onboard`; `.beads/` present **but the warp is empty** → `new-project` ("weft-ready but
nothing to build incremental work on"). It proceeds only when `.beads/` is present **and**
`bd list` is non-empty.

A freshly-onboarded repo has `.beads/` + seeded *memories* and an **empty warp** (onboard
does no planning; `bd remember` memories are durable knowledge, **not** `bd list`
issues/epics). So if a user runs `onboard` and then `feature` ("add X"), `feature` hits the
second branch and **bounces them to `new-project`** — defeating the onboard→feature handoff
the master spec calls for. (It is not an infinite loop — the redirect targets `new-project`,
not `onboard` — but it sends the user to the wrong door.)

The conflation: `feature`'s empty-warp branch treats "empty warp" as "nothing to build on,"
but the thing `feature` builds on is the existing **code**, not an existing warp. A
freshly-onboarded repo *has* existing code — `feature` (incremental work on existing code)
is exactly right for it, even with an empty warp.

**Fix: weft-managed = `.beads/` present.** Concretely, in `feature/SKILL.md` Phase 0:
**delete the second exit branch** (`.beads/`-present-but-empty-warp → `new-project`) and keep
only the first (`.beads/` absent → `onboard`). `feature` then proceeds on any repo with
`.beads/`, minting its own epic (`bd create --type epic`) — it needs no pre-existing warp
work. `new-project` stays directly reachable (the user can pick it at onboard's handoff or
invoke it directly); its own Phase 0 is **unchanged** and still correct — it redirects to
`feature` only when the warp is non-empty, so a greenfield request on an empty-warp
(onboarded) repo gets the full greenfield flow. With this change
the three doors are coherent — `onboard` (no `.beads/`), `feature` (`.beads/` present),
`new-project` (greenfield, by user choice) — no dead-ends, no wrong-door bounces.

This is a small, surgical edit to `feature/SKILL.md` Phase 0 (delete one branch, relax the
condition); the routing prose and the rest of the skill are unchanged.

## 4. GSD contrast

| | GSD `/gsd-map-codebase` | weft `onboard` |
|---|---|---|
| Mapping | 4 parallel agents (1 with `--fast`) | 1 `Explore` pass over the 4 axes |
| Output | 7 `.planning/codebase/*.md` files | `bd remember` entries (beads is the brain) |
| Surfacing | subsequent commands read the files | `bd prime` hook injects memories every session |
| State setup | conventions-aware planning | memories seeded + repo is weft-managed |
| Scope | full map command (+ drift remap modes) | minimal onboarding door only |

## 5. Testing & validation

Prompt-layer only:

- **Plugin gates:** `claude plugin validate ./plugin --strict` + `. --strict` +
  grep-discipline (`grep -RnE 'weft/(agents|references|workflows)/' plugin/` → no matches;
  `${CLAUDE_PLUGIN_ROOT}` for any intra-plugin path).
- **Manual dogfood (the real coverage):** in a throwaway non-weft scratch dir, run
  `onboard` — confirm `bd init` creates `.beads/`, the Explore pass runs once, the digest
  seeds `bd remember` entries (verify via `bd memories` / a fresh `bd prime`), and the
  closing handoff offers `feature`/`new-project`. Then confirm `feature` (with the relaxed
  `.beads/`-present precondition) **accepts** the onboarded-but-empty-warp repo and mints
  an epic — proving it no longer bounces the user to `new-project`.

No Go changes; no unit tests.

## 6. ADR impact

One likely ADR to capture post-plan via `/capture-adrs`:

- **onboard as compressed single-pass mapping → `bd remember` (riding the `bd prime`
  hook), plus the `feature` precondition relaxed to `.beads/`-present** to close the
  three-door routing triad. (Complements `weft-yup`, which established feature's
  adaptive-composition + routing; this records the onboard mapping-compression choice and
  the precondition correction.)

## Out of scope

- **The weft SessionStart hook** — dynamic warp-status / beads-plugin-independent
  orientation. Filed as a separate follow-up bead; minimal-v1 onboard rides the existing
  `bd prime` hook.
- **jj colocation** — onboard does bd only; `jj-init` owns VCS setup (closing note points
  there).
- **Dolt remote / sync setup** — local `bd init` only; the user wires a remote later.
- **GSD's drift-remap modes** (`--paths` / `last_mapped_commit`) and `--focus` flags —
  not ported this round.
- **Any planning** — onboard creates no epic/picks; that is `feature` / `new-project`.
- **Any engine change** — purely prompt-layer.
