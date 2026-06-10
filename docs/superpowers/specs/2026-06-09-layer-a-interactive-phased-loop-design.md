<!--
  ~ SPDX-License-Identifier: Apache-2.0
  ~ Copyright 2026 Weft Contributors
-->

# Restoring the GSD Layer-A interactive/phased loop — design

**Date:** 2026-06-09
**Bead:** `weft-ccy` (epic)
**Status:** Approved by Sean (brainstorming session); pending design-review gate
**Supersedes nothing; refines:** ADR `weft-cfp` (v1 interaction model: deferral, not compression)

## Context

weft v1 ships the front-loaded spine: `new-project` (one-shot Q&A → research →
warp emission) + autonomous `execute`. GSD's per-phase interactive rhythm
(discuss → plan → execute → verify/UAT → ship, repeated per phase) was deferred
in seam-5 scoping. ADR `weft-cfp` records that this was scope deferral, not a
design stance, and names epic `weft-ccy` as the additive restoration path.

The 2026-06 dogfood validated planning quality (epic + 22 picks + 42 edges,
correct wave decomposition) but surfaced three gaps:

1. No per-phase interactive rhythm after plan approval (discuss / UAT / ship-per-phase).
2. No lightweight entry for small features against an existing codebase
   (`new-project` correctly refused "add weft doctor" but had nowhere to send it;
   the full flow is ~28 min / ~125k tokens — wrong tool for a small feature).
3. No path to onboard an existing repo that has never run weft.

### Grounding findings (recorded as `bd note` traces on `weft-ccy`)

- **beads natively supports phase sub-epics:** nesting to 3 levels;
  `bd ready --parent` recurses into grandchildren; only `blocks` edges gate
  readiness (parent-child edges do not). The scheduler substrate needs no change.
- **But the emission layer does:** the warp-plan schema is deliberately
  single-epic (seam-2 §6, `plan.EpicKey = "@epic"`); `plan emit`'s create path
  does not echo the created epic id; the re-plan upsert does not yet apply
  new-pick dependency edges (§8 sub-seam). Epic `weft-ccy.3`'s "prompt-layer
  only" claim is therefore partially wrong — phased emission touches the engine.
- **Existing invariant that shapes the design:** epic = ship unit = one PR
  (`weft finish` operates on an epic). Phase sub-epics therefore ship as one PR
  per phase — exactly GSD's rhythm.

## Decisions made in this session

| Decision | Choice |
|---|---|
| Scope | Everything: core loop (ccy.1–3) + feature/onboard entries + ccy.4 upstream surface |
| Loop driver | Both, layered: discrete gate skills first; thin driver skill later |
| Entry points | Two new skills: `feature` and `onboard` |
| Plan timing | **Just-in-time per phase** — roadmap up front, picks planned per phase after its discuss |
| Phasing mode | **Default / auto-discovered, no flag** — single-phase plans degenerate to today's one-shot shape |

## 1. The interaction model (spine)

Three entry skills, one per-phase rhythm, one later driver.

**Entries:**

- `new-project` — greenfield. The planner always decomposes into phases; there
  is no `--phased` flag. **Single-phase degenerate case:** when decomposition
  yields exactly one phase, project epic = phase epic, picks are planned
  immediately, and emission produces exactly today's shape (epic + picks) —
  the current one-shot flow survives as the degenerate case, not a mode.
- `feature` (new) — lightweight entry for incremental work on a codebase weft
  already manages.
- `onboard` (new) — make an existing repo weft-ready, then hand off to
  `feature` or `new-project`.

**Per-phase rhythm** (multi-phase projects), per phase sub-epic:

```text
discuss → plan-this-phase (JIT picks into the phase sub-epic)
        → execute --epic <phase> → verify-work → finish   (phase = one PR)
```

**Default ending change:** today `new-project` always ends "warp emitted, run
execute." Under this design a multi-phase project ends "roadmap emitted, run
discuss on phase 1." This is deliberately more interactive by default — the
restoration this epic exists for. A fully-autonomous multi-phase weave requires
either answering discuss per phase or the driver skill's later auto mode.

**Driver** (layered later): a thin skill that walks the rhythm phase-by-phase,
pausing at the two interactive gates (discuss, verify-work). Pure composition
of the gate skills + existing verbs; no new semantics.

## 2. Engine changes (the one engine touch)

Three small, testable changes to the emission layer:

1. **warp-plan schema vNext — optional `phases[]` block.** Each phase carries
   title, goal/description, acceptance, and `needs` (inter-phase edges). When
   `phases[]` is present, `plan emit` creates project epic → phase sub-epics →
   inter-phase `blocks` edges, with no picks (roadmap emission). When absent,
   behavior is exactly today's (single epic + picks). `phases[]` is the natural
   encoding of discovered phasing, not an opt-in flag. Note: `Validate`
   (`internal/plan/plan.go`) currently rejects plans with zero picks; that rule
   must be conditionalized when `phases[]` is present.
2. **`plan emit` echoes created epic ids** in the `data` envelope (today the
   create path emits mode/created/edges/… but not the epic bead id; skills must
   query around the gap).
3. **Complete the §8 re-plan sub-seam:** apply new-pick dependency edges on
   upsert. Per-phase pick emission then *is* the existing re-plan path targeted
   at the phase sub-epic (`plan emit <file> --epic <phase-id>`) — no new
   emission mode.

