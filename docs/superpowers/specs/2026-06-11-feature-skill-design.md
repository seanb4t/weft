<!--
  ~ SPDX-License-Identifier: Apache-2.0
  ~ Copyright 2026 Weft Contributors
-->

# `feature` skill — lightweight incremental-work front door (ccy.6) — design

**Date:** 2026-06-11
**Bead:** `weft-ccy.6` (child of epic `weft-ccy`, Phase D)
**Status:** Approved by Sean (brainstorming session); pending design-review gate
**Refines:** `docs/superpowers/specs/2026-06-09-layer-a-interactive-phased-loop-design.md` (master spec §5)
**Depends on:** `weft-ccy.1` (discuss — DONE), `weft-ccy.3` (phased planner + `plan emit --epic` — DONE)

## Context

The 2026-06 dogfood found weft has **no lightweight entry for small features** against
an existing codebase: `new-project` correctly refused "add weft doctor" but had nowhere
to send it, and its full flow is ~28 min / ~125k tokens — the wrong tool for a small
feature. `feature` is that missing front door.

### What GSD does today (grounding via deepwiki on `open-gsd/gsd-core`)

GSD's small-feature door is **`/gsd-quick`** — not `/gsd-new-project` (new-project /
new-milestone only), and not `/gsd-fast` (≤3-file trivial changes). Key properties:

- `/gsd-quick` **defaults to skipping discuss, research, and validation** for speed.
- Rigor is **opt-in via composable flags**: `--discuss` (lightweight discuss →
  `CONTEXT.md`), `--research` (ecosystem investigation), `--validate` (plan-check +
  post-exec verify), `--full` (all).
- HOW-decisions persist to `CONTEXT.md` (in `.planning/quick/` for quick tasks); GSD's
  planner reads it to avoid re-asking locked questions.

### How weft translates it (the two invariants that shape this design)

1. **beads is the brain.** weft has no `CONTEXT.md` / `PLAN.md` / `.planning/`. The
   epic's bead **`design` field** is weft's `CONTEXT.md` (ratified in ADR `weft-b19`:
   the `design` field is the discuss→planner contract). Picks are weft's `PLAN.md`
   (the bead description IS the plan).
2. **Adaptive over flags.** weft's `discuss` skill explicitly drops GSD's mode flags
   ("v1 ships the default adaptive mode only"). `feature` follows suit: it gates
   discuss/recon behind **adaptive judgment**, not `--discuss`/`--research` flags.

So `feature` *is* weft's `/gsd-quick`, re-expressed on weft's substrate — and it falls
out as a thin composition of pieces already shipped.

## Decisions made in this session

