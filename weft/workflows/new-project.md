<!--
  ~ SPDX-License-Identifier: Apache-2.0
  ~ Copyright 2026 Weft Contributors
-->

<!-- adapted from /gsd-new-project + its companion warp-generation agent (GSD Core, MIT) -->

# new-project workflow

Thin orchestrator for `/weft-new-project`. Drives adaptive questioning,
parallel research, and requirement extraction (adapted from GSD Core's
methodology), then dispatches `weft-planner` to produce a `warp-plan.json`,
validates it, and gates on human approval before materialising the warp.

---

## Phase 1 — Adaptive questioning

**Goal:** fully understand project scope, goals, constraints, and technical
preferences before any research begins.

If a `[project description]` was supplied, treat it as the seed and ask only
the follow-up questions that are genuinely ambiguous. If no description was
provided, open with an exploratory conversation guided by a
"dream extraction" philosophy: ask about the user's desired outcome, their
constraints, their audience, and their preferences for the technology
approach. Continue until you can answer, with confidence, the following:

1. What is the project's primary observable goal — what will a user or
   operator be able to do when it ships?
2. What are the hard constraints (language, platform, deployment model,
   budget, timeline)?
3. What is explicitly out of scope for the first shippable version?
4. What prior art, existing systems, or integrations must be considered?

Keep the questioning adaptive: stop asking when each of the four questions
above can be answered. One round of follow-up is usually sufficient; do not
over-clarify.

---

## Phase 2 — Parallel research

**Goal:** surface ecosystem context that the user may not know, grounding
the requirements in reality.

Dispatch four parallel research agents (or, in a single-context session,
four sequential research passes), each scoped to one investigative axis:

| Axis | What to investigate |
|---|---|
| **Technology stack** | Available libraries, frameworks, and tools that match the constraints; maturity and community health. |
| **Feature landscape** | What similar projects provide; what gaps exist; what users typically expect. |
| **Architecture patterns** | Common structural approaches; trade-offs relevant to the stated constraints. |
| **Risks and pitfalls** | Known failure modes, scaling cliffs, licensing issues, or integration hazards. |

The host runtime dispatches these as parallel agent invocations (Claude Code
`Agent` tool, or equivalent — host dispatch is the host's concern; this
workflow describes the step, not the plumbing). Each investigative pass
produces a brief synthesis of findings (held in context, not written to
files). When all four passes are complete, synthesise them into a unified
research summary held in working context.

---

## Phase 3 — Requirement extraction

**Goal:** derive a scoped, categorised requirement set from the conversation
and research findings.

From the questioning transcript and research synthesis, extract requirements
and categorise them:

- **v1 (must-have):** required for the project's primary observable goal.
- **v2 (future):** desirable but not blocking v1.
- **out-of-scope:** explicitly excluded.

Assign each v1 requirement a short, stable ID (e.g. `R-001`). Confirm the
list with the user before proceeding to planning. Requirements are held in
context only — they feed directly into the weft-planner dispatch; they are
not written to separate markdown files.

---

## Phase 4 — Dispatch weft-planner

**Goal:** produce a `warp-plan.json` from the extracted requirements.

Dispatch the `weft-planner` agent with the following context:

- The synthesised research summary.
- The confirmed v1 requirement list (with IDs).
- The project's primary observable goal and stated constraints.

Use model `opus` (from `weft-planner`'s frontmatter `model: opus` field,
per the `model:*` convention). Host-runtime agent dispatch (Claude Code
`Agent` tool, etc.) is the host's concern; this step describes what to pass,
not how the host wires it.

The agent's sole output artifact is `warp-plan.json`. It applies
Goal-Backward decomposition, vertical-slice bias, file-ownership dependency
reasoning, wave thinking, and TDD-mode heuristics per its own methodology
(see `weft/agents/weft-planner.md`).

---

## Phase 5 — Validate: weft plan check

Run the engine validator against the produced plan file:

```
weft plan check warp-plan.json
```

The engine exits 0 and returns a JSON envelope on both valid and invalid
plans. Inspect `data.valid`:

```json
{ "ok": true, "verb": "plan.check", "data": { "valid": true, "issues": [] } }
```

- If `data.valid` is `true`: proceed to Phase 6.
- If `data.valid` is `false`: present `data.issues` to the user, return to
  the `weft-planner` agent with the issues list for a revised
  `warp-plan.json`, and re-run this phase.

Do not proceed to Phase 6 with an invalid plan.

---

## Phase 6 — Human approval gate: weft plan emit --dry-run

Present the full warp preview to the human for approval:

```
weft plan emit warp-plan.json --dry-run
```

This command exits 0 and prints the complete preview — epic, issues, edges,
and any warn-and-tolerate overlap notes — without mutating any state. The
dry-run output is the **human approval gate**: present it in full and wait
for explicit go-ahead before proceeding.

If the human requests changes: return to Phase 4 (or Phase 3 if the
requirement scope changed), produce a revised `warp-plan.json`, and re-run
Phase 5 and Phase 6.

---

## Phase 7 — Materialise the warp: weft plan emit

On explicit human approval of the dry-run preview:

```
weft plan emit warp-plan.json
```

This materialises the warp atomically — the epic, all picks as beads, and
all dependency edges are created in beads. Beads is now the source of truth.
`warp-plan.json` is a transient artifact; it may be discarded or archived.

Confirm to the user that the warp is live and that `bd ready` will list the
first set of ready picks (a shed is woven from them with `weft shed form` when
execution begins).

---

## What this workflow does NOT do

- It does not write GSD's planning-state markdown artifacts (the beads warp
  is the plan; beads is the brain).
- It does not call any external initialisation or state-management tooling
  from prior layers; all state transitions go through `weft` verbs and `bd`
  mutations.
- It does not manage host-runtime dispatch mechanics (agent spawning,
  tool-list construction, context injection) — those are the host runtime's
  responsibility per design §7.
