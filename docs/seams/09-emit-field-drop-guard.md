<!--
  ~ SPDX-License-Identifier: Apache-2.0
  ~ Copyright 2026 Weft Contributors
-->

# Seam 9 — Harden `plan emit` against bd field-drop

> Status: **draft** (pre-design-reviewer). Sub-spec of
> [`docs/design.md`](../design.md) §9 / [seam 2](02-planning-emission.md) §8
> (deferred "warp-plan.json JSON Schema + bd create --graph input mapping" and
> "confirm beads label constraints"). Tracked as bead `weft-hjx.15` (child of
> `weft-hjx`). **Rescopes** the original seam-2 §8 item: the formal warp-plan
> JSON Schema is *dropped* (see §2); this seam delivers the genuinely grounded
> half — the emission-side guard. `weft plan emit` / `internal/plan` /
> `internal/cli/plan.go` already exist (seam 2, `weft-hjx.3`); this seam modifies
> the emit path only.

## 1. Scope

Make `weft plan emit` **refuse to silently lose warp data**.

`bd create --graph` drops fields it does not recognise. weft's emit path
discards bd's diagnostics on success, so a dropped field — a real loss in the
authored warp — is invisible to the operator. This seam closes that gap by
surfacing what bd already reports and gating the emit on it.

In scope:

- A bd-backed **dry-run preflight** on the first-emit path (`bd create --graph`)
  that fails *before* mutating beads when a field would be dropped or the graph
  shape does not match what weft built (§4).
- Stop discarding `bd` stderr on the real create/import calls; surface warnings
  in the envelope (§4, §6).
- A `--allow-drop` escape hatch for the rare forward-compat case (§4.2).
- A soft `schema_version` compatibility check (§4.3).
- Documenting two facts this seam's grounding settled: bd drops `acceptance`,
  and bd accepts weft's colon-namespaced label families (§5) — closing two
  seam-2 §8 open items.

Out of scope (explicitly deferred):

- The formal warp-plan.json JSON Schema (§2 — dropped, not deferred-with-intent).
- Deep verification of the replan/`bd import` path — delivered in this seam (§7: post-import read-back).
- The other seam-2 §8 items (`[plan].structural` globs, drift detection,
  `has_checkpoint`) — separate seams.

## 2. Why the JSON Schema was dropped

The seam-2 §8 line read "`warp-plan.json` JSON Schema (formal) + the
`bd create --graph` input mapping". On inspection the **schema half has weak
intent** for weft as it exists today:

- `warp-plan.json` is weft-native (it replaces GSD's `ROADMAP.md` / `PLAN.md`
  markdown — not a carryover). Its **author is the `weft-planner` LLM agent**
  (`plugin/agents/planner.md`), driven by the `new-project` skill — not a human
  in an editor.
- The agent already has a validation feedback loop: it runs `weft plan check`,
  which calls `internal/plan.Validate()` — and `Validate()` does **structural
  *and* relational** checks (ref uniqueness, charset `^[a-zA-Z0-9._-]+$`,
  priority range, `needs` resolves to a known ref, self-`needs`, reserved
  `@epic`). A JSON Schema can express the structural subset but **not** the
  relational checks, so it would re-encode part of `Validate()` and add a second
  source of truth (drift risk) for no consumer weft has — there is no human/IDE
  `$schema` author and no third-party plan producer.

The schema only earns its keep once a human/third-party authoring surface
exists. Until then it stays deferred under `weft-hjx`. The **emission-side
mapping** half, by contrast, addresses a real, already-observed correctness
risk — this seam delivers that.

## 3. The gap (grounded)

Grounded live against `bd 1.0.5` this session:

`bd create --graph <file> --dry-run --json` reports the parsed graph and warns
about unknown fields:

```text
# stdout
{
  "dry_run": true,
  "node_count": 2,
  "edge_count": 1,
  "parent_deps": 1,
  "schema_version": 1,
  "nodes": [ { "key": "@epic", "priority": 2, "title": "…", "type": "epic" }, … ],
  "validation_notes": [ "dry-run validates the graph structure only; …" ]
}
# stderr
warning: graph plan node["@epic"] has unknown field(s): [acceptance] (silently dropped — see 'bd create --graph' schema)
warning: graph plan edge[0] has unknown field(s): [bogus_edge_field] (silently dropped — …)
```

