<!--
  ~ SPDX-License-Identifier: Apache-2.0
  ~ Copyright 2026 Weft Contributors
-->

# `explore` + `spike` Skills Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add two pre-planning skills — `explore` (Socratic ideation → deferred `seed` beads + `bd remember`) and `spike` (Given/When/Then feasibility experiments in throwaway jj changes → verdict `spike` bead + `bd remember`) — porting `/gsd-explore` and `/gsd-spike` to weft's bead+jj substrate.

**Architecture:** Purely prompt-layer; pure composition of `bd` + `jj` + the `Explore` subagent, no engine change. Both are standalone doors upstream of the existing funnel (`explore` WHAT-shaping before a project exists; `spike` feasibility anywhere an approach is uncertain). Outputs are bead-native — no `.planning/` files, no kept spike code.

**Tech Stack:** Claude Code plugin skills (markdown + YAML frontmatter); `bd` (beads — `bd create`, `bd defer`, `bd close`, `bd remember`, `bd note`, `bd list`); `jj` (throwaway experiment changes); the `Explore` subagent (Task tool) for `explore`'s optional research pass.

**Spec:** `docs/superpowers/specs/2026-06-12-explore-spike-skills-design.md`

**Design bead:** `weft-ccy.4`

---

## Grounding notes (verified against the repo)

- **Files touched:** `plugin/skills/explore/SKILL.md` and `plugin/skills/spike/SKILL.md` are the two new-creates. Parent `plugin/skills/` exists; siblings `discuss/`, `execute/`, `feature/`, `new-project/`, `onboard/`, `phase-driver/`, `plan-phase/`, `verify-work/`. **Skill-only** — no `weft/commands/` + `weft/workflows/` pair (ADR `weft-88z`, matching the newest skills).
- **Skills auto-discovered** — `plugin/.claude-plugin/plugin.json` does not enumerate skills; creating the `SKILL.md` files registers them. (README skill index maintenance is decoupled — the newest skills are absent from it — so this plan does not touch `plugin/README.md`.)
- **bd flags (verified live):** `bd create` has **no `--status` flag** — deferral is two-step: `bd create --type task --labels seed --description "…" --json` (flat top-level `.id`) then **`bd defer <id>`**. `bd defer`/`bd undefer` exist; `bd ready` returns only `status=open`, so deferred seeds are correctly excluded from the ready set. `bd create` confirmed flags: `--type {task,chore,…}`, `--labels` (comma-sep), `--parent`, `--description`/`--stdin`, `--json`. `bd remember "…" --key <key>` confirmed (memories surfaced by the `bd prime` hook at session start). `bd close <id> --reason`, `bd note <id> --stdin` confirmed.
- **jj throwaway lifecycle:** the working copy is always a commit and auto-snapshotted, so `jj new main` preserves the user's in-flight `@` as a commit; `jj abandon @` discards only the experiment; `jj edit <saved-change-id>` returns the user to where they started. All jj invocations pass `--no-pager` per the `jj-agent-safety` profile.
- **`Explore` subagent** is the existing Task-tool agent `feature`/`onboard` use for recon — distinct namespace from this `explore` *skill*; `explore` MAY dispatch it for the research pass.
- **No automated unit tests apply** — markdown prompts. Per-task gate is the CI discipline run locally: `claude plugin validate ./plugin --strict`, `claude plugin validate . --strict`, and `grep -RnE 'weft/(agents|references|workflows)/' plugin/` returning no matches.

---

## File Structure

| File | Responsibility | Action |
|---|---|---|
| `plugin/skills/explore/SKILL.md` | Socratic WHAT-shaping ideation → deferred `seed` beads + `bd remember`; hands off to new-project/feature/spike. | Create |
| `plugin/skills/spike/SKILL.md` | Given/When/Then feasibility experiments in throwaway jj changes → verdict `spike` bead + `bd remember`; optional epic context. | Create |

---

## Task 1: explore skill

**Files:**

- Create: `plugin/skills/explore/SKILL.md`

- [ ] **Step 1: Create the skill file**

Create `plugin/skills/explore/SKILL.md` with exactly this content:

