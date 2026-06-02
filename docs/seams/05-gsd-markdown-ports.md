<!--
  ~ SPDX-License-Identifier: Apache-2.0
  ~ Copyright 2026 Weft Contributors
-->

# Seam 5 — GSD markdown ports

> Status: **exploratory design**, captured from a brainstorming session.
> Sub-spec of [`docs/design.md`](../design.md) §9 open seam 5 (capstone).
> Tracked as bead `weft-hjx.5` (child of `weft-hjx`). Not yet
> `design-reviewer`-approved. No implementation exists.

## 1. Scope

Which GSD command/agent markdown ports over as Weft **reference drafts**, how
each adapts to the beads+jj substrate, where the prompts live, and how they
reach each host runtime. This is the **port map + adaptation principles** — it
specifies the prompts the other seams defer to (seam 2's `/weft-new-project`,
seam 4's resolver brief, the orchestrator/executor command markdown that calls
the seam-1 verbs). The actual prompt *files* are authored during
implementation; this seam is the plan for them.

Out of scope: the engine verbs the prompts call (seams 1–4); the `weft install`
transform internals (§7, deferred).

## 2. Porting strategy: hybrid per-artifact

GSD is MIT; Weft is Apache-2.0; design.md §1 says "clean-room," the Attribution
note says "adapted." The reconciliation:

> **"Clean-room" scopes to the Go engine** (new code, not a fork of GSD's
> Node/`cjs`). **Prompts are ported per-artifact, section by section:**

| Section character | Disposition |
|---|---|
| Substrate-agnostic **methodology** (vertical-slice planning, file-ownership reasoning, executor TDD discipline, reviewer rigor, verify gates) | **ADAPT** — start from GSD's MIT text, modify for beads+jj+weft verbs, credit in `NOTICE`. |
| **Layer-B/C mechanics** (writing `ROADMAP.md`/`STATE.md`/`PLAN.md`, `git worktree` choreography, `gsd-tools.cjs` calls) | **REWRITE** — these layers are *deleted* in Weft (design.md §3/§4), so there is nothing to adapt to; write fresh for beads+jj. |
| Specialized surfaces with no v1 Weft analog (phase-type commands, doc-synthesis, etc.) | **DROP** for v1; revisit later. |

So most ports are *partial*: keep the reasoning, replace the plumbing.

## 3. Port map

GSD ships 67 commands + 33 agents; the Weft v1 core is a handful. Each row
classifies the ADAPT / REWRITE / DROP split.

| GSD artifact | Weft target | What ADAPTs | What REWRITES / DROPs |
|---|---|---|---|
| `/gsd-new-project` + `gsd-roadmapper` | `/weft-new-project` cmd | adaptive questioning, parallel research, requirement extraction | roadmap/`STATE.md`/`PROJECT.md` writing → **DROP**; output is `warp-plan.json` → `weft plan emit` (seam 2) |
| `/gsd-plan-phase` + `gsd-planner` | `weft-planner` agent | vertical-slice bias, file-ownership→dep reasoning, wave thinking (seam 2 §4) | `PLAN.md` / wave-annotation writing → REWRITE to emit `warp-plan.json` |
| `/gsd-execute-phase` + `execute-phase` workflow | `/weft-execute` cmd (thin orchestrator) | wave orchestration, verify-gate methodology, fresh-context dispatch | `worktree-safety.cjs` choreography → REWRITE to `weft shed form/isolate/integrate/cleanup` (seams 1/3) |
| `gsd-executor` | `weft-executor` agent | TDD discipline, deviation handling, atomic-unit-of-work | commit/worktree mechanics → REWRITE to `weft pick seal` + jj profile (seams 1/3) |
| `gsd-code-reviewer` | `weft-reviewer` agent / `pick verify` gate | review rigor (bugs/security/quality) | `REVIEW.md` artifact → REWRITE to verify-verdict **data** (seam 1 `pick verify`) |
| `gsd-code-fixer` | `weft-resolver` agent | fix-as-guidance, verify-each, atomic intent | `REVIEW-FIX.md`/commit mechanics → REWRITE to marker-editing in `conflict open`/`finalize` (seam 4) |
| `/gsd-verify-work` | `/weft-verify` cmd | UAT flow, auto-diagnosis | state-file writes → DROP (state is beads) |
| `/gsd-ship` | `/weft-finish` cmd | PR-from-work, review-before-merge methodology | branch/worktree mechanics → REWRITE to `weft finish open/reconcile` (seam 1) |
| `gsd-tools.cjs` calls (inside every orchestrator) | weft / bd / jj verbs | — | REWRITE every call → a `weft`/`bd`/`jj` verb (the engine is the new tool layer) |
| `references/*.md` | `weft/references/*.md` | substrate-agnostic refs (TDD, verify discipline, fresh-context principle) | ADD jj agent-safety profile + bead↔change-id spine refs |
| ~60 other commands / ~28 other agents (phase-types `ui`/`ai`/`spec`, doc-synth, namespaces) | — | — | **DROP** for v1 |
| `.planning/` artifacts (ROADMAP/STATE/PROJECT/SUMMARY) | — | — | **DROP** — beads is the brain (design.md §3) |

