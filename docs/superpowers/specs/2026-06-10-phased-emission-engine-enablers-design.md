<!--
  ~ SPDX-License-Identifier: Apache-2.0
  ~ Copyright 2026 Weft Contributors
-->

# Phased-emission engine enablers — design

**Date:** 2026-06-10
**Bead:** `weft-ccy.5` (phase B of epic `weft-ccy`)
**Status:** Approved by Sean (brainstorming session); pending design-review gate
**Parent spec:** `docs/superpowers/specs/2026-06-09-layer-a-interactive-phased-loop-design.md` §2

## Context

The Layer-A restoration spec requires the planner to emit multi-phase projects
as a **roadmap** — project epic → phase sub-epics → inter-phase `blocks` edges,
no picks — with pick-level planning happening just-in-time per phase via the
re-plan path. Three emission-layer gaps block that (spec §2); this design makes
them concrete. Everything else in the phased model is prompt-layer.

### Grounding (recorded as `bd note` traces on `weft-ccy.5`)

- `plan.Validate` (`internal/plan/plan.go`) unconditionally requires ≥1 pick —
  must be conditionalized for roadmap plans.
- The graph builder (`internal/plan/emit.go`) already models `parent_key` and
  `type` on nodes — phase sub-epics are expressible with today's wire types.
- `BuildReplan` **already applies** edges between matched picks via
  `importRecord.dependencies`; the §8 gap is only `DeferredEdges` (edges
  touching newly created picks), surfaced as data but never applied.
- `planFirstEmit` invokes `bd create --graph` without `--json` and discards the
  created ids; `warpReadback` already rebuilds the ref→id map post-import.
- deepwiki (gastownhall/beads): `bd create --graph` supports epic-type nodes
  parented to another epic node and `blocks` edges between epics (no type
  restrictions); `bd create --graph --json` returns a `GraphApplyResult` whose
  `IDs` map keys node keys to created issue ids; `bd import` has **no forward
  references** within a batch — `depends_on_id` must be an existing id.
- **Empirical (live probe, scratch bd DB, 2026-06-10):** `bd ready` respects
  epic blocking **transitively** — a pick under a `blocks`-blocked epic is not
  ready even with zero edges of its own, and releases when the blocking issue
  closes. Epic→epic edges alone are therefore graph-enforced phase gating; no
  derived pick-level gating edges are needed. (This corrected an earlier
  assumption; an auto-gating mechanism was designed and then deleted.)

### Decisions

| Decision | Choice |
|---|---|
| Inter-phase gating | Epic→epic `blocks` edges only — bd's transitive epic gating does the rest |
| `phases[]` vs `picks` | Mutually exclusive per plan file (roadmap emission vs pick emission); no hybrid |
| Envelope change | Additive `ids` map; no version bump |
| §8 scope | Apply deferred new-pick edges; removed-pick supersede stays deferred |

## 1. `phases[]` — warp-plan schema vNext

A new optional top-level block, mutually exclusive with `picks`:

```json
{
  "epic":   { "title": "...", "description": "...", "acceptance": "..." },
  "phases": [
    { "ref": "p1", "title": "...", "description": "...", "acceptance": "...", "needs": [] },
    { "ref": "p2", "title": "...", "description": "...", "acceptance": "...", "needs": ["p1"] }
  ]
}
```

**Model** (`internal/plan/plan.go`): a `Phase` struct (`Ref`, `Title`,
`Description`, `Acceptance`, `Needs`) and `Phases []Phase` on `WarpPlan`.
Unknown-field tolerance in `Parse` is unchanged.

**Validation** (`Validate`, conditionalized):