Key properties:

- Unknown-field warnings go to **stderr** and the command still exits **0**.
- The dry-run `nodes` echo **omits `labels` and `description`** — so the
  verification surface is the **stderr warnings** plus the **`node_count` /
  `edge_count` / `schema_version`** fields, not an echo diff.

The current emit path (`internal/cli/plan.go`):

- `weft plan emit --dry-run` is **weft-internal** — it renders
  `planPreviewText` and returns; it **never calls bd**, so bd's warnings are
  never exercised on a dry run.
- The real `planFirstEmit` runs `bd create --graph <path>` and inspects
  `res.Stderr` **only when `res.Code != 0`**; on success it keeps `res.Stdout`
  and **discards stderr**. bd's drop warnings (stderr + exit 0) are thrown away.
- `planReplan` runs `bd import <path>` with the same `Code`-only check.

Net: a dropped field corrupts the warp silently. This contradicts weft's
fail-loud ethos and the gh-api error-classification convention (surface real
failures; never re-silence them).

## 4. The guard: a bd-backed dry-run preflight

On the **first-emit path** (`planFirstEmit`), before the real
`bd create --graph`, run a preflight:

```text
bd create --graph <staged-path> --dry-run --json
```

Parse its `stdout` (the dry-run envelope) and `stderr` (warnings), then gate on
three checks. Because the preflight is a dry run it **mutates nothing**, so
aborting here preserves seam-2's atomic-emit guarantee.

### 4.1 Checks

1. **Drop warnings.** Scan stderr for the stable marker substring
   `unknown field(s)` (per the gh-api convention: classify on bd's stable marker
   phrase, not loose English). Any match means a field weft sent would be lost.
2. **Count check.** Assert `node_count == 1 + len(picks)` (epic + picks) and
   `edge_count == len(derivation.Edges)` (authored `needs` + derived
   file-overlap edges; `parent_deps` is bd-derived and counted separately, so it
   is **not** included in the expected `edge_count`).
3. **schema_version.** Soft compatibility check — see §4.3.

weft fully controls the `GraphJSON` payload and already folds `acceptance` into
the description (§5), so a **correct** weft + a **compatible** bd produce **zero
drop warnings** and exactly-matching counts. A failure therefore means a weft
bug or a weft↔bd version skew — both of which must block.

### 4.2 Strictness and the `--allow-drop` escape

- Default: a drop warning **or** a count mismatch is a **hard failure**
  (`exit.Hardf` → exit 2; this is a data-integrity/system condition, not a user
  invocation error). The real `bd create --graph` is **not** run. The error
  surfaces bd's verbatim warning lines.
- `weft plan emit --allow-drop` downgrades a *drop warning* to a surfaced
  warning and proceeds to the real create. It does **not** bypass the count
  check (a count mismatch is structural, never an intended drop). The flag
  exists for the forward-compat case where a newer bd legitimately ignores a
  field weft still sends; it is loud and opt-in, never the default.

**`--dry-run` interaction.** This seam makes `weft plan emit --dry-run`
bd-backed: alongside the existing weft preview (`planPreviewText` — edges +
tolerated overlaps for the human gate, §3), it now runs the preflight
(`bd create --graph --dry-run --json`) and folds its warnings + counts +
`schema_version` into the dry-run envelope — replacing the prior behavior where
dry-run never called bd, so the planner agent's existing dry-run gate now
actually catches drops *before* the human approves. The strictness matrix
applies identically in dry-run: a drop exits non-zero (firing the agent's "on
red, fix and re-run" loop) unless `--allow-drop` downgrades it to a surfaced
warning + exit 0; a count mismatch is always hard. The only difference from a
real emit is that no mutating `bd create --graph` call follows.

### 4.3 schema_version (soft check)

`internal/plan` carries a constant `ExpectedGraphSchemaVersion` (currently `1`,
the version this build was grounded against). The preflight compares it to the
dry-run's `schema_version`:

- **Equal** → silent.
- **Different** → surface a **note/warning** (envelope `next` / a `warnings`
  field) but **do not block**. Reserving the hard-fail for actual drops/count
  mismatch avoids brittle coupling that would break `plan emit` on any benign bd
  schema bump. The mismatch is a signal to re-ground weft, not a stop.

### 4.4 The real create

After the preflight passes, run the real `bd create --graph <path>` and **stop
discarding stderr**: if `Code == 0` but stderr is non-empty, fold those lines
into the emit envelope (`warnings`) so nothing bd says is lost — a belt-and-
suspenders backstop for any warning the dry run did not surface.

## 5. Facts settled by grounding (close seam-2 §8 items)

These need no new code — they become spec-documented contract:

- **`acceptance` is dropped by bd** on graph nodes (confirmed via the dry-run
  warning). `emit.go`'s workaround — folding `epic.acceptance` into the epic
  description under an `## Acceptance` heading — is therefore **correct and
  necessary**, not a guess. Closes seam-2 §8 "`acceptance` representation
  unconfirmed". (The preflight's drop guard now also *enforces* that no future
  refactor reintroduces a raw `acceptance` node field.)