## 4. The thin-orchestrator pattern ports

GSD's "Thin Orchestrator" (workflow loads context → resolves model → spawns
fresh agent) ports directly — only the *tool layer* changes:

| GSD orchestrator step | Weft orchestrator step |
|---|---|
| `gsd-tools.cjs init <workflow>` (load context) | `bd ready` / `weft resume` / `weft shed form` |
| `gsd-tools.cjs resolve-model <agent>` | the bead's `model:*` label (Rule 5 convention) |
| spawn `Agent(prompt, ctx, model, tools)` | host-runtime dispatch (Claude Code `Agent`, etc.) — unchanged pattern |
| `gsd-tools.cjs state ...` (write `.planning/`) | `bd` mutations + `weft` verbs (no state files) |

Weft command markdown is therefore *thinner* than GSD's: the dangerous
choreography that GSD spread across workflow markdown + `worktree-safety.cjs`
now lives behind coarse `weft` verbs (seams 1/3/4), so the orchestrator prompt
is mostly "call this verb, dispatch that agent, branch on the JSON."

## 5. Layout, naming, runtime portability

- **Source tree** (runtime-agnostic, authored in Claude-Code-native format — the
  lingua franca, as GSD does):

  ```
  weft/
    commands/    /weft-new-project, /weft-execute, /weft-verify, /weft-finish
    agents/      weft-planner, weft-executor, weft-reviewer, weft-resolver
    workflows/   the orchestrator bodies the commands invoke
    references/  jj agent-safety profile, bead↔change-id spine, TDD/verify discipline
  ```

- **Naming:** `/weft-*` commands, `weft-*` agents (mirrors `/gsd-*`).
- **Runtime portability:** a `weft install --runtime <claude|codex|…>` transform
  (the engine's analog of GSD's npx installer) rewrites the agnostic source into
  each host's expected format/location (command-syntax form, frontmatter
  stripping) and places it (`.claude/commands/`, etc.). Claude Code is the v1
  reference runtime; the transform's exact rules are a §8 sub-seam.

## 6. Licensing & attribution

- **Adapted** prompt sections (methodology) carry GSD provenance in a top-level
  `NOTICE` file crediting **GSD Core (MIT, © its contributors)**, consistent
  with design.md's Attribution. Weft prompt files keep Apache-2.0 SPDX headers.
- **Rewritten** sections (beads+jj mechanics) are original Apache-2.0, no GSD
  provenance.
- MIT permits this reuse; the `NOTICE` + SPDX headers satisfy both licenses.
  Per-file, a short provenance comment notes "adapted from `gsd-<x>.md`" where
  applicable.

## 7. Reference-draft discipline

Because this seam produces *reference drafts*, not final prompts, each ported
prompt authored in implementation MUST:

- be classified section-by-section (ADAPT / REWRITE) per §2 before drafting;
- call **only** the stable verb surfaces of seams 1–4 (never reach around the
  engine into raw multi-step jj choreography the verbs encapsulate);
- carry its NOTICE/provenance per §6;
- be validated against the substrate it targets (a ported prompt that still
  references `.planning/` or `worktree` is a porting bug).

## 8. Open sub-seams (next design steps)

- The `weft install` transform rules per runtime (frontmatter, command-syntax,
  placement) — and whether install is a `weft` verb or a separate tool.
- The per-prompt section classification worksheets (one per §3 row) produced
  when each prompt is actually drafted.
- Model-routing convention (`model:*` labels) ↔ GSD's `resolve-model`.
- Whether `weft-reviewer` is a distinct agent or folded into `pick verify`'s
  gate (seam 1 left verify thin).

## 9. Cross-spec note

This seam consumes — does not change — the seam 1–4 verb surfaces; the ported
prompts are their callers. It is the only seam whose deliverable is *prompts*
rather than *engine mechanics*, and it closes design.md §9's open-seam list.

## Attribution

Command/agent prompt structure and methodology adapted from **GSD Core**,
MIT-licensed, © its contributors (see `NOTICE` once prompts are authored). Weft
is independently licensed Apache-2.0.
