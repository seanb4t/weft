<!--
  ~ SPDX-License-Identifier: Apache-2.0
  ~ Copyright 2026 Weft Contributors
-->

# Seam 1 — Engine command surface

> Status: **shipped** (design-reviewer READY, round 1). Sub-spec of
> [`docs/design.md`](../design.md) §9 open seam 1. Tracked as bead `weft-hjx.1`
> (child of `weft-hjx`). Round-1 minors fixed inline (`pick land` bookmark,
> `bd reopen` vs `--status open`, parent §5 staleness). Implemented and
> released.

## 1. Scope

The stable verbs the Go engine (`weft`) exposes for the command-markdown / host
runtime to call. The engine is **deterministic plumbing** wrapping `bd` + `jj`
and choreographing workspaces (design.md §7). It does **not** dispatch agents
and does **not** write prose — the host runtime owns both.

This seam resolves two non-blocking findings deferred from the design.md
`design-reviewer` round (round 2 READY):

- **(a)** the bead `open → in_progress` transition point;
- **(b)** the wave-member integration tiebreaker.

Out of scope (other seams): how planning emits beads (seam 2); workspace
lifecycle internals — stale handling, crash cleanup, `<repo>_worktrees/` layout
(seam 3); conflict-resolution UX (seam 4); GSD markdown ports (seam 5).

## 2. Granularity: hybrid

The engine's reason for existing is to **delete** GSD's choreography
(`worktree-safety.cjs`) by absorbing it. How much it absorbs follows one rule:

> **Coarse** (atomic) verbs for dangerous multi-step choreography that must not
> be composed by a prompt; **thin** primitives as escape hatches for steps the
> agent legitimately composes or varies.

The discriminating test for each verb: *is this a dangerous multi-step
sequence that must be atomic, or a query / single-op the agent composes
freely?*

## 3. Output contract

Mirrors GSD's `gsd-tools.cjs`, validated against how the Claude Code harness
consumes a CLI: the agent runs `weft` via Bash and gets stdout + stderr + exit
code back as text, serving two modes simultaneously — **display/judgment**
(reads prose) and **deterministic branching** (parses a field).

| Aspect | Contract |
|---|---|
| Default stdout | Human-readable text (readable transcripts). |
| `--json` | Uniform JSON envelope (deterministic branching). |
| `--pick <path>` | Extract one field via dot/bracket path; prints the bare value. No `jq` dependency in prompts. |
| Errors | Human text on stderr by default; `--json-errors` for a structured error object. |

**Exit codes reflect whether the *engine* did its job — never the verdict of
the work:**

