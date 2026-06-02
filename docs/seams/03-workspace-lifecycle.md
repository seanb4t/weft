<!--
  ~ SPDX-License-Identifier: Apache-2.0
  ~ Copyright 2026 Weft Contributors
-->

# Seam 3 — Workspace lifecycle

> Status: **design-reviewer READY** (round 2). Sub-spec of
> [`docs/design.md`](../design.md) §9 open seam 3. Tracked as bead `weft-hjx.2`
> (child of `weft-hjx`). Round-1 + round-2 minors fixed inline. No
> implementation exists yet.

## 1. Scope

The jj workspace lifecycle for the parallel-wave executor model (design.md §4
Approach C): the `<repo>_worktrees/` layout, identity, creation, jj-stale
handling, and crash-orphan reaping. Introduces one new verb — `weft reap` —
extending the [seam 1](01-command-surface.md) surface.

Closes hooks deferred from seam 1: the `shed form --max` parallelism dial
config, `ws add` paths, and `shed isolate`/`shed cleanup` internals.

Out of scope: the command-surface contract (seam 1); conflict-resolution UX
(seam 4); how planning emits beads (seam 2).

## 2. Thesis: jj + beads delete `worktree-safety.cjs`

GSD's `worktree-safety.cjs` is a reaper with three guards (lock-mtime,
PID-liveness, branch-ancestry), a transactional ff-merge cleanup tail, and a
per-worktree recovery sentinel file. Most of it dissolves on Weft's substrate:

| GSD mechanism | Why it exists | Weft |
|---|---|---|
| ff-merge → remove → delete-branch cleanup tail | git worktrees need merge-back; each has a temp branch | `jj workspace forget` + `rm -rf`. Shared commit graph: the change already exists, no merge, no branch. |
| **Ancestry guard** (only reap merged worktrees) | `git worktree remove` of unmerged work **loses it** | **Deleted.** `jj workspace forget` is non-destructive — committed changes stay full graph citizens (recoverable by change-id), and the forget is `jj undo`-able. Reaping a *sealed* pick never loses work. (Grounded: deepwiki `jj-vcs/jj` — `forget` = `remove_wc_commit(ws)` in one transaction; only an **empty** `@` is abandoned.) |
| Recovery sentinel (`.recovery-pending.json`) | durable "a worktree was created" record | **Deleted.** The bead *is* the sentinel: `status=in_progress` + `jj-change:<id>` label (§5.1). |
| mtime staleness for crash detection | no external source of task truth | **Deleted.** Crash detection is bead-state reconciliation (§5). |
| PID-liveness guard | don't reap a running agent | **Kept** — the one guard that survives, as a race guard only (§5). |

What remains to specify: layout/identity (§3), lifecycle + ordering (§4),
reaping (§5), stale handling (§6), creation/parallelism (§7).

## 3. Layout & identity

- **Workspaces are siblings of the target repo**, never nested inside its
  working copy (a nested working copy breaks jj/colocated-git snapshotting).
- **Path:** `../<repo>_worktrees/<bead-id>/`, the jj-skill convention. Repo
  prefixing avoids collisions across multiple checkouts. (`workspace.root` in
  `.weft/config.toml` overrides the parent dir.)
- **Identity:** the jj **workspace name is the bead-id** (design.md §4
  `--name <bead-id>`). This is the join key for `weft reap` — given a
  workspace, its owning bead is its name; no separate mapping table.
- **Dot caveat:** bead-ids contain dots (`weft-hjx.1`). The implementation MUST
  confirm jj accepts dotted workspace names; if not, apply a documented,
  reversible sanitization (e.g. `.` → `__`) so the name ↔ bead-id mapping stays
  bijective. **If sanitization is in effect, the reap loop (§5) reads
  `bead = desanitize(ws.name)`**, not `ws.name` literally — the inverse mapping
  is part of the join.

## 4. Lifecycle & the ordering invariant

A pick's workspace moves through:

```
(none) ──isolate──▶ in-use ──collect/integrate──▶ landed ──cleanup──▶ (none)
                       │                              
                       └────────── crash ───────────▶ orphan ──reap──▶ (none)
```

**Ordering invariant (refines seam 1 `shed isolate`):** set the bead
`in_progress` **before** `jj workspace add`. Consequences:

- A crash *between* the two leaves an `in_progress` bead with **no** workspace.
  `weft resume` (read-only, seam 1 §4.5) **surfaces** it as in-flight; recovery
  is then an *explicit* reset to `open` (`pick redo`, which tolerates a missing
  `jj-change` label, or a bare `bd update --status open`), after which the next
  `shed form` re-picks it. `resume` never re-dispatches by side-effect. Nothing
  to reap, nothing lost.
- There is never a window where a workspace exists while its bead still looks
  reapable (`status != in_progress`). This removes the need for an age-based
  race guard.

`shed isolate` per member therefore is: `jj git fetch` (once per wave) →
`bd update <bead> --status in_progress` → `jj workspace add ../<repo>_worktrees/<bead-id> --name <bead-id> -r trunk()`.

## 5. Reaping: `weft reap`

Crash recovery is **bead-state reconciliation**, not filesystem archaeology.

```
weft reap [--epic E]          # reconcile jj workspace list ↔ bead state
  for ws in `jj workspace list` (excluding default):
    bead = desanitize(ws.name)             # §3: name is the join key
    if bead.status != in_progress:        → orphan      → reap
    elif bead in active_wave:              → in-use      → skip   # fast-skip (see below)
    elif executor_live(bead):              → in-use      → skip   # the authoritative guard
    else:                                  → crashed     → reap
  reap = jj workspace forget <name> + rm -rf <path>
```