````markdown
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
        --description "<idea>. Trigger: <when this should be picked up>." \
        --json | jq -r .id)
  bd defer "$ID"
  ```

  `bd create` has no `--status` flag, so defer is the second step; `bd defer`
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
````

- [ ] **Step 2: Validate the plugin tree (registers the new skill)**

Run:

```
claude plugin validate ./plugin --strict
claude plugin validate . --strict
grep -RnE 'weft/(agents|references|workflows)/' plugin/ ; echo "grep-exit=$?"
```

Expected: both `validate` runs exit 0 and accept the new `explore` skill; grep prints no lines and `grep-exit=1`.

- [ ] **Step 3: Commit**

Commit per `references/vcs-preamble.md`. Message: `feat(weft-ccy.4): explore skill — Socratic ideation to deferred seed beads`.

---

## Task 2: spike skill

**Files:**

- Create: `plugin/skills/spike/SKILL.md`

**Depends on:** none (independent of Task 1; both are standalone doors).

- [ ] **Step 1: Create the skill file**

Create `plugin/skills/spike/SKILL.md` with exactly this content:

````markdown
---
description: Feasibility experiments — decompose a technical question into Given/When/Then hypotheses, run throwaway jj-change experiments, record VALIDATED/INVALIDATED/PARTIAL verdicts as a spike bead + a bd remember finding. Optional epic context.
argument-hint: "[question] [epic-id]"
---

<!--
  ~ SPDX-License-Identifier: Apache-2.0
  ~ Copyright 2026 Weft Contributors
-->

<!-- adapted from /gsd-spike (GSD Core, MIT), rewritten to throwaway-jj + bead-backed state -->

# spike workflow

Focused feasibility experiments before committing to an approach. weft's analog
of `/gsd-spike`. Each experiment is a **Given/When/Then** hypothesis run in a
**throwaway jj change** that is abandoned afterward — the code is disposable, the
**verdict** is what survives, as a `spike` bead + a `bd remember` finding.

Run loose (pre-project: "is X feasible before I commit?") or with an optional
`[epic-id]` (mid-flow: "can this phase's approach work?", feeding `plan-phase`).

> jj note: all jj commands use `--no-pager`; experiment code is never kept. The
> working copy is always a commit and auto-snapshotted, so your in-flight work is
> never lost across the spike.

---

## Phase 1 — Intake + decompose

Treat the optional `[question]` as the seed; one light round of framing
questions if the technical uncertainty is unclear. Decompose it into **2–5
independent experiments**, each phrased as a **Given/When/Then** hypothesis:

```
Given <starting condition>
When  <the thing we try>
Then  <the observable result that would confirm/deny feasibility>
```

State the experiments to the user before running them. (No `--quick`/`--text`
mode flags in v1 — default adaptive mode only, consistent with `discuss`.)

---

## Phase 2 — Run each experiment in a throwaway jj change

Before touching jj, **save the current working-copy change id** so you can
restore the user afterward:

```
SAVED=$(jj --no-pager log -r @ --no-graph -T 'change_id.short(12)')
```

Then, for each experiment:

```
jj --no-pager new main          # loose; epic-scoped: jj --no-pager new <epic-tip>
# ... write the minimal experiment code, run it ...
```

For the epic-scoped case, resolve `<epic-tip>` from the epic's most recent landed
pick — its `jj-change:<id>` label — or a bookmark at the epic tip. Observe the
result against the Given/When/Then and record the verdict — **VALIDATED**,
**INVALIDATED**, or **PARTIAL** — with the concrete evidence (what ran, what
happened). Then discard the experiment:

```
jj --no-pager abandon @
```

After all experiments, **return the user to where they started**:

```
jj --no-pager edit "$SAVED"
```

Never keep experiment code; the verdict + evidence captured in Phase 3 is the
durable artifact.

---

## Phase 3 — Persist the verdicts

Record one **`spike` bead** capturing the question, every Given/When/Then, each
verdict, and the evidence:

```
ID=$(printf '%s' "Spike: <question>

<for each experiment: the Given/When/Then + VERDICT + evidence>" \
  | bd create --type chore --labels spike --stdin --json | jq -r .id)
```

If invoked with `[epic-id]`, parent the spike bead under it
(`bd create … --parent <epic-id> …`) and fold the validated decision into the
epic's `design` field so `discuss`/`plan-phase` see it **this session**:

```
bd update <epic-id> --design "<settled technical choice + why, from the spike>"
```

Close the spike bead (the investigation is done):

```
bd close "$ID" --reason "spike concluded: <one-line verdict summary>"
```

Record the validated finding as a durable memory for **future** sessions
(`bd prime` injects memories at session start, so this surfaces next session —
the epic `design` field above is the same-session path):

```
bd remember "spike: <library/approach> <VALIDATED|INVALIDATED|PARTIAL> for <use-case> — <evidence>" --key spike-<slug>
```

---

## Phase 4 — Conclude + hand off

Report the verdicts. A **VALIDATED** spike unblocks planning — run `plan-phase`
(epic-scoped) or `feature`/`new-project`. An **INVALIDATED** one redirects the
approach — back to `explore`/`discuss` with the finding in hand.

---

## What this workflow does NOT do

- It does not keep experiment code — every experiment change is abandoned; only
  the verdict survives.
- It does not write `.planning/spikes/` files or a wrap-up skill — the durable
  outputs are the `spike` bead + `bd remember`.
- It does not plan or implement the validated approach — that is
  `plan-phase`/`feature`.
- It does not leave the user's working copy moved — it restores the saved `@`.
````

- [ ] **Step 2: Validate the plugin tree**

Run:

```
claude plugin validate ./plugin --strict
claude plugin validate . --strict
grep -RnE 'weft/(agents|references|workflows)/' plugin/ ; echo "grep-exit=$?"
```

Expected: both `validate` runs exit 0 and accept the new `spike` skill; grep prints no lines and `grep-exit=1`.

- [ ] **Step 3: Commit**

Commit per `references/vcs-preamble.md`. Message: `feat(weft-ccy.4): spike skill — Given/When/Then experiments in throwaway jj changes`.

---

## Task 3: Validation (dogfood gate)

**Files:** none modified — this is the acceptance gate.

**Depends on:** Tasks 1–2.

Drive both skills interactively against throwaway inputs and confirm the bead/jj outputs. Use a `spike-check` scratch label so the test beads are easy to find and clean up.

- [ ] **Step 1: explore — seeds + memory land**

Run `explore` on a toy idea (e.g. "a weft doctor command that checks the toolchain"). Steer it to produce at least one seed and one context memory. Verify:

```
bd list --label seed --json | jq -r '.[] | select(.status=="deferred") | "\(.id) \(.title)"' | head
bd memories explore 2>&1 | rg -i "explore-" | head
```

Expected: ≥1 deferred `seed` bead exists; ≥1 `explore-*` memory is present. Confirm the seed is **not** in `bd ready`:

```
bd ready --json | jq -r '.[] | select(.labels // [] | index("seed"))' | head
```

Expected: empty (deferred seeds are excluded from the ready set).

- [ ] **Step 2: spike — throwaway change abandoned, verdict survives**

Note the current change first, then run `spike` on a trivial Given/When/Then (e.g. "Given Go 1.26, When I call `errors.Join`, Then it compiles" — something quick):

```
SAVED=$(jj --no-pager log -r @ --no-graph -T 'change_id.short(12)')
```

Run `spike`. After it concludes, verify the experiment change was abandoned (working copy is back at `$SAVED`) and the durable outputs remain:

```
jj --no-pager log -r @ --no-graph -T 'change_id.short(12)'        # expect: $SAVED
bd list --label spike --json | jq -r '.[] | "\(.id) \(.status) \(.title)"' | head
bd memories spike 2>&1 | rg -i "spike-" | head
```

Expected: working copy is back on `$SAVED`; a closed `spike` bead exists; a `spike-*` memory is present. The `@ == $SAVED` equality above is the restoration proof — `jj abandon` leaves no trace, so a clean abandon shows up as the working copy being exactly back where it started. As a final sanity check, confirm the working copy holds the user's pre-spike state and none of the experiment's files:

```
jj --no-pager st
```

Expected: the user's original working-copy state (whatever `$SAVED` contained) — no leftover experiment files. If `@ != $SAVED` or experiment files remain, the skill failed to `jj abandon @` / `jj edit "$SAVED"`.

- [ ] **Step 3: Re-run the plugin validation gate**

```
claude plugin validate ./plugin --strict
claude plugin validate . --strict
grep -RnE 'weft/(agents|references|workflows)/' plugin/ ; echo "grep-exit=$?"
```

Expected: both `validate` runs exit 0; grep prints no lines, `grep-exit=1`.

- [ ] **Step 4: Clean up scratch beads + record the result**

Delete the throwaway test beads created during the dogfood (use the ids surfaced above):

```
# bd delete <seed-id> <spike-id>   # remove the dogfood test beads
bd note weft-ccy.4 "dogfood <date>: explore produced a deferred seed bead (excluded from bd ready) + an explore-* memory; spike ran G/W/T in a throwaway jj change, abandoned it (working copy restored to saved @), left a closed spike bead + spike-* memory; plugin validate strict x2 + grep-discipline clean."
```
<!-- adr-capture: sha256=65ad30c2a568fba2; session=cli; ts=2026-06-12T12:40:06Z; adrs=weft-fwn -->