- **The colon-namespaced label families are accepted.** `weft-ref:<ref>`,
  `phase:*`, and `jj-change:<id>` passed the dry run with no warning or
  rejection. Closes seam-2 §8 "confirm beads label constraints accept the
  colon-namespaced families". (Note: dry-run validates structure; if bd later
  adds label *value* validation, the preflight surfaces it.)

## 6. Output contract

The table below summarises which `data` fields appear on each emit path.
`warnings` is `[]string`, **never null** — per the engine output-contract
convention (empty `[]` on a clean emit, never JSON `null`).

| Path | Mode | `warnings` present? | Notes |
|---|---|---|---|
| First emit (wet) | `create` | yes | Carries any surfaced bd warning or `schema_version` mismatch note. Empty on a clean emit. |
| First emit (dry-run) | `create` | yes | Preflight runs; warnings folded in. Empty when preflight is clean. Exit follows the strictness matrix (§4). |
| Re-plan / upsert (wet) | `upsert` | yes | Post-import read-back guard (§7): `bd import` stderr folded in; `verification` field present (never null); a non-round-tripping authored field or a failed read-back is exit 2. |
| Re-plan / upsert (dry-run) | `upsert` | yes | No bd call precedes the dry-run on this path (§7); `warnings` is always `[]`. |

The first-emit success envelope **preserves the existing seam-2 fields
unchanged** — `mode`, `created` (pick count), `edges` (the derivation slice),
`tolerated`, `bd_output` — and **adds two** (additive only; no rename, so the
`--pick` extractor and any `data.*` consumer keep working):

- `schema_version` — bd's observed graph schema version (§4.3). Present on the
  first-emit path (preflight provides it); absent on the re-plan path (bd import
  does not expose it).
- `warnings` — `[]string` (never null), carrying any surfaced bd warning or a
  `schema_version` mismatch note.

```json
{
  "ok": true,
  "verb": "plan.emit",
  "data": {
    "mode": "create",
    "created": 5,
    "edges": [ { "from": "p2", "to": "p1" } ],
    "tolerated": [],
    "schema_version": 1,
    "warnings": [],
    "bd_output": "…"
  },
  "next": "…"
}
```

Re-plan (upsert) wet envelope example:

```json
{
  "ok": true,
  "verb": "plan.emit",
  "data": {
    "mode": "upsert",
    "epic": "weft-42",
    "updated": ["p1"],
    "created": ["p2"],
    "removed": [],
    "deferred_edges": [],
    "tolerated": [],
    "warnings": [],
    "bd_output": "…",
    "verification": []
  },
  "next": "…"
}
```

Re-plan dry-run envelope example (`warnings` is always `[]` — no bd call
precedes it on this path). Note: `verification` is intentionally absent here —
dry-run does no import, so there is no read-back and no verification field.

```json
{
  "ok": true,
  "verb": "plan.emit",
  "data": {
    "dry_run": true,
    "mode": "upsert",
    "epic": "weft-42",
    "updated": ["p1"],
    "created": ["p2"],
    "removed": [],
    "deferred_edges": [],
    "tolerated": [],
    "warnings": []
  },
  "next": "…"
}
```

On a hard-fail preflight the envelope is the standard `exit.Hardf` error (exit
2) whose message includes bd's verbatim warning lines.

## 7. Replan / import path — post-import read-back guard

`bd import --dry-run` is **weak** — it reports only "Would import N issues…"
with no per-field warnings (grounded: it did not flag a bogus field; its help
states "the importer accepts every field"). So the rich preflight is not
available on the replan path.

