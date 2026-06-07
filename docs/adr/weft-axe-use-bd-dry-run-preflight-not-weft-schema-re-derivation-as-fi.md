<!-- markdownlint-disable MD013 -->
<!-- adr-render: source=bd:weft-axe; do not edit manually; use `/adr update weft-axe` -->

# Use bd dry-run preflight, not weft schema re-derivation, as the field-drop guard

**Date:** 2026-06-07
**Status:** Accepted
**Decision:** weft-axe
**Deciders:** Sean Brandt

## Context

`weft plan emit` must detect fields that `bd create --graph` would silently drop, before mutating the warp. Two approaches exist: weft could independently model bd's accepted graph schema, or it could invoke bd in dry-run mode and parse bd's own diagnostics. Grounded against bd 1.0.5: `bd create --graph <file> --dry-run --json` warns on unknown fields to stderr (exit 0) and reports `node_count`/`edge_count`/`schema_version` on stdout — a no-mutation call.

## Decision

Implement the guard as a bd-backed dry-run preflight: `weft plan emit` runs `bd create --graph <path> --dry-run --json` before the real create, parses bd's stderr drop-warnings (matched on the stable `unknown field(s)` marker) and stdout node/edge counts, and gates on the result. weft does NOT maintain an independent model of bd's accepted fields.

## Rationale

- bd is authoritative for its own accepted fields; delegating to bd avoids a second source of truth.
- bd's stable warning marker substring (`unknown field(s)`) gives version-stable classification without parsing free-form English (mirrors the gh-api error-classification convention).
- weft's design principle is a thin wrapper over bd/jj/gh; re-deriving bd's schema inverts that relationship and creates a maintenance burden on every bd field change.
- `node_count`/`edge_count` give a structural-integrity signal orthogonal to field-level warnings; `schema_version` gives a drift signal.

## Alternatives Considered

- **weft re-derives bd's accepted field set independently.** No runtime dependency on bd's dry-run flag and fully offline-testable, but creates a second source of truth, requires a weft update on every bd field change, carries high drift risk, and contradicts the thin-wrapper design. Rejected.

## Consequences

- Positive: the guard auto-tracks bd schema changes with no weft code change; the dry-run is a no-mutation call preserving seam-2's atomic-emit guarantee; a stderr-surfacing backstop on the real create ensures no warning is discarded even on success.
- Negative: the first-emit path now makes two bd subprocess calls (dry-run + real create); weft is coupled to bd's dry-run output format, so a bd flag rename requires a weft update.
- Neutral: `bd import --dry-run` is weak (no per-field warnings), so the replan path gets only the stderr-surfacing backstop, not the rich preflight; deeper replan verification is a follow-up bead.
