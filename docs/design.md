<!--
  ~ SPDX-License-Identifier: Apache-2.0
  ~ Copyright 2026 Weft Contributors
-->

# Weft — Design

> Status: **design-reviewer READY** (round 2). Round-1 findings addressed
> (executor empty-commit, status mechanics, change-id storage, ship rebase).
> §9 open seams deferred to child specs. No implementation exists yet.

## 1. What Weft is

Weft is a spec-driven AI development orchestration system — the methodology of
[GSD Core](https://github.com/open-gsd/gsd-core) rebuilt on purpose-built
substrates. GSD's value is **context engineering**: keep the main agent context
clean by doing heavy work in fresh subagent contexts, coordinated by durable
state that survives session boundaries.

GSD is three stacked layers, and two exist only because GSD had no real tools
underneath it:

- **Layer A — Methodology** (the borrowable IP): the spec → plan → execute →
  verify → ship loop, fresh-context subagents, verify gates, atomic units of work.
- **Layer B — Homegrown tracker**: `.planning/` files (`ROADMAP.md`, `STATE.md`,
  phase → plan → task hierarchy). Exists only because there was no issue tracker.
- **Layer C — Homegrown VCS choreography**: per-task atomic git commits as an
  audit log, git worktrees for parallel isolation, branch templates.

**Weft keeps Layer A, replaces Layer B with [beads], and replaces Layer C with
[jj].** beads and jj are the purpose-built versions of B and C.

This is a **clean-room reimplementation inspired by GSD**, in a new repository —
not a fork of gsd-core (which is a Node/npm package; Weft is a Go binary).

## 2. The metaphor (and the vocabulary it gives us)

| Loom term | Weft concept |
|-----------|--------------|
| **warp** | the bead dependency graph — the plan, held under tension before weaving |
| **weft** | the woven work — agent changes laid across the warp |
| **loom** | jj — holds the warp, runs parallel sheds, tolerates a dropped pick |
| **pick** | a single woven change (one bead → one jj change) |
| **shed** | a parallel wave — the set of ready beads woven together |

beads holds the *structure* (warp); jj does the *weaving* (loom + weft). This is
exactly the "beads is the brain, jj is the substrate" split.

## 3. Source of truth: beads is the brain

Decision: **all planning, dependency structure, task state, and scheduling live
in beads.** jj is a pure execution substrate. Human-readable prose is minimal or
generated from beads. There is no `ROADMAP.md`, `STATE.md`, or `SUMMARY.md`.

| GSD concept (Layer B/C) | Weft native equivalent |
|---|---|
| `ROADMAP.md` + phase/plan/task hierarchy | beads epics → issues → sub-issues with dependency edges |
| `{phase}-{plan}` task IDs | bead IDs (the natural key) |
| `STATE.md` "where am I / what's next" | `bd ready` / `bd blocked` + `jj log` |
| `/gsd-progress --next` (hand-rolled) | `bd ready` — the dependency graph computes it |
| Atomic per-task git commit = audit log | jj change (auto-snapshotted, stable change-ID); audit = `jj op log` |
| git worktree per parallel executor | `jj workspace` per executor |
| `wip:` handoff commit | nothing — jj's working copy is always already a commit |
| `/gsd-ship` → branch → PR | jj bookmark → `jj git push` → GitHub PR |

## 4. Execution model: jj (Approach C)

Genuine parallel execution requires **filesystem isolation**, which jj's
conflict-tolerance does *not* provide (two agents writing the same file is a disk
race, not a merge conflict). The jj skill is explicit: multiple agents MUST NOT
share one working copy. So each parallel executor gets its own
`jj workspace add`. jj's payoff is on the **integration** side, where GSD is most
contorted.

- **Isolation:** `jj workspace add ../.weft-workspaces/<bead-id> --name <bead-id> -r trunk()`
- **Integration:** all workspaces share one commit graph, so each executor's
  change already exists the moment it commits — no worktree merge-back. The
  orchestrator topologically orders the wave's change-ids by the bead dep graph
  and rebases them into a dep-ordered **linear stack** via
  `jj rebase -s <change> -o <prev-tip> --skip-emptied`. Result: one change per
  bead, bisectable, driven by the bead DAG.
- **Conflicts** land as first-class objects instead of blocking the wave;
  resolved post-hoc at the lowest conflicted ancestor (`jj new <lowest>` → edit
  markers → `jj squash`, which auto-heals descendants).

The net effect is a **deletion**: GSD's entire `worktree-safety.cjs` merge-back
choreography collapses to `jj workspace add` + `jj rebase --skip-emptied` +
`jj workspace forget`. The conflict intelligence moves out of the orchestrator
and into jj.

### 4.1 Recovery (grounded decision — NOT op-restore)

`jj op restore` is **structurally unsafe** for a multi-workspace orchestrator:
it rewinds the *global* operation log, making every other workspace stale, and
`jj workspace update-stale` can silently resurrect pre-rewind content (upstream
bug jj-vcs/jj#9208). The `--what` flag scopes by *kind* (`repo`,
`remote-tracking`), not by workspace. There is no per-workspace scoping.

Recovery is therefore **change-scoped and bead-driven**, which maps cleanly onto
beads-as-brain:

| Recovery need | Correct jj primitive | Why safe |
|---|---|---|
| Verify finds task N broken → redo it | `jj abandon <change-id>` | Change-scoped; descendants rebase onto parent; other workspaces untouched. Bead stores the change-id. |
| "That whole operation was wrong" (no agents in flight) | `jj op revert <op-id>` | Appends an inverse op — does not rewind, does not stale other workspaces. Lock-free safe. |
| Forensics / find a lost intermediate state | `jj --at-op=<id> log`, `jj evolog` | Read-only. |
| Nuclear "rewind everything" | `jj op restore` | **Human-gated only.** Never automated, never while a wave runs. |

The op log records only *successful* operations, so a *failed* agent command
leaves no op-log trace. Recovery cannot be built on the op log even in principle;
it must be driven by the tracker (beads) re-dispatching, with `jj abandon` as the
cleanup. The two design choices reinforce each other.

## 5. The task lifecycle

Two loops: an **orchestrator** (main context, talks to beads) and an
**executor** (fresh context, one workspace).

**Orchestrator:**

1. **Schedule:** `bd ready` returns unblocked issues — this is the entire
   scheduler.
2. **Form a shed (wave):** take the ready set (bounded by a parallelism dial);
   members are mutually independent by construction.
3. **Isolate + dispatch:** per bead, `jj workspace add …`, then spawn a fresh
   executor pointed at that workspace + that bead.
4. **Collect** each executor's reported change-id.
5. **Integrate:** rebase the wave into a dep-ordered linear stack.
6. **Verify** → `bd close` or `jj abandon` + `bd reopen`.
7. **Cleanup:** `jj workspace forget <name>` + `rm -rf`.
8. Loop until `bd ready` is empty for the epic.

**Executor (fresh context, one workspace):**

1. Read the bead — **the bead description IS the plan** (no `PLAN.md`).
2. The workspace's working copy (`@`) is **already** an empty change on
   `trunk()` (created by `jj workspace add -r trunk()`); edits auto-snapshot
   into it. No `jj new` is needed — a redundant `jj new trunk()` would strand a
   phantom empty commit that `--skip-emptied` does **not** clean (it only
   abandons commits *emptied by* the rebase, not ones empty beforehand).
3. Do the TDD work. (jj agent-safety profile applies: `--no-pager`, `--git`
   diffs, `-m` always, edit conflict markers not `jj resolve`, change-IDs not
   commit-hashes, `jj git fetch` at task start.)
4. `jj commit -m "<type>(<bead-id>): <title>"` → stable change-id.
5. Return change-id to the orchestrator, which pins it as the canonical
   `jj-change:<id>` **label** (queryable; one storage mechanism, not "label or
   note"). The bead is **already** `in_progress` — set at workspace-add time
   during shed isolation (see [seam 1](seams/01-command-surface.md)), not at
   executor return. A custom `in_review` status MAY be configured later to
   distinguish "awaiting verify" from "in flight"; it is not a built-in.

### 5.1 The spine: bead ↔ change-id

A single pointer (each bead carries its jj change-id in the `jj-change:<id>`
label) collapses three GSD subsystems into one:

- **Recovery** — verify fails → `jj abandon $(change-id)` + reopen the bead
  (status → `open`; see [seam 1](seams/01-command-surface.md) `pick redo` for
  the `bd update --status open` vs `bd reopen` distinction).
- **Audit** — the PR body is generated from the epic's closed beads, each
  carrying its change-id; no `SUMMARY.md`.
- **Resume after compaction** — a fresh session reads `bd ready`/`bd blocked` +
  change-ids; it never parses markdown to reconstruct "where am I."

## 6. Ship

Epic done → `jj bookmark set <epic> -r @` → `jj git push -b <epic>` →
`gh pr create`, with the PR body assembled from the epic's closed beads. After
squash-merge: `jj git fetch && jj rebase -b @ -o main --skip-emptied &&
jj bookmark delete <epic>` (`-b @` explicit — never `-r @`, which truncates
multi-pick chains).

## 7. Engine: Go

Decision: **the engine is a Go binary** — a deterministic helper that wraps
`bd` + `jj` and choreographs workspaces. It does **not** dispatch agents; the
host runtime (Claude Code subagents, etc.) does, driven by Weft's command
markdown. This is confirmed to be GSD's actual model (`execute-phase.md`:
"Orchestrator coordinates, not executes"; subagent spawning is runtime-specific
via the host's `Agent(...)` Task tool, with a sequential fallback for runtimes
that can't reliably spawn).

Rationale:

- **No SDK needed on the primary path** — host runtime dispatches. The optional
  headless seam (autonomous waves in CI) is a `claude -p` exec, trivial from Go.
- **Distribution matches its neighbors** — a single static binary next to `bd`
  (Go) and `jj` (Rust); no Node runtime to require.
- **Workload fit** — goroutines + channels are the literal shape of the
  parallel-wave fan-out/collect.
- **Dev loop** — fast compile (the objection to Rust does not transfer to Go).

The one gap (no official Claude Agent SDK in Go) only matters if fully
autonomous, host-less waves become a *primary* goal; today's evidence says
that's optional. If that changes, Python + uv (official SDK + good distribution)
is the hedge.

## 8. Repo conventions

Mirrors the holomush Go setup:

- Apache-2.0 + SPDX headers (license-eye / `.licenserc.yaml`)
- colocated jj + git
- beads for issue tracking (dogfooded — Weft tracks its own development in beads)
- cocogitto (`cog.toml`) tag-only releases; conventional commits enforced in CI
- `.editorconfig`; `cmd/<binary>/` + `internal/` layout; go 1.26
- (deferred until build starts: Taskfile, `.golangci.yaml`, `.goreleaser.yaml`)

## 9. Open seams (next design steps)

Seam sub-specs live in [`docs/seams/`](seams/), each tracked as a child bead of
`weft-hjx`.

- The Go engine's command surface (the stable verbs the prompts call) —
  **designed:** [`docs/seams/01-command-surface.md`](seams/01-command-surface.md)
  (`weft-hjx.1`).
- How planning emits beads/warp (the `/weft-new-project` equivalent) instead of
  `ROADMAP.md`.
- Workspace lifecycle details: stale handling, cleanup on crash, the
  `.weft-workspaces/` layout.
- Conflict-resolution UX when a wave produces first-class conflicts.
- Which GSD command/agent markdown ports over as reference drafts.

## Attribution

Methodology and (eventually) several command/agent prompts are adapted from
**GSD Core**, MIT-licensed, © its contributors. Weft is independently licensed
Apache-2.0.

[jj]: https://github.com/jj-vcs/jj
[beads]: https://github.com/gastownhall/beads
