<!--
  ~ SPDX-License-Identifier: Apache-2.0
  ~ Copyright 2026 Weft Contributors
-->

# `explore` + `spike` skills — upstream ideation + feasibility (ccy.4) — design

**Date:** 2026-06-12
**Bead:** weft-ccy.4 (parent epic: weft-ccy — Restore GSD Layer-A interactive/phased loop)
**Status:** design
**Port source:** `/gsd-explore`, `/gsd-spike` (GSD Core, MIT), rewritten to bead-backed state

## Context

`weft-ccy` restores GSD's pre-planning + interactive surface over weft's stable
verb surface + beads. The core loop landed (discuss, verify-work, engine
enablers, phased planner, feature, onboard, phase-driver). `weft-ccy.4` is the
last functional child: GSD's **pre-planning ideation surface** — the doors that
come *before* you commit to building. The 06-09 epic spec §7 scoped three
skills (explore / spike / sketch) "with bead-backed outputs (seeds/notes; no
`.planning/` files)."

**Scope decision (this round): `explore` + `spike` only.** `sketch` (throwaway
HTML mockups for visual/UI direction) is the one of the three whose output is
not naturally bead-shaped — it produces browser-rendered artifacts, and its GSD
downstream is `/gsd-ui-phase`, which weft has not ported. Rather than rush a
poor-fit adaptation, `sketch` (plus the ui-phase consumer question) splits to a
**follow-up bead** with its own design. This is sequencing, not dropping: the
06-09 spec's ui-phase deferral was explicitly "in this round," and that round is
now complete.

## Decisions made in this session

