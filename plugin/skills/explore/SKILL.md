---
description: Socratic ideation for a vague idea — probing WHAT-shaping questions, optional light research, outputs forward ideas as deferred seed beads and durable context as bd remember. Pre-project; hands off to new-project/feature.
argument-hint: "[topic]"
---

<!--
  ~ SPDX-License-Identifier: Apache-2.0
  ~ Copyright 2026 Weft Contributors
-->

<!-- adapted from /gsd-explore (GSD Core, MIT), rewritten to bead-backed state -->

# explore workflow

The front of the funnel: open-ended Socratic ideation **before** there is a
project or epic. weft's analog of `/gsd-explore`. It shapes WHAT (is this worth
building, and what is it?) — the complement of `discuss`, which shapes HOW a
known thing gets built. It plans nothing itself; it routes the conversation's
outputs to beads and hands off.

Beads is the brain: forward-looking ideas become **deferred `seed` beads** in the
warp backlog; durable context becomes **`bd remember`** memories. No `.planning/`
files.

> Note: this `explore` *skill* is distinct from the `Explore` *subagent* (a Task
> tool agent used for codebase recon). This skill MAY dispatch that subagent for
> its optional research pass below.

---

## Phase 1 — Socratic conversation

Treat the optional `[topic]` as the seed. Run a Socratic conversation —
**2–5 single questions, one at a time** — probing constraints, users, scope,
tradeoffs, dependencies, and risks. Same per-question rhythm as `discuss`
(2–3 recommended-choice options where natural), but aimed at WHAT/whether, not
HOW. Stop once the idea is clear enough to either seed or hand off; do not
over-interrogate a small idea.

If a clear scope-creep tangent appears, note it as a candidate seed (Phase 3)
rather than chasing it now.

---

## Phase 2 — Optional research (≤1 pass)

When a factual question or technology comparison arises mid-conversation that you
cannot answer from context, offer **one** lightweight research pass: dispatch a
single `Explore` subagent with the specific question, and hold its digest in
context. This is deliberately **not** `new-project`'s 4-agent fan-out — explore
is ideation, not requirements research. Never dispatch more than one pass.

---

## Phase 3 — Route outputs to beads

Propose the outputs to the user, then persist the agreed ones:

- **Forward-looking actionable ideas → deferred `seed` beads.** One bead per
  idea, carrying its trigger condition in the description:

  ```
  ID=$(bd create --type task --labels seed \
        --title "<short idea title>" \
        --description "<idea>. Trigger: <when this should be picked up>." \
        --json | jq -r .id)
  bd defer "$ID"
  ```

  `bd create` requires `--title` and has no `--status` flag, so defer is the
  second step; `bd defer`
  moves the seed out of `bd ready` until it is undeferred/promoted. Seeds live in
  the warp backlog as schedulable work-in-waiting.

- **Durable context / decisions → `bd remember`** (project-global, surfaced by
  the `bd prime` hook every future session; no epic needed):

  ```
  bd remember "<observation or decision from the ideation>" --key explore-<slug>
  ```

- **Open research questions** → a `seed` bead if it implies work, otherwise a
  `bd remember` note to revisit.

Keep each output concise. Do not invent outputs the conversation did not produce.

---

## Phase 4 — Hand off

explore plans nothing itself. Close by pointing the user onward:

- An idea ready to **build** → `new-project` (greenfield) or `feature`
  (incremental work on an existing weft-managed repo).
- An idea blocked by **feasibility uncertainty** → `spike` (run experiments
  first).
- Otherwise the seeds sit in the backlog (`bd list --label seed`) until a
  trigger fires.

---

## What this workflow does NOT do

- It does not plan or emit a warp — that is `new-project` / `feature`.
- It does not run `new-project`'s multi-agent research fan-out — at most one
  `Explore` pass.
- It does not write `.planning/` files — ideas are deferred `seed` beads, context
  is `bd remember`.
- It does not run feasibility experiments — that is `spike`.