The guard on `planReplan` is therefore **post-import read-back**: after a
successful `bd import`, weft immediately re-reads the epic's children via
`bd list --parent <epic> --json` and diffs the live state against the authored
expectations that `BuildReplan` captured for every pick.

### 7.1 What is verified

For each pick (sorted by ref for determinism):

- **Ref presence** — if the bead is absent from the read-back entirely (ref not
  found in any `weft-ref:<ref>` label) → hard failure (the create/update did
  not persist at all).
- **Title** — exact equality between the sent title and the read-back title.
- **Priority** — exact equality.
- **Labels (subset check)** — every authored label weft sent must be present in
  the read-back labels. bd may add its own labels (e.g. status/system labels),
  so equality is not required — only that no authored label was dropped.
- **Description presence** — when weft sent a non-empty description
  (`HasDesc == true`), the read-back description must be non-empty/non-whitespace.
  Content is not compared (bd may normalise whitespace/markdown).

`dependencies` are **out of scope** for read-back — they are not in `bd list`
output (only `dependency_count`) and are handled separately as `DeferredEdges`.

### 7.2 Outcome

- **Any discrepancy → hard exit 2** (`exit.Hardf`) with the full list of
  discrepancy strings. The import ran but the warp is incomplete; the operator
  must investigate. This is symmetric with the first-emit drop guard (§4.2).
- **Clean → exit 0** with `"verification": []` in the success envelope so
  `--json` consumers can confirm verification ran.
- **Read-back `bd list` itself fails** → hard exit 2 (can't verify == hard;
  we don't know whether the import persisted correctly).

### 7.3 Output contract update

The re-plan (upsert) wet envelope now carries an additional field:

- `verification` — `[]string` (never null): empty on a clean round-trip,
  populated with human-readable discrepancy strings on a hard failure (though in
  the failure case the envelope is the `exit.Hardf` error, not a success
  envelope).

## 8. Testing

- `internal/plan` (pure): no change to `GraphJSON`; add a test asserting the
  emitted payload carries **only** bd-known node/edge fields (a regression guard
  that no field is added without grounding).
- `internal/cli/plan.go` (fake `run.Runner`):
  - preflight detects a drop warning → hard-fail exit 2, no real-create call
    (assert the scripted runner never saw the non-dry-run `create --graph`).
  - `--allow-drop` → drop warning surfaced in `warnings`, real create runs.
  - count mismatch → hard-fail even with `--allow-drop`.
  - `schema_version` mismatch → surfaced in `warnings`, emit proceeds.
  - success → `warnings: []`, `schema_version` populated, real create runs after
    the dry-run preflight (assert call order).
- One **integration test** (build-tagged, real bd) asserting a representative
  `GraphJSON` produces zero `unknown field(s)` warnings and matching counts
  against the live bd — the drift sentinel that catches a bd schema change. CI
  runs this automatically via the `integration` job in `.github/workflows/ci.yml`
  (bd pinned at v1.0.5, checksum-verified; bump deliberately to re-validate
  GraphJSON against the new bd schema).

## 9. Open sub-seams / follow-ups

- **warp-plan.json JSON Schema** — stays deferred under `weft-hjx` until a
  human/third-party authoring surface exists (§2).
- `[plan].structural` default globs + `plan.overlap_max` default; declared-vs-
  actual file drift; `has_checkpoint` representation — unchanged seam-2 §8 items.

## 10. Decisions (ADR candidates)

- **D1** — Rescope seam 9: drop the warp-plan JSON Schema; deliver the emission-
  side field-drop guard instead. Rationale: the schema's only consumer would be
  a human/IDE author weft does not have; the agent author already validates via
  `plan check`; the emission drop is a real, observed correctness risk.
- **D2** — Guard mechanism is a **bd-backed dry-run preflight** that reuses bd's
  own warnings + counts, rather than weft re-deriving bd's schema. Keeps weft a
  thin wrapper and self-correcting against bd changes.
- **D3** — Drop/count failures are **hard** (exit 2) with an opt-in
  `--allow-drop` for drops only; `schema_version` mismatch is **soft**.
<!-- adr-capture: sha256=cb164d6b476f98b6; session=cli; ts=2026-06-07T00:25:48Z; adrs=weft-108,weft-axe,weft-2y4 -->
