<!-- markdownlint-disable MD013 -->
<!-- adr-render: source=bd:weft-7nr; do not edit manually; use `/adr update weft-7nr` -->

# onboard: single-pass codebase map to bd remember; feature precondition = .beads-present

**Date:** 2026-06-11
**Status:** Accepted
**Decision:** weft-7nr
**Deciders:** Sean Brandt

## Context

The `onboard` skill (ccy.7) makes an unmanaged repo weft-ready without creating file-based map artifacts. GSD's analogue (`/gsd-map-codebase`) uses four parallel mappers emitting seven `.planning/codebase/*.md` files that subsequent commands read. weft has no `.planning/` layer — beads is the brain — and the beads `bd prime` SessionStart hook already injects `bd remember` memories every session. Separately, ADR `weft-yup` (feature skill) routed BOTH a no-`.beads/` repo AND an empty-warp repo to `onboard`; but a freshly-onboarded repo has `.beads/` + seeded memories and an empty warp (onboard seeds memories, not warp issues), so `feature` would bounce the user to `new-project` instead of accepting the repo — defeating the onboard→feature handoff.

## Decision

(1) `onboard` maps the codebase in **one** `Explore` pass over GSD's four axes (stack+integrations / architecture+structure / conventions+testing / concerns), seeds the digest as `bd remember` entries (per-axis keys + a `weft-orientation` memory), and relies on the **existing `bd prime` hook** for session-start injection — no `.planning/` files, no CLAUDE.md prose, no new weft hook. (2) `feature`'s Phase 0 precondition is relaxed to **`.beads/`-present** only — the empty-warp→`new-project` exit branch is deleted — so "weft-managed" is defined by `.beads/` presence alone. This **amends the routing clause of ADR `weft-yup`** (which is otherwise unchanged and remains in force).

## Rationale

- beads is the brain: weft has no `.planning/` layer; `bd remember` is the correct durable substrate for codebase-map knowledge.
- `bd prime` already injects memories at SessionStart (verified live); riding it is zero new infrastructure and guarantees future-session availability without a weft-owned hook.
- One `Explore` pass is proportionate for minimal-v1 onboard; the four GSD axes are coverage-complete in one well-scoped prompt.
- `feature` builds on existing **code**, not a pre-existing warp; "weft-managed = `.beads/`-present" is the correct semantic boundary — correcting the empty-warp conflation in `weft-yup`'s routing clause.
- Deleting the empty-warp→`new-project` branch closes the wrong-door bounce for freshly-onboarded repos while leaving `new-project` directly reachable by user choice at onboard's handoff.

## Alternatives Considered

**Four parallel Explore mappers → seven memories (GSD-parity) — rejected.** Maximum coverage, mirrors GSD — but 4× token cost and premature for minimal-v1; a single well-scoped pass is coverage-complete on typical repos.

**Map → CLAUDE.md prose or a new weft SessionStart hook — rejected.** Human-readable / could surface dynamic warp status — but CLAUDE.md prose drifts silently and isn't machine-queryable; a new weft hook earns its keep only for dynamic surfacing or beads-plugin-independence (neither needed for v1), and riding `bd prime` is zero new infrastructure. (The dynamic-status hook is deferred to follow-up bead weft-ccy.9.)

**Keep `feature`'s empty-warp→`new-project` branch (weft-yup as shipped) — rejected.** Preserves the original routing — but bounces every freshly-onboarded repo to `new-project`, defeating the onboard→feature handoff; conflates "empty warp" with "nothing to build on" when feature builds on existing code.

## Consequences

**Positive:** No `.planning/` file layer to maintain or drift. The onboard→feature handoff works for freshly-onboarded repos. Three-door routing (onboard / feature / new-project) is coherent — no dead-ends, no wrong-door bounces. Zero new infrastructure: no new hook, no new verb, no Go change.

**Negative:** Single-pass coverage may miss depth four specialized agents would surface (no `--fast`/full toggle in v1). Map memories aren't queryable by warp tooling (`bd list`); inspect via `bd memories`. `feature` now accepts any `.beads/`-present repo regardless of warp state.

**Neutral:** A weft-owned SessionStart hook for dynamic warp-status is out of scope (follow-up `weft-ccy.9`). jj colocation, Dolt-remote setup, and GSD drift-remap modes are out of scope for onboard v1.
