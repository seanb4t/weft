---
description: Lightweight front door for incremental work on an existing weft-managed repo — brief Q&A, adaptive recon/discuss, one epic + picks against existing code, in minutes.
argument-hint: "[feature description]"
---

<!--
  ~ SPDX-License-Identifier: Apache-2.0
  ~ Copyright 2026 Weft Contributors
-->

<!-- adapted from /gsd-quick (GSD Core, MIT), rewritten to bead-backed state -->

# feature workflow

weft's analog of GSD's `/gsd-quick`: the lightweight entry for incremental work on a
codebase weft already manages. Mints one feature epic, adaptively shapes HOW and recons
the existing code **only when warranted**, then plans and emits a single epic + picks
against the current source — in minutes. Beads is the brain; there is no research
fan-out, no phase discovery, and no sidecar files.

Use `new-project` instead for a brand-new project (it does the full Q&A + research +
roadmap). Use `onboard` to make an unmanaged repo weft-ready first.

---

## Phase 0 — Precondition

Confirm the repo is weft-managed before doing anything else:

```
bd list --json
```

The repo is weft-managed when `.beads/` is present **and** `bd list --json` returns a
non-empty warp (at least one existing epic/issue). If `.beads/` is absent or the warp is
empty, stop and tell the user this repo is not weft-managed yet — point them to
`onboard` (to make it weft-ready) or `new-project` (for a greenfield build) — and exit.

---

## Phase 1 — Brief Q&A → mint the feature epic

Treat the `[feature description]` as the seed. Ask only the follow-up questions that are
genuinely ambiguous — one light round. Do **not** run new-project's "dream extraction";
this is a small, bounded feature. Stop asking once you can state, with confidence:

1. The feature's observable goal — what a user or operator can do once it ships.
2. Its acceptance criteria — checkable conditions for done.
3. What is explicitly out of scope.

Then mint the feature epic directly in beads (it is just a bead — the warp-plan path
cannot emit an epic with no picks):

```
bd create --type epic --title "<feature title>" \
  --description "<observable goal + scope>" \
  --acceptance "<checkable acceptance criteria>" \
  --json
```

Capture the returned epic id from the `--json` output's flat top-level `.id`. All later
phases target this id.

---

## Phase 2 — Adaptive recon (at most one Explore pass)

Decide whether planning this feature needs grounding in the existing code that the
planner cannot cheaply infer (unfamiliar subsystem, non-obvious file ownership,
integration points that must be matched). State your call in one line so the user can
override it.

- **Needs grounding** → dispatch **one** `Explore` subagent scoped to the relevant area.
  Ask it for a recon digest: existing patterns, the files this feature will touch and who
  owns them, integration points, and local conventions. Hold the digest in context for
  the planner. Never dispatch more than one recon pass.
- **Obvious** (e.g. "add a `--version` flag") → skip recon.

---

## Phase 3 — Adaptive discuss

Decide whether the feature has genuine HOW gray areas — library choices, config format,
storage backend, naming/convention decisions a planner would otherwise guess. State your
call in one line.

- **Has gray areas** → invoke the `discuss` skill against the feature epic:
  `discuss <epic-id>`. It settles the gray areas through structured questions and
  persists the locked decisions to the epic's bead `design` field (beads is the brain;
  no CONTEXT.md). The planner reads them in Phase 4.
- **Obvious** → skip discuss.

Recon (Phase 2) precedes discuss deliberately: HOW is shaped better once the code is
known.

Default bias for Phases 2–3: **skip both for obvious features** — that is what keeps this
a minutes-long flow. Escalate only on genuine need.

---

## Phase 4 — Plan (weft-planner, Shape B picks-only)

Dispatch the `weft-planner` agent (model `opus`, per its frontmatter) scoped to the
feature epic. Pass it:

- the epic's goal + acceptance (`bd show <epic-id> --json`),
- the recon digest from Phase 2 (if any),
- the locked HOW decisions from the epic `design` field (if discuss ran).

Instruct the planner to emit **Shape B (single-epic picks)** — this is one cohesive
feature, so it is single-phase by construction; no phase discovery. Its `warp-plan.json`
will carry an `epic` block, but `plan emit --epic` ignores it (only `picks[]` are
processed on the re-plan path), so the planner need not match the minted epic's title.

---

## Phase 5 — Validate + human approval gate

```
weft plan check warp-plan.json
```

Inspect `data.valid`. If `false`, present `data.issues`, return to Phase 4 for a revised
plan, and re-run. Do not proceed with an invalid plan.

```
weft plan emit warp-plan.json --epic <epic-id> --dry-run
```

This computes the upsert diff in-memory against the feature epic without mutating state
(the re-plan dry-run does not call bd) and prints the preview — on a freshly-minted epic,
every pick is a **create**. Present the preview in full as the approval gate and wait for
explicit go-ahead. If the user requests changes, return to Phase 4 (or Phase 3 if the HOW
shifted) and re-run Phases 4–5.

---

## Phase 6 — Materialise + hand off

On approval:

```
weft plan emit warp-plan.json --epic <epic-id>
```

The picks land parented to the feature epic. Confirm the feature is planned and end with:
**"feature planned — run `execute --epic <epic-id>`."** `bd ready` will surface the
feature's first ready picks.

---

## What this workflow does NOT do

- It does not onboard an unmanaged repo — that is `onboard`; `feature` only routes to it.
- It does not run ecosystem research (new-project's 4-agent fan-out) — out of scope.
- It does not discover phases or emit a roadmap — `feature` is always single-epic.
- It does not execute or verify picks — that is `execute` and `verify-work`.
- It does not write planning-state markdown; beads is the source of truth.
