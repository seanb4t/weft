<!--
  ~ SPDX-License-Identifier: Apache-2.0
  ~ Copyright 2026 Weft Contributors
-->

# Seam 2 — Planning → warp emission

> Status: **exploratory design**, captured from a brainstorming session.
> Sub-spec of [`docs/design.md`](../design.md) §9 open seam 2. Tracked as bead
> `weft-hjx.3` (child of `weft-hjx`). Not yet `design-reviewer`-approved. No
> implementation exists.

## 1. Scope

How a spec becomes the **warp** — the bead graph (epic → issues + dependency
edges) — instead of GSD's `ROADMAP.md` + `PLAN.md` hierarchy. The
`/weft-new-project` equivalent.

Defines the **emission contract**: the `warp-plan.json` the planning step
produces, and the `weft plan` verbs that turn it into beads. Does **not** define
the planning *prompt* (the ported `/weft-new-project` command markdown is seam
5) nor conflict-resolution UX (seam 4).

Closes design.md §3's "ROADMAP.md + phase/plan/task hierarchy → beads epics →
issues → sub-issues with dependency edges" mapping.

## 2. The boundary: author (agent) vs emit (engine)

Per design.md §7 the engine never dispatches agents or authors prose. Planning
splits cleanly:

| Stage | Owner | Output |
|---|---|---|
| **Author** the plan (adaptive questions, research, requirement extraction, task breakdown, file-ownership estimates) | agent / command-markdown (judgment) | `warp-plan.json` |
| **Emit** the warp (validate, derive dep edges, dry-run, create atomically) | the `weft` engine (deterministic) | beads epic + issues + edges |

This is the seam-1 hybrid rule: emission is dangerous multi-step graph
choreography → coarse engine verb; authoring is judgment → agent.

The emission primitive already exists in the brain: **`bd create --graph
<json>`** atomically creates an epic + issues + dependency edges from one file
(`bd import` JSONL upsert is the incremental counterpart). `weft plan emit`
wraps it with validation, dep derivation, and a dry-run gate — it does not
reinvent graph creation.

## 3. `warp-plan.json`

The authored artifact. Human/agent-produced, transient — once emitted, **beads
is the source of truth** (there is no persisted `ROADMAP.md`).

```json
{
  "epic":  { "title": "…", "description": "…", "acceptance": "…" },
  "picks": [
    { "ref": "p1",
      "title": "…",
      "description": "the bead description IS the plan (design.md §5) — read_first, steps, acceptance",
      "needs":  ["p2"],
      "files":  ["internal/loom/rebase.go", "internal/loom/*.go"],
      "priority": 2,
      "labels": ["phase:build"] }
  ]
}
```

- `ref` is a **stable, plan-local identity key** — the author MUST keep it
  stable across revisions, because it is the durable plan↔warp join (§7). On
  emit, `weft plan emit` assigns each pick a bead-id via `bd create --graph`,
  rewrites `needs` edges to those bead-ids, and **stamps a `weft-ref:<ref>`
  label** on each created bead. The `ref → bead-id` mapping therefore lives in
  the warp itself (beads is the brain) — no sidecar state file, and the plan
  file is never mutated post-emit.
- `description` carries the whole plan for that pick — there is no separate
  `PLAN.md` (design.md §5: the bead description *is* the plan).
- `files` is the pick's declared file-ownership estimate; it drives §4 dep
  derivation. `needs` is the author's explicit dependency.

## 4. Dependency derivation & the overlap policy

`weft plan emit` builds the warp's edges from two sources:

1. **Explicit `needs`** — authored true dependencies (always become edges).
2. **File-overlap edges** — derived, per the policy below.

### 4.1 Overlap is conflict-minimization, not crash-safety

With per-workspace isolation (seams 1/3), two same-shed picks editing the same
path edit **separate working copies** — not a disk race. The collision surfaces
at *integration* as a **first-class jj conflict** (design.md §4), resolved
post-hoc. So file-overlap edges exist to *minimize* integration conflicts, not
to prevent corruption. This is a dial in tension with §4's "conflicts are cheap"
thesis, so it is deliberately **advisory**, not absolute.

### 4.2 Advisory-threshold policy

For each pair of otherwise-independent picks `(a, b)`:

```
shared = files(a) ∩ files(b)
if shared contains a STRUCTURAL file                 → add edge (serialize)
elif |shared| > plan.overlap_max                     → add edge (serialize)
else if shared ≠ ∅                                   → WARN + tolerate
```