| Decision | Choice |
|---|---|
| Scope | `explore` + `spike` now; `sketch` (+ ui-phase question) → follow-up bead with its own design |
| explore outputs | Forward ideas → **seed beads** (`bd create --type task --labels seed --title …` then `bd defer <id>` — `bd create` requires `--title` and has no `--status` flag; trigger in description); durable context/decisions → **`bd remember`**. No holder epic, no `.planning/` files. |
| spike code lifecycle | Each experiment in a **throwaway `jj new main` change**, run, record verdict, then **`jj abandon`**. The verdict is the durable artifact, not the code. |
| spike outputs | A **spike bead** (`--label spike`, G/W/T + verdict + evidence, closed when done); validated finding → **`bd remember`** (consumed by discuss/plan-phase, weft's analog of GSD wrap-up→skill). |
| Coupling | **Standalone doors, soft handoffs.** No skill hard-invokes another. explore *suggests* spike; spike's `bd remember` is *automatically* read by discuss/plan-phase. |
| Context | `spike` takes optional `[epic-id]` (loose or epic-scoped); `explore` is always loose (pre-project). |
| Naming | Keep `explore` (GSD parity). It is a *skill* (`/weft:explore`), a distinct namespace from the existing `Explore` *subagent* (dispatched via the Task tool); the spec calls out the distinction. |
| Engine | None. Pure prompt-layer composition over existing verbs + `bd`, like every other ccy skill. |

## 1. `explore` skill

The front of the funnel: open-ended Socratic ideation before there is a project
or epic. weft's analog of `/gsd-explore`, rewritten to bead-backed state.

**Flow:**

1. Treat the optional `[topic]` as the seed. Run a **Socratic conversation** —
   2–5 single questions, one at a time, probing constraints, users, scope,
   tradeoffs, dependencies, and risks. Same per-question rhythm as `discuss`,
   but **WHAT-shaping** (is this worth building, and what is it?) rather than
   `discuss`'s **HOW-shaping** (how do we build this known thing?).
2. **Optional research (≤1 pass).** When a factual question or technology
   comparison arises mid-conversation, offer one lightweight `Explore`-subagent
   research pass — deliberately *not* `new-project`'s 4-agent fan-out. Hold the
   digest in context.
3. **Route outputs to beads.** After the conversation, propose and persist:
   - **Forward-looking actionable ideas → seed beads:**
     `bd create --type task --labels seed --title "<short idea>" --description "<idea + trigger conditions>"`
     then `bd defer <id>` (`bd create` requires `--title` and has no `--status`
     flag — deferral is the second step). They live in the warp backlog as schedulable work-in-waiting;
     a seed is promoted/planned when its trigger fires.
   - **Durable context / decisions → `bd remember`** (project-global, surfaced
     by the `bd prime` hook every future session; no epic needed).
   - **Open research questions** → either a seed bead (if it implies work) or a
     `bd remember` note (if it is just an open question to revisit).
4. **Hand off.** explore plans nothing itself. When a seed is ripe, point the
   user to `new-project` (greenfield build) or `feature` (incremental work on an
   existing codebase). If feasibility is the blocker, suggest `spike`.

**`explore` is always loose** — there is no epic context, by design. Its seeds
and memories are the bridge from "vague idea" to "a thing `new-project`/`feature`
can plan."

**Naming note.** The `explore` *skill* (this) is distinct from the `Explore`
*subagent* that `feature`/`onboard` dispatch via the Task tool for codebase
recon. Different namespaces (skill vs agent type); no runtime collision. The
skill MAY itself dispatch the `Explore` subagent for its step-2 research pass.

## 2. `spike` skill

Focused feasibility experiments before committing to an approach. weft's analog
of `/gsd-spike`, rewritten to jj-throwaway + bead-backed state.

**Flow:**

1. **Intake.** Treat the optional `[question]` as the seed; one light round of
   questions to frame the technical uncertainty. Decompose it into **2–5
   independent experiments**, each a **Given/When/Then** hypothesis.
   (A `--quick` style "skip intake" is out of scope for v1 — default adaptive
   mode only, consistent with `discuss`'s no-mode-flags decision.)
2. **Run each experiment in a throwaway jj change.** First **save the user's
   current working-copy change id** (`jj log -r @ --no-graph -T
   'change_id.short(12)'`) so it can be restored afterward — `jj new` moves `@`,
   and the spike must not leave the user parked on `main`. Then `jj new main`
   (epic-scoped: `jj new <epic-tip>`, where `<epic-tip>` is resolved from the
   epic's most recent landed pick — its `jj-change:<id>` label — or
   `jj log -r <epic-bookmark>`), write the minimal experiment code, run it,
   observe the result against the Given/When/Then. Record the verdict —
   **VALIDATED / INVALIDATED / PARTIAL** — with the concrete evidence. Then
   `jj abandon @` the change and **`jj edit <saved-change-id>`** to return the
   user to where they started: the code is disposable, the verdict is what
   survives. jj is purpose-built for this — the working copy is always a commit
   (the user's in-flight work is auto-snapshotted and never lost), and
   `jj abandon` cleanly discards the experiment. (All jj invocations in the
   SKILL.md follow the `jj-agent-safety` profile: `--no-pager`, `--git` on
   diffs, change-ids not commit hashes.)
3. **Persist outputs to beads:**
   - A **spike bead**: `bd create --type chore --label spike --description "<the
     technical question + each Given/When/Then + verdict + evidence>"`, closed
     when the spike concludes (`bd close`). If invoked with `[epic-id]`, parent
     the spike bead under that epic (`--parent <epic-id>`) and optionally fold
     the validated decision into the epic's `design` field for the planner.
   - The validated finding → **`bd remember`** (e.g. "spike: library X validated
     for use-case Y — <evidence>"), so a later `discuss`/`plan-phase` consumes
     the settled technical choice without re-spiking. This is weft's analog of
     GSD's `--wrap-up` → project-local skill.
4. **Hand off / conclude.** Report the verdicts. A VALIDATED spike unblocks
   planning (run `plan-phase`/`feature`); an INVALIDATED one redirects the
   approach (back to `explore`/`discuss`).

**`spike` takes optional epic context.** Loose (pre-project: "is X feasible
before I commit?") or epic-scoped (mid-flow: "can this phase's approach actually
work?", feeding `plan-phase`). The only difference is whether the spike bead is
parented and whether the finding lands on an epic `design` field.

## 3. Coupling and relationship to existing skills

**Standalone doors, soft handoffs** — no skill hard-invokes another:

- `explore` **suggests** `spike` when a feasibility question blocks ideation, and
  `new-project`/`feature` when an idea is ready to build. It does not invoke
  them.
- `spike`'s `bd remember` finding flows to `discuss` and `plan-phase` through the
  shared bead substrate, not a call edge. **Latency caveat:** memories are
  injected by the `bd prime` hook at *session start*, so a finding written by
  `spike` surfaces automatically in the **next** session — not retroactively in
  the current one. For same-session use (spike → immediately plan that epic),
  the epic-scoped `spike` also folds the validated decision into the epic's
  `design` field / notes, which `discuss`/`plan-phase` read directly. The
  `bd remember` entry is the durable, cross-session form; the epic field is the
  immediate, same-session form.
- Both sit *upstream* of the existing funnel: `explore` (WHAT) → `new-project` /
  `feature` (plan) → `discuss` (HOW) → `execute` → `verify-work` → `finish`;
  `spike` (feasibility) attaches wherever an approach is uncertain.

This keeps each skill independently understandable and testable, and avoids
re-introducing GSD's tighter command-chaining.

## 4. GSD contrast

| | GSD | weft |
|---|---|---|
| explore outputs | `.planning/{notes,todos,seeds,research}/…` files | seed beads (deferred) + `bd remember` |
| spike code | `.planning/spikes/NNN/` (kept) | throwaway `jj` change, abandoned |
| spike verdict | `MANIFEST.md` + `--wrap-up` skill file | spike bead + `bd remember` |
| chaining | explore invokes spike/sketch; spike feeds plan-phase | standalone; soft handoffs via the bead substrate |
| modes | `--quick`, `--text`, etc. | default adaptive mode only (v1, per `discuss`) |

## 5. Out of scope

- **`sketch` + the `ui-phase` question** — split to a follow-up bead with its own
  design (it is the one skill whose output is not bead-shaped and whose GSD
  consumer weft has not ported).
- **GSD's `--quick` / `--text` / batch mode flags** — v1 ships default adaptive
  mode only, consistent with `discuss`/`verify-work`.
- **Engine changes** — none; the engine stays non-interactive. Both skills are
  prompt-layer composition over existing verbs + `bd`.
- **A dedicated `seed` bead type or `weft seed` verb** — seeds are ordinary
  deferred beads with a `seed` label; no new type or engine surface.

## 6. Testing & validation

- **Prompt-layer gates:** `claude plugin validate ./plugin --strict` and
  `. --strict`; the intra-tree path-citation grep discipline (reject
  `weft/(agents|references|workflows)/` paths in `plugin/`).
- **explore dogfood:** run `explore` on a toy idea; assert ≥1 seed bead lands
  (`bd list --label seed --status deferred`) and a `bd remember` memory is
  created (surfaced by `bd prime`).
- **spike dogfood:** run `spike` on a trivial Given/When/Then; assert the
  throwaway change is abandoned (no lingering change in `jj log`), and a spike
  bead (`bd list --label spike`) + a `bd remember` finding remain.

## ADR impact

Likely one capture-worthy decision: **ideation/feasibility outputs are
bead-native** — explore emits deferred `seed`-labelled beads + memories and
spike runs throwaway-jj experiments whose only durable output is a verdict bead +
memory (no `.planning/` files, no kept spike code). This is the substrate
translation of GSD's file-based pre-planning surface, and it is the load-bearing
choice here. `/capture-adrs` after the plan will confirm.