| Decision | Choice |
|---|---|
| HOW-shaping (D1) | **Compose-adaptive** — mint the epic early (`bd create --type epic`), then *adaptively* invoke the **real `discuss` skill** on it (locked HOW → `epic.design`), then plan picks-only and `plan emit --epic`. Genuinely reuses discuss + ccy.3's re-plan path; HOW persists durably. Rejected the "inline discuss-style, single atomic emit" alt: it only reuses discuss's *style*, bakes HOW into picks with no separate locked record, and diverges from GSD's CONTEXT.md model. |
| Recon (D2) | **Explore-adaptive** — at most one `Explore` subagent pass over the relevant existing code → recon digest; skipped when the feature is obvious. |
| Repo precondition | **Require weft-managed** (has `.beads` + a live warp). An unmanaged repo is routed to `onboard` (ccy.7); `feature` does not bootstrap. Keeps it lean and single-purpose. |
| Control model | **Adaptive judgment, no flags** (diverges from GSD's `--discuss`/`--research`); emit is **always gated** by `plan check` + `--dry-run` (no `--validate` opt-out). |

## 1. Shape & scope

Two prompt artifacts:

| File | Change |
|---|---|
| `plugin/skills/feature/SKILL.md` (**new**) | The lightweight front door: brief Q&A → mint epic → adaptive recon → adaptive discuss → planner Shape B → `plan emit --epic` → suggest `execute`. |
| `plugin/skills/new-project/SKILL.md` | Add an opening routing check: incremental work against an existing weft-managed repo → point to `/weft-feature` (close the dogfood dead-end). |

Purely prompt-layer; no engine change. ~80% of `feature` is orchestration over existing
units (`bd create`, `Explore`, `discuss`, `weft-planner` Shape B, `plan emit --epic`,
`execute`).

## 2. The flow

`/weft-feature [description]`, against an existing weft-managed repo:

1. **Precondition check.** Confirm the repo is weft-managed: `.beads/` present **and**
   `bd list --json` returns a non-empty warp (at least one existing epic/issue). If
   `.beads/` is absent or the warp is empty → tell the user and point to `onboard`
   (ccy.7); stop.
2. **Brief Q&A → mint the epic.** Treat the description as the seed; ask only genuinely
   ambiguous follow-ups (one light round — no "dream extraction"). When the feature's
   observable goal, acceptance, and scope are clear, `bd create --type epic` with
   title / description(goal) / acceptance. Capture the epic id. (No requirement-ID
   extraction ceremony — it is one epic.)
3. **Adaptive recon (≤1 Explore pass).** Decide whether planning needs existing-code
   grounding the planner cannot cheaply infer. If yes → dispatch **one `Explore`
   subagent** scoped to the relevant area → recon digest (existing patterns, file
   ownership, integration points, conventions), held in context for the planner. If the
   feature is obvious → skip. Never more than one pass.
4. **Adaptive discuss.** Decide whether the feature has genuine HOW gray areas
   (library / config / convention choices). If yes → invoke the **real
   `discuss <epic-id>` skill** (locked decisions persist to `epic.design`). If obvious
   → skip. Recon precedes discuss deliberately: HOW is shaped better once the code is
   known.
5. **Plan (planner Shape B, picks-only).** Dispatch `weft-planner` scoped to the epic,
   consuming `bd show <epic>` goal/acceptance + the recon digest + `epic.design`
   (discuss decisions, if any). Instruct **Shape B (single-epic picks)** — one cohesive
   feature, no phase discovery. Output a picks-only `warp-plan.json`. (The planner's
   `warp-plan.json` still carries an `epic` block, but `plan emit --epic` ignores it
   entirely — only `picks[]` are processed on the re-plan path — so the planner need
   not match the already-minted epic's title; the epic block is a harmless no-op here.)
6. **Validate + approve.** `plan check` → `plan emit --epic <epic> --dry-run` (the human
   approval gate; in-memory upsert preview, all-creates on a fresh epic) → on approval
   `plan emit --epic <epic>`.
7. **Done.** Picks land parented to the feature epic. End: "feature planned — run
   `execute --epic <epic>`."

## 3. The adaptive gates (the one judgment rule)

This is the core divergence from GSD. GSD gates discuss/recon behind **flags**; weft
gates them behind **adaptive judgment**, and is **transparent** about the call:

- The skill states its decision in one line ("self-contained — skipping discuss and
  recon" / "two gray areas: storage backend + config format — running a quick
  discuss") so the user can override either direction.
- **Default bias: skip both for obvious features** — that is what buys "minutes, not
  ~28". Escalate only on genuine ambiguity (discuss) or a real need for existing-code
  grounding the planner cannot cheaply get (recon).

There are no `--discuss` / `--research` / `--validate` flags. Ecosystem research (GSD's
`--research`) is deliberately absent entirely — that is `new-project`'s 4-agent fan-out,
out of scope here. Validation (GSD's `--validate`) is not optional: weft's emit is
always gated by `plan check` + `--dry-run`.

## 4. `new-project` routing (the refusal pointer)

Add a routing check at `new-project`'s opening: if the request is **incremental work
against an existing weft-managed repo** — the repo has `.beads` + a live warp, and the
ask is "add/change X" rather than "build a new X" — point the user to `/weft-feature`
instead of running the full greenfield flow. This closes the dogfood dead-end
(`new-project` refusing "add weft doctor" with nowhere to send it). The mirror of this
is `feature`'s own precondition check pointing an *unmanaged* repo to `onboard` — three
doors that route to each other rather than dead-ending.

## 5. Composition map

`feature` reuses, in order: `bd create --type epic` (new, tiny) → `Explore` subagent
(adaptive) → `discuss` skill / ccy.1 (adaptive) → `weft-planner` Shape B / ccy.3 →
`plan emit --epic` / ccy.3+ccy.5 re-plan path → `execute --epic` (existing). The only
genuinely new prose is the precondition check, the brief Q&A, and the two adaptive-gate
decisions. Every heavy piece is reused; the heaviest (research fan-out, phase discovery)
is simply absent — which is why it is a minutes tool.

## 6. Testing & validation

Prompt-layer only:

- **Plugin gates:** `claude plugin validate ./plugin --strict` + `. --strict` +
  grep-discipline (`grep -RnE 'weft/(agents|references|workflows)/' plugin/` → no
  matches; use `${CLAUDE_PLUGIN_ROOT}` for any intra-plugin path).
- **Manual dogfood (the real coverage):** run `feature` on the canonical missing case —
  **"add a `weft doctor` subcommand"** — against this repo. Confirm: one epic + picks
  against existing code in minutes (not ~28); research fan-out absent; discuss/recon
  fire only if genuinely warranted; picks parented to the feature epic via
  `plan emit --epic`. Separately confirm `new-project` now routes such a request to
  `feature`.

No Go changes; no unit tests (the engine paths `feature` drives are already covered by
ccy.3/ccy.5 tests).

## 7. ADR impact

One likely planner-policy/workflow ADR to capture post-plan via `/capture-adrs`:

- **`feature` as adaptive bead-native composition** — mint-epic-first to reuse the real
  `discuss` skill (`epic.design` = GSD's `CONTEXT.md`) and `plan emit --epic`, with
  **adaptive gates replacing GSD's opt-in flags**, and a **require-weft-managed**
  precondition that routes to `onboard`. (`weft-b19` covers the design-field contract
  and `weft-cfp` the interaction model; the new part is the adaptive-over-flags +
  epic-first-composition + three-door routing decision.)

## Out of scope

- **Onboarding an unmanaged repo** — that is `onboard` (ccy.7); `feature` only routes
  to it.
- **Ecosystem research** (GSD's `--research` / new-project's 4-agent fan-out).
- **Phase discovery / roadmaps** — `feature` is always single-epic (Shape B); multi-phase
  projects are `new-project`'s domain.
- **A `/gsd-fast`-style ≤3-file trivial path** — not ported this round; a trivial change
  can just be executed directly. (Revisit per need, seam-5 discipline.)
- **GSD's mode flags** (`--discuss`/`--research`/`--validate`/`--full`) — replaced by
  adaptive judgment.
- **Any engine change** — purely prompt-layer.