- `phases` present and `picks` present → invalid ("a plan carries phases or
  picks, not both").
- `phases` present: ≥1 phase; each `ref` required, matches the existing
  `refPattern`, not `@epic`, unique; `title` and `description` required;
  `needs` must reference a known sibling phase ref and not the phase itself.
  Cycle detection stays delegated to `bd create --graph` (as for picks today).
- `phases` absent: today's rules apply **byte-identically**, including
  "at least one pick is required".
- `plan check`'s human summary text branches with the shape: roadmap plans
  report `valid: N phase(s), no issues` (never `0 pick(s)`, which would
  misread as an empty plan). The data envelope (`valid`, `issues`) is
  unchanged.

**Emission** (`GraphJSON`): when phases are present, nodes are the project epic
(`@epic`, exactly as today) plus one node per phase: `Type: "epic"`,
`ParentKey: EpicKey`, `Labels: ["weft-ref:<ref>"]`, default priority,
acceptance folded into description exactly as the project epic does today (the
graph schema's acceptance field remains unconfirmed — same posture). Edges are
the authored `needs`, emitted as `blocks` edges between phase nodes. No
file-overlap derivation runs (phases carry no `files`).

**Preflight contract** (`planFirstEmit` → `plan.CheckPreflight`): the expected
counts the seam-9 gate compares against bd's dry-run MUST branch with the plan
shape. Pick plans pass `wantNodes = 1+len(picks)`, `wantEdges = len(d.Edges)`
(today's behavior, unchanged). Roadmap plans pass `wantNodes = 1+len(phases)`,
`wantEdges = sum(len(phase.needs))` — the phase edges come from the authored
`needs` inside `GraphJSON`, not from `Derive`, so `d.Edges` is empty on this
path and must not be used. Without this branch, every roadmap emit hard-fails
at the preflight count check (ADR weft-2y4 makes a count mismatch an
unconditional exit 2 that `--allow-drop` cannot override).

The `weft-ref:<ref>` label on phase sub-epics reuses the existing pick identity
mechanism, so phase epics are addressable the same way picks are: future
tooling can resolve phase refs from labels, and `plan emit --epic
<phase-epic-id>` (the per-phase JIT path) needs no change to find its target.

**Gating semantics (documented, not implemented):** the roadmap's epic→epic
`blocks` edges gate entire phases. Verified bd behavior: children of a blocked
epic are excluded from `bd ready` transitively and release when the blocker
closes. Phase N's epic closes via `weft finish` (phase = ship unit = one PR),
which releases phase N+1.

## 2. Epic-id echo

`planFirstEmit` adds `--json` to the `bd create --graph` invocation and parses
the result's `IDs` map (node key → created bead id). The `plan.emit` envelope
gains an additive field:

```json
"ids": { "@epic": "weft-abc", "p1": "weft-abc.1", "p2": "weft-abc.2" }
```

- Present on every successful create-path emit (pick plans get `@epic` plus one
  entry per pick ref; roadmap plans get `@epic` plus one entry per phase ref).
- Initialized as an empty-capable map; serialization must never be `null`
  (house empty-list/empty-map contract).
- **Envelope counts branch with the plan shape.** Pick plans keep today's
  fields untouched (`"picks"` on dry-run, `"created"` on live emit, both
  `len(picks)`). Roadmap plans carry `"phases": len(phases)` in place of
  `"picks"`, and `"created": len(phases)` (the phase sub-epics, mirroring the
  pick semantics of counting created children, not the project epic). The
  `"picks"` key is **absent** on the roadmap path — never present-but-zero,
  which would misread as an empty plan. Consumers (the JIT phase planner, the
  `new-project` skill) branch on which key is present.
- If `--json` parsing fails, hard-fail (`exit.Hardf`) — the warp was created
  but the contract output could not be produced; the operator must investigate
  rather than receive a silently degraded envelope.
- No `schema_version` bump: the envelope change is additive, and the graph
  payload's version is untouched. `bd_output` (raw stdout) stays as-is for
  transparency — `ids` is added alongside it, keeping the change strictly
  additive. The human-text summary gains a readable id list.

Consumer note: the seam-10 weave fixture currently works around the missing
ids by querying bd for the only epic in a scratch DB; once `ids` exists it is
the natural first consumer, giving the new field a standing regression test.
(How the test migrates is plan-level detail, not contracted here.)

## 3. §8 completion — apply deferred edges on re-plan

`planReplan`, after `bd import` succeeds and `warpReadback` rebuilds the
ref→id map (both exist today):

1. For each `DeferredEdge`, resolve both endpoints through the refreshed map.
2. Apply each via `bd dep add <from-id> <to-id> --type blocks` (confirmed: `bd
   import` cannot forward-reference ids within a batch, so post-import
   application is the only correct mechanism).
3. Any unresolvable endpoint or failed `dep add` → `exit.Hardf` ("re-plan
   applied but N edge(s) could not be wired; the warp is incomplete —
   investigate"), matching the existing `VerifyReplan` read-back posture.
4. The envelope key is **renamed** `deferred_edges` → `applied_edges`: the
   semantics change from "surfaced, NOT applied" to "applied post-import" is
   the feature, and keeping a name that says "deferred" would misstate the
   contract. No machine consumer reads `deferred_edges` today (it was
   surfaced for humans), so the rename is safe; the shape (edge list, `[]`
   never null) is unchanged. Seam-2 §7/§8 docs are updated to match.

Removed-pick supersede remains an open §8 sub-seam (out of scope here).

## Error handling summary

| Failure | Behavior |
|---|---|
| Plan with both `phases` and `picks` | `plan check` issue; `plan emit` refuses (invocation error, exit 1) |
| `bd create --graph --json` unparseable | Hard fail (exit 2) after creation — loud, never a degraded envelope |
| Deferred-edge `dep add` failure | Hard fail (exit 2) with the unwired edge list |
| `phases` absent | Behavior byte-identical to today (regression-tested) |

## Testing

- **`internal/plan` unit tests:** validation matrix (both-present, empty
  phases, dup/invalid/self/unknown refs); roadmap `GraphJSON` golden shape
  (epic node + phase epic nodes + blocks edges, `weft-ref` labels); pick-plan
  `GraphJSON` unchanged (golden regression); `BuildReplan` deferred-edge
  resolution table; empty-map/empty-list contracts.
- **`internal/cli` tests (injectable runner):** `ids` map parsed and emitted on
  the create path; hard-fail on unparseable graph output; deferred-edge `bd dep
  add` invocations and hard-fail on failure; `--allow-drop` rejection on the
  re-plan path unchanged.
- **Integration (scratch bd, weave-test harness):** roadmap emit round-trip —
  `phases[]` plan → project epic + phase sub-epics + edges in bd; **a pick
  created under a blocked phase epic is absent from `bd ready` and appears
  after the blocking phase closes** (pins the transitive-gating behavior this
  design depends on, so a bd regression surfaces in our suite); per-phase
  re-plan into a phase sub-epic applies new-pick edges.
- Seam-9 preflight: the field-drop guard applies to roadmap payloads through
  the same gate, but the count contract is NOT automatic — unit tests must
  cover the branched `wantNodes`/`wantEdges` (see §1 Preflight contract),
  including a roadmap plan with inter-phase `needs` edges passing the count
  check and a deliberate mismatch still hard-failing.

## Out of scope

- The phased planner / `new-project` prompt changes (epic phase C, `weft-ccy.3`).
- Removed-pick supersede (§8 remainder).
- Hybrid plans (roadmap + phase-1 picks in one file) — rejected in the parent
  spec's plan-timing decision.
- Any `weft` verb surface change beyond `plan emit`'s output; no new flags.

## Discovered during implementation

**bd import ignores the JSONL `parent` field on both the create and update paths
(verified 2026-06-10).** Consequently, the shipped re-plan path does not rely on
`parent` in the JSONL payload for new-pick parentage; instead, `planReplan`
parses `bd import --json`'s positional ids array (`ids[i]` = bead id for JSONL
record `i`) and issues `bd dep add --type parent-child` for each created pick
before the scoped read-back, so the read-back sees the new picks as children of
the epic. `importRecord.Parent` is retained in the struct for forward-compatibility
in case `bd import` adds parent support in a future release. The wiring step
uses the same hard-fail posture as `VerifyReplan`: any empty id or failed
`dep add` exits 2 immediately.
