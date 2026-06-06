<!--
  ~ SPDX-License-Identifier: Apache-2.0
  ~ Copyright 2026 Weft Contributors
-->

# Bead–change spine

The bead–change spine is the durable link between a **bead** (the unit of
planned work in the warp) and a **jj change** (the unit of committed work in
the VCS). Every seam-level operation that touches both layers reads or writes
this spine.

## How the spine works

When `weft pick seal` completes a pick, it writes the jj change-id of the
sealed change back to the bead as the label `jj-change:<id>`. Subsequent
engine verbs — `integrate`, `land`, `redo`, `resume` — read that label to
locate the exact change without relying on ephemeral git SHAs or branch names.

```
bead weft-abc.1  ──(jj-change:sqpuoqvx)──▶  jj change sqpuoqvx
```

The label is written once on seal, updated on redo (new change, new label),
and consumed on land (change merged, label archived in closed bead state).

## Identity labels

Every weft bead carries a canonical set of labels that form its identity in
the warp:

| Label | Written by | Read by | Purpose |
|-------|-----------|---------|---------|
| `weft-ref:<ref>` | `weft plan emit` (seam 2) | `weft pick`, `weft shed` | Joins a bead to its plan row; the plan↔warp join key |
| `phase:<name>` | `weft plan emit` / engine transitions | scheduling logic | Lifecycle phase — `planned`, `active`, `sealed`, `landed` |
| `jj-change:<id>` | `weft pick seal` | `integrate`, `land`, `redo`, `resume` | Pins bead to its jj change-id |

### `weft-ref` detail

The `weft-ref:<ref>` label is the stable join between a plan document (the
specification artefact) and its bead (the scheduler/tracking record). Seam 2
(`weft plan emit`) creates both the bead and this label in a single atomic
step, ensuring the warp is always consistent with the plan.

### `phase` transitions

```
planned ──(pick start)──▶ active ──(pick seal)──▶ sealed ──(land)──▶ landed
                                ◀──(redo)──────────┘
```

### `jj-change` lifecycle

- Written: `weft pick seal` writes `jj-change:<new-id>` to the bead.
- Updated: `weft pick redo` abandons the old change (via the `jj-change` label)
  and reopens the bead (status → `open`); the fresh `jj-change` label is written
  by the subsequent `weft pick seal`, not by `redo` itself.
- Consumed: `weft pick land` reads the label, resolves the change, and integrates
  it into the target branch. The bead is then closed.

## `model:*` routing labels (Rule 5)

A bead MAY carry a `model:*` label that selects the dispatch model for the
agent assigned to work it. This is the weft analog of GSD Core's
`resolve-model` (spec §4/§8).

| Label | Dispatch model |
|-------|---------------|
| `model:haiku` | Claude Haiku (fast, low-cost; suitable for mechanical tasks) |
| `model:sonnet` | Claude Sonnet (balanced; default for most implementation beads) |
| `model:opus` | Claude Opus (highest capability; reserved for complex design beads) |

`weft shed` reads the `model:*` label when dispatching an agent to a ready
bead. If no `model:*` label is present, the configured default model applies
(see `weft.yaml` → `dispatch.default_model`).

The label is set at bead creation time (by `weft plan emit` or manually) and
MUST NOT be changed once a pick is active, as it would silently reroute an
in-flight agent.