- **`active_wave`** is the in-memory bead-id set of the wave the orchestrator is
  currently driving. At the primary entry points — orchestrator startup and
  `resume` — no wave is in flight, so it is **empty** and the check is a no-op;
  it exists only as a cheap fast-skip if `reap` is ever invoked mid-wave.
  `executor_live` is the authoritative guard, so correctness never depends on
  `active_wave` being populated.
- **No ancestry guard** — `forget` is non-destructive (§2): a *sealed* pick is a
  full graph citizen; an unsealed crash leaves at worst an empty `@` (abandoned)
  or a recoverable snapshot commit. Reaping is always safe.
- **No sentinel, no mtime** — the bead is the record; status is the signal.
- **The single guard is liveness** (`executor_live`): don't reap a workspace
  whose executor is actively running. Its exact mechanism (PID file written at
  dispatch, host-runtime task query) is a §10 sub-seam; the *model* only
  requires a boolean "is this bead's executor alive."
- **`weft reap` is the safety net for incomplete cleanup.** `pick land` closes
  the bead; if `shed cleanup` did not finish, the lingering workspace now has a
  *closed* bead → `status != in_progress` → reaped by the same path. No
  special-casing.
- **Resolution workspaces are a second kind.** [Seam 4](04-conflict-resolution.md)
  creates `<sanitized-bead-id>-resolve` workspaces for conflict resolution. The
  reaper recognizes the `-resolve` suffix as a *kind* marker: it strips the
  suffix, `desanitize`s the remainder to the **owning** bead-id, and applies the
  same rule — reap unless the owning bead is `in_progress` **and** a resolver is
  live (`executor_live`). (Bead-ids never end in `-resolve`, so the suffix is an
  unambiguous discriminant.) `conflict finalize` reaps on the happy path; this
  rule is the crash safety net.

### 5.1 `reap` vs `shed cleanup`

| Verb | Scope | When | Purpose |
|---|---|---|---|
| `shed cleanup <wave>` (seam 1) | manifest (one wave) | happy path, end of each wave | tear down the wave's workspaces |
| `weft reap [--epic E]` | broad (all workspaces) | orchestrator init / `resume` after a crash | collect orphans no manifest covers |

`reap` runs at orchestrator startup and is idempotent — safe to call before any
wave forms.

## 6. Stale handling (jj-stale ≠ crash-orphan)

"Stale" is two distinct conditions; conflating them is a trap:

| Condition | Meaning | Detection | Action |
|---|---|---|---|
| **crash-orphan** | executor died | bead-state reconciliation (§5) | reap |
| **jj-stale** | working copy fell behind after history was rewritten under it | `jj workspace list` / `jj st` reports stale | §6.1 tiered policy |

jj-stale is **rare** in Weft: waves are sequential, integration runs *after*
collect, and workspaces are cleaned per-wave — so sibling executors never
rewrite each other's trunk. It surfaces mainly on `resume` after a long gap, or
if a human runs jj ops in the default workspace mid-wave.

### 6.1 Tiered jj-stale policy

```
on stale workspace for an in_progress bead:
  if change is non-empty:   jj workspace update-stale   # preserve real work, continue
  else:                     jj abandon <change>; bd update <bead> --status open;
                            reap; let resume re-isolate fresh from trunk()
```

`update-stale` is non-destructive (rebases the working copy onto current op
state); any conflicts it surfaces are first-class jj objects (§4), handled by
the executor, not blockers. Empty/near-empty workspaces aren't worth
preserving — a clean re-isolation is simpler than reconciling a stale,
barely-started working copy.

## 7. Creation & parallelism

- **Serialize `jj workspace add`.** GSD serialized git worktree-add to dodge
  `.git/config.lock` contention. Whether jj contends identically is unverified,
  so `shed isolate` creates workspaces serially by default (it is one Go
  process; trivial to serialize). Parallelizing creation is a §10 optimization,
  gated on confirming jj concurrent-add safety. Executors still run in parallel
  *after* creation.
- **Parallelism dial:** `shed.max` in `.weft/config.toml` caps wave size
  (concurrent workspaces = concurrent executors = token cost). Conservative
  default (≈3); `shed form --max N` overrides per-invocation.
- **Trunk freshness:** `shed isolate` runs `jj git fetch` once per wave so
  `-r trunk()` is current before creating workspaces (avoids isolating onto a
  stale trunk that integration would immediately rebase).

## 8. Config: `.weft/config.toml`

The one project-local config file Weft needs so far:

```toml
[shed]
max = 3                       # parallelism dial; --max overrides

[workspace]
root = "../weft_worktrees"    # optional; default is ../<repo>_worktrees
```

## 9. Cross-spec reconciliations

This seam refines two earlier docs (both updated alongside this spec):

- **design.md §4** — workspace path `../.weft-workspaces/<bead-id>` →
  `../<repo>_worktrees/<bead-id>` (jj convention).
- **seam 1 `shed isolate`** — the wrapped sequence is reordered to
  *status-first, then workspace-add* (the §4 ordering invariant).

## 10. Open sub-seams (next design steps)

- `executor_live` mechanism (PID file at dispatch vs host-runtime task query).
- Whether `jj workspace add` is concurrent-safe (gates parallel creation).
- Disk-pressure policy if a wave's workspaces exceed a size budget.
- `.weft/config.toml` full schema + precedence (flag > env > file > default).

## Attribution

Lifecycle model contrasts with **GSD Core**'s `worktree-safety.cjs`,
MIT-licensed, © its contributors. Weft is independently licensed Apache-2.0.