| Code | Meaning |
|---|---|
| `0` | Engine succeeded (the *work's* verdict — pass/fail, conflict/clean — is **data in the body**). |
| `1` | Invocation error (bad args, missing workspace, unknown bead). |
| `2` | Hard failure (underlying `jj`/`bd`/`gh` command failed). |

**Handoffs are body fields on exit 0, not exit codes.** A `shed integrate` that
produces a first-class jj conflict *succeeded* (design.md §4: conflicts are
first-class objects, not blockers) — exit `0` with a `conflicts[]` field. A
`pick verify` whose gate fails *ran fine* — exit `0` with `{pass: false}`. The
prompt branches on `.conflicts` / `.pass`, keeping "the tool broke" and "the
work needs attention" from collapsing into one number.

### 3.1 Envelope shape

```json
{
  "ok": true,
  "verb": "shed.integrate",
  "data": {
    "groups": [
      [{"bead": "weft-a1", "change": "q2"}, {"bead": "weft-a2", "change": "x9"}],
      [{"bead": "weft-a3", "change": "k4"}]
    ],
    "conflicts": [
      {"bead": "weft-a3", "change": "k4"}
    ]
  },
  "next": "resolve-conflicts"
}
```

`ok`, `verb`, `data` are always present. `data.conflicts` and the top-level
`next` are verb-dependent — `conflicts` is nested under `data`, and each entry
is `{bead, change}` only (`paths`/`n` enrichment is deferred to seam 4 §8 and is
not emitted). `next` is an advisory hint (a string the prompt MAY branch on),
never authoritative — the authoritative state is always re-derivable from `bd`
+ `jj`.

## 4. The verb set

Noun groups: `shed` (wave-level), `pick` (bead-level), `ws` (workspace escape
hatches), `finish` (epic-level), plus top-level `resume`.

### 4.1 `weft shed …` — the orchestrator loop

| Verb | Kind | Wraps | Notes |
|---|---|---|---|
| `shed form --epic E [--max N]` | thin | `bd ready` ∩ epic, capped by the parallelism dial `--max` | Returns the wave (member bead-ids) as JSON. The scheduler. `--max` default deferred to seam 3 config. |
| `shed isolate <wave>` | coarse | once: `jj git fetch`; per member: `bd update --status in_progress` **then** `jj workspace add … -r trunk()` | **Resolves (a):** the `open → in_progress` transition happens at isolation, not at executor return. Status-first ordering is the [seam 3](03-workspace-lifecycle.md) lifecycle invariant (a crash mid-isolate leaves no reapable workspace). |
| `shed integrate <wave>` | coarse | partition members by file overlap, rebase each group as its own `trunk()`-rooted sub-stack via `jj rebase -s <change> -o <prev-tip>` (no `--skip-emptied`); **tiebreak change-id lexicographic** within a group | **Resolves (b):** wave members are mutually independent, so the dep graph imposes no intra-wave order. **Seam 11** refines this: two picks that touch no common file can never conflict, so integrate builds a **forest** (one sub-stack per file-overlap group, cursor reset to `trunk()` per group) and emits `groups` (a list of `{bead,change}` sub-stacks) + any `conflicts[]`, confining each conflict to its group. `--skip-emptied` is intentionally omitted: it abandons an emptied member, making `prev=<ch>` a dead reference for the next `-o <ch>`; without it every member survives and the cursor stays valid (see ADR `weft-hjx.7`). |
| `shed cleanup <wave>` | coarse | per member: `jj workspace forget` + `rm -rf` | Idempotent teardown. |
| `shed abandon <wave>` | coarse | `bd update --status open` for members (they are `in_progress`) + `shed cleanup` + `jj abandon` the in-flight change-ids | Bails an in-flight wave. Members are `in_progress` (not closed), so the transition is `--status open`, **not** `bd reopen` (which is for closed beads). `jj abandon` cleans working state; changes stay recoverable via `jj op log` / `jj evolog`. |
| `shed status <wave>` | thin | read member states + change-ids | Inspection. |

### 4.2 `weft pick …` — bead-level

| Verb | Kind | Wraps | Notes |
|---|---|---|---|
| `pick seal <bead>` | thin | `jj commit -m "<type>(<bead-id>): <title>"` → change-id, write `jj-change:<id>` label | **Executor-side.** Guards two load-bearing invariants: the conventional-commit message (parsed for PR-body/audit) and the change-id label (the spine, §5.1). |
| `pick verify <bead>` | thin | run the bead's gate | Exit `0` + `{pass: bool, …}`. Verdict is data. |
| `pick land <bead>` | thin | assert the pick's change ∉ `conflicts()` → `bd close --suggest-next` | Happy path. The change is already in the integrated stack (`shed integrate`); landing asserts the change is **conflict-free** (never land a conflicted change — [seam 4](04-conflict-resolution.md) §6) then closes the bead. There is **no per-pick bookmark** (bookmarks are epic-level, §4.4 / design.md §6). |
| `pick redo <bead>` | coarse | `jj abandon $(jj-change)` (skipped if no `jj-change` label yet) + reopen the bead (status → `open`) | The §4.1 recovery primitive, atomic. `jj abandon` is a no-op when the crash preceded `pick seal` (no change to abandon). Reopen depends on prior state: `bd update --status open` if the bead is `in_progress` (design.md §5 verifies before close); `bd reopen` only if redoing an already-landed (closed) pick. |

### 4.3 `weft ws …` — thin workspace escape hatches

`ws add <bead>` · `ws forget <bead>` · `ws list`. Single-bead isolation outside
a formed wave. Internals (paths, stale handling) are seam 3.

### 4.4 `weft finish …` — epic-level (design.md §6)

*Naming:* "finishing" is the textile term for post-loom processing of cloth —
the epic comes off the loom and is finished out into the world.

| Verb | Kind | Wraps | Notes |
|---|---|---|---|
| `finish open <epic>` | coarse | `jj bookmark set` + `jj git push -b` + assemble PR body from the epic's closed beads + `gh pr create` | PR body generated from closed beads (§5.1 audit). |
| `finish reconcile <epic>` | coarse | `jj git fetch` + `jj rebase -b @ -o main --skip-emptied` + `jj bookmark delete` | Post-squash-merge cleanup. `-b @` explicit (never `-r @`, which truncates chains). |

### 4.5 `weft plan …` — planning → warp emission (seam 2 / seam 9)

| Verb | Kind | Wraps | Notes |
|---|---|---|---|
| `plan check <file>` | thin | `internal/plan.Validate` | Structural + relational validation. Always exits 0; validity is data: `{ok, verb:"plan.check", data:{valid:bool, issues:[…]}}`. |
| `plan emit <file>` | coarse | `bd create --graph` (first emit) / `bd import` (re-plan) | See below. |

**`plan emit` flag contract** ([seam 9](09-emit-field-drop-guard.md)):

| Flag | Effect |
|---|---|
| *(none)* | First emit: bd-backed preflight (`bd create --graph --dry-run --json`) runs before the real create; hard-fails (exit 2) if bd would silently drop any graph field or if node/edge counts mismatch. |
| `--dry-run` | bd-backed dry run: runs the preflight and folds its warnings + counts + `schema_version` into the envelope — no mutation follows. Exit code follows the strictness matrix below. |
| `--allow-drop` | Downgrades a *drop warning* to a surfaced entry in `data.warnings` and proceeds. Does **not** bypass a count mismatch (count mismatch is always hard). Forward-compat escape hatch; never the default. **First emit only** — rejected as an invocation error (exit 1) when combined with `--epic`; the bd import path has no field-drop preflight, so the flag would be a silent no-op there. |
| `--epic <id>` | Re-plan against an existing epic (`bd import` upsert). After import succeeds, weft re-reads the epic's children via `bd list` and verifies that every authored field (title, priority, labels, description presence) round-tripped. Any discrepancy is a hard exit 2 (seam 9 §7). |

**Exit-2 contract (seam 9 §4 and §7):**

`weft plan emit` may exit 2 in three distinct cases — all are data-integrity failures, not invocation errors:

1. **Field drop (first emit):** the bd preflight stderr contains `unknown field(s)` — a field weft sent would be silently lost. Surfaced verbatim. Downgrade with `--allow-drop`.
2. **Count mismatch (first emit):** `node_count` or `edge_count` in the preflight envelope does not match what weft built (`1 + len(picks)` nodes, `len(derivation.Edges)` edges). Always hard; `--allow-drop` does not bypass it.
3. **Read-back mismatch (re-plan):** after `bd import` succeeds, a post-import `bd list` read-back reveals that one or more authored fields (title, priority, label, description) did not persist. The error lists each discrepancy. Cannot be bypassed.

A `schema_version` difference between weft's `ExpectedGraphSchemaVersion` and the bd preflight's reported value is a **soft warning** only — it appears in `data.warnings` but does not block the emit.

**`data.warnings`** is `[]string`, never null (empty on a clean emit). Any surfaced bd warning or `schema_version` mismatch note appears here.

**`data.verification`** (re-plan path only) is `[]string`, never null. Empty on a clean round-trip; populated with discrepancy strings when verification detects a drop (though in that case exit 2 fires before the success envelope is emitted).

### 4.6 `weft resume --epic E` — read-only projection

The computed `STATE.md` (design.md §3 maps GSD's `STATE.md` →
`bd ready`/`bd blocked` + `jj log`). Projects durable state — it never restores
or mutates:

```
landed:    closed picks + change-ids
in-flight: in_progress beads ↔ workspaces ↔ change-ids
ready:     bd ready (next shed)
blocked:   bd blocked + why
conflicts: unresolved first-class conflicts in the integrated forest
```

**Output key aliases:** the `resume --json` `data` object uses vocabulary from
this spec rather than bare `bd` status names:

| Output key | bd status | Notes |
|---|---|---|
| `landed` | `closed` | Picks that completed the full lifecycle and were `bd close`d. |
| `in_flight` | `in_progress` | Picks currently being worked (workspace exists or is being created). |
| `ready` | — (from `bd ready`) | Unblocked, not yet picked up. |
| `blocked` | `blocked` | Waiting on a dependency. |
| `conflicts` | — (from `jj conflicts()`) | Change-ids with unresolved first-class jj conflicts. |

These aliases are load-bearing for prompt consumers: use `data.landed`,
`data.in_flight`, etc. when branching on `resume --json` output.

**Read-only is a hard invariant.** Even after a crash, recovery is a *separate
explicit* `pick redo` / `shed abandon` — never an implicit resume side-effect.
`resume --json` is also the substrate a fresh session consumes after compaction
(§5.1) and the facts a host-side `dev-flow:handoff-prompt` renders into a
briefing.

## 5. Deliberate non-verbs

These are *deletions* the substrate earns, not omissions:

| Not a verb | Why |
|---|---|
| `weft pause` | Nothing to save — workspaces are durable jj commits and bead status is the truth. A paused wave and a crashed wave are identical to `resume`. Same deletion as GSD's `wip:` handoff commit (§3). |
| `weft handoff` (state-save) | Identical to pause: state is always durable. |
| agent dispatch | Host runtime owns it (§7). The engine choreographs; it never spawns. |
| prose / briefings | Host-side (`dev-flow:handoff-prompt`); the engine emits facts (`resume --json`). |

## 6. Open sub-seams (next design steps)

- Per-verb input/output field schemas (the full `data` shape per verb).
- The `--pick <path>` path grammar (dot/bracket; array indexing).
- Error taxonomy detail (the `--json-errors` object shape; the `1` vs `2` split
  per failure class).
- Where the parallelism dial default lives (flag vs config) — **resolved in
  [seam 3](03-workspace-lifecycle.md):** `shed.max` in `.weft/config.toml`,
  `--max` overrides.

## Attribution

Command-surface shape adapted from **GSD Core** (`gsd-tools.cjs`), MIT-licensed,
© its contributors. Weft is independently licensed Apache-2.0.