Engine work follows house conventions: injectable runner for subprocess calls,
empty-list JSON contract (`[]`, never null), unit tests per change.

## 3. `discuss` skill (weft-ccy.1)

Interactive HOW-shaping scoped to **any epic id** — phase sub-epic or feature
epic; deliberately not gated on phasing, so it delivers value on today's
one-shot epics immediately. Adaptive questions about implementation decisions,
library/config preferences, and conventions. Persists decisions to the epic's
bead `design` field + notes (beads is the brain; no CONTEXT.md). The per-phase
planner consumes these when planning that phase's picks. Port source:
`/gsd-discuss-phase` (GSD Core, MIT), rewritten to bead-backed state.

## 4. `verify-work` skill (weft-ccy.2)

Interactive UAT skin over verify-data. Enumerates the phase's deliverables
with an explicit fallback chain — phase epic acceptance criteria → closed
picks' acceptance criteria → phase goal/description as last resort (the
bead-backed analog of GSD's `PLAN.md must_haves` → `ROADMAP.md
success_criteria` → phase-goal chain) — and walks the human
through them y/n one at a time, auto-diagnoses failures, and files fix picks
(new beads) under the phase epic. Complements — never replaces — the machine
`pick verify` gate (verdict-as-data, exit 0). Also works on any epic today.
Port source: `/gsd-verify-work`.

## 5. `feature` skill (new)

The lightweight front door the dogfood found missing. Brief Q&A (no 4-agent
research fan-out — at most one codebase-recon pass), inline HOW questions
(discuss-style, reusing the discuss skill inline), planner emits **one epic +
picks against the existing code**, approval gate, emit, then suggest `execute`.
Target: minutes, not ~28. `new-project` refers non-new-project requests here
instead of dead-ending.

## 6. `onboard` skill (new)

Minimal v1: make an existing repo weft-ready — `bd init`, seed conventions and
memories from a codebase-mapping pass, then hand off to `feature` or
`new-project`. No planning of its own. Port source: `/gsd-map-codebase`
(GSD Core, MIT), deliberately compressed — GSD's four parallel mapper agents
emitting seven `.planning/codebase/*.md` files become a single mapping pass
seeding bead-backed conventions/memories (beads is the brain).

## 7. explore / spike / sketch (weft-ccy.4)

Stays last, as the epic already says. Port `/gsd-explore`, `/gsd-spike`,
`/gsd-sketch` with bead-backed outputs (seeds/notes; no `.planning/` files)
once the core loop exists.

## 8. Epic reshape + sequencing

`weft-ccy` is reshaped into phases (dogfooding the phased structure on itself):

| Phase | Content | Why this order |
|---|---|---|
| **A** | discuss (ccy.1) + verify-work (ccy.2) | Prompt-only, zero engine deps, immediate value on one-shot epics |
| **B** | Engine enablers: schema `phases[]`, emit epic-id echo, §8 re-plan edges | Unblocks phasing; small, testable Go work |
| **C** | Phased planner + auto-discovered phasing in `new-project` (ccy.3 reshaped: JIT roadmap, no flag, single-phase degenerate case) | Needs B |
| **D** | `feature` + `onboard` skills | Reuses discuss inline; independent of C |
| **E** | Driver skill | Composes A + C ("both, layered") |
| **F** | ccy.4 (explore/spike/sketch) + a dogfood re-run as the validation gate | Proves the parity claim |

Bead reshape actions: update `weft-ccy` acceptance (drop "opt-in phased mode"
language); rewrite `weft-ccy.3` to the JIT/auto-discovered design; add children
for engine enablers, feature, onboard, and driver; wire `blocks` edges
(engine enablers → ccy.3; ccy.1 + ccy.3 → driver; ccy.1 → feature;
ccy.1/.2/.3 → ccy.4). Pick-level decomposition happens per phase, just-in-time
— consistent with the design itself.

## ADR impact

ADR `weft-cfp`'s consequence line "an opt-in `--phased`/`--interactive` mode
can coexist with the current one-shot path" needs an addendum: phasing is
auto-discovered, and the one-shot path survives behaviorally as the
single-phase degenerate case rather than as a separate mode. Capture via
`/capture-adrs` after the plan phase (expected ADRs: auto-discovered phasing +
degenerate case; JIT per-phase planning; roadmap emission contract). Until the
addendum lands, ADR `weft-cfp`'s consequence line contradicts this spec — the
spec is authoritative; filing the addendum is part of phase A/B acceptance.

## Testing & validation

- **Skills (prompt layer):** existing plugin gates (`claude plugin validate`
  strict, both manifest paths; intra-tree path-citation rewrite grep) + the
  phase-F dogfood re-run against the GSD walkthrough.
- **Engine:** unit tests per change — schema vNext parse/validate (phases[]
  present/absent), emit epic-id echo, re-plan new-pick edge application,
  empty-list contract tests.
- **Validation gate (phase F):** a dogfood run of a multi-phase project
  exercising the full rhythm, plus a small-feature run through `feature`.

## Out of scope

- Porting GSD's remaining command surface beyond the named skills (67 commands;
  seam-5 discipline: port additively, per need). In particular GSD's optional
  `/gsd-ui-phase` inter-step (discuss → UI spec → plan) is deliberately not
  ported in this round.
- Engine-side interactivity (the engine stays non-interactive; all interaction
  lives in the skill/prompt layer).
- Autonomous multi-phase weave without human gates (arrives only as the driver
  skill's explicit auto mode, phase E).