- **Structural files** (manifests, lockfiles, schemas, codegen output) are
  serialized on *any* overlap — concurrent edits there are almost always real
  conflicts. The set is `[plan].structural` globs in `.weft/config.toml`
  (language-agnostic starter defaults, project-overridable).
- **Incidental overlap** beyond `plan.overlap_max` serializes; at or below it,
  `emit` **warns and tolerates** — the picks stay in the same shed and any
  resulting conflict is a first-class jj object resolved via seam 4.
- Edge direction for derived edges is deterministic: **provisional-`ref`
  lexicographic** (bead-ids do not exist until `emit` runs `bd create --graph`,
  so the tiebreaker keys on `ref`, not bead-id). Stable across re-emission. The
  later pick depends on the earlier, so the earlier lands in the prior shed.

### 4.3 Declared vs actual files

`files` is a plan-time *estimate*; a pick may touch an undeclared path. Under
this policy an undeclared overlap simply produces an unanticipated first-class
conflict at integration — degraded, not fatal (workspace isolation still holds).
Detecting declared-vs-actual drift (comparing `jj diff` paths against `files`)
and feeding it back into planning is a §8 sub-seam.

## 5. The `weft plan` verbs

Extends the [seam 1](01-command-surface.md) surface.

| Verb | Kind | Wraps | Notes |
|---|---|---|---|
| `plan check <file>` | thin | schema + acceptance validation of `warp-plan.json` | Exit 0 + `{valid: bool, issues: […]}`. No mutation. |
| `plan emit <file> [--dry-run]` | coarse | derive edges (§4) → preview → `bd create --graph` | `--dry-run` prints the full warp (epic, issues, edges) **and the warn+tolerate overlaps** without mutating. Without it, emits atomically. |

`emit` follows the seam-1 contract: text default, `--json` envelope, engine-
success exit codes (a warn+tolerate overlap is **data on exit 0**, not an
error). The dry-run preview is the human approval gate before the warp is
written — mirrors how this very project is being planned.

`emit` is re-runnable: edge derivation is pure computation, and the only
mutation is the atomic `bd create --graph` (or the §7 upsert on re-plan). An
interruption before that single call leaves no partial warp; re-running is safe.

## 6. Warp structure

The emitted graph:

- **epic** = the ship unit (design.md §6: `weft finish` operates on an epic =
  one PR). One `warp-plan.json` → one epic.
- **issues** = picks (one bead → one pick → one jj change, per the vocabulary).
- **edges** = explicit `needs` ∪ derived file-overlap edges (§4).
- **labels** = `phase:*`, any authored labels, and the `weft-ref:<ref>` identity
  label stamped at emit (§3/§7); the `jj-change:<id>` label is added later at
  execution time (seam 1 `pick seal`), not at emission.

`bd ready` then computes the sheds: because the dangerous overlaps are encoded
as edges, the ready set at any point is parallelizable with bounded conflict
risk — the warp's *tension* (its dep structure) is what makes the weave safe.

## 7. Re-planning

A spec evolves; the warp must follow without being rebuilt from scratch:

- **Additive / changed picks:** re-run `weft plan emit`. It rebuilds the
  `ref → bead-id` map by reading the epic's `weft-ref:<ref>` labels (§3 —
  identity lives in beads), then upserts via `bd import`: a `ref` with no
  matching label creates a new pick; a matched `ref` updates that bead in place.
  Idempotent — re-emitting an unchanged plan is a no-op. (This is why `ref`
  values must be stable: they are the upsert key.)
- **Removed picks:** a pick dropped from the plan is **superseded**
  (`bd supersede`), never silently deleted — its history and any landed change
  stay auditable. The exact reconciliation (diff the plan against the live epic)
  is a §8 sub-seam.

## 8. Open sub-seams (next design steps)

- `warp-plan.json` JSON Schema (formal) + the `bd create --graph` input mapping.
- `[plan].structural` default globs + `plan.overlap_max` default value.
- Declared-vs-actual file drift detection (compare `jj diff` paths to `files`).
- `has_checkpoint` representation (GSD's user-interaction gate → a bead flag /
  `bd human`).
- Re-plan reconciliation for removed/reordered picks (supersede policy).

## 9. Cross-spec note

Introduces the `weft plan` verb group (`check`, `emit`) — additive to the seam-1
surface; no existing verb changes. The `[plan]` config block extends seam 3's
`.weft/config.toml`.

## Attribution

Planning methodology (adaptive questioning, file-ownership-derived waves) adapted
from **GSD Core**'s `/gsd-new-project` + `gsd-planner`, MIT-licensed, © its
contributors. Weft is independently licensed Apache-2.0.
