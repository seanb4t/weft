<!-- markdownlint-disable MD013 -->
<!-- adr-render: source=bd:weft-2y4; do not edit manually; use `/adr update weft-2y4` -->

# Hard drop/count failures (exit 2) with opt-in --allow-drop; soft schema_version

**Date:** 2026-06-07
**Status:** Accepted
**Decision:** weft-2y4
**Deciders:** Sean Brandt

## Context

The bd preflight can detect three distinct conditions: a field-drop warning, a node/edge count mismatch, and a `schema_version` difference. Each has different semantics: a drop means data loss in the authored warp; a count mismatch means weft's derivation and bd's parse structurally disagree; a `schema_version` difference may be a benign bd bump. The strictness choice defines the error contract of the `weft plan emit` public interface.

## Decision

Drop warnings and count mismatches are hard failures (exit 2, `exit.Hardf`) by default. `--allow-drop` downgrades drop warnings ONLY to surfaced warnings and proceeds; count mismatches are ALWAYS hard regardless of `--allow-drop`. `schema_version` mismatch is a soft warning surfaced in the envelope that does not block emit. The matrix applies identically in `--dry-run`.

## Rationale

- A dropped field is a warp data-integrity failure; defaulting to hard-fail matches weft's fail-loud ethos and the gh-api error-classification convention.
- A count mismatch is structural (weft built a graph bd disagrees with) and is never an intended forward-compat case, so it must always hard-fail.
- A `schema_version` mismatch is a signal to re-ground weft, not a stop; hard-failing on it would break `plan emit` on any benign bd patch release.
- `--allow-drop` is explicitly opt-in and loud, targeting the narrow forward-compat case (a newer bd legitimately ignores a field weft still sends); it never silences the count check.

## Alternatives Considered

- **All three conditions hard-fail (no escape hatch).** Maximally strict but benign `schema_version` bumps would break `plan emit`, and operators have no forward-compat path for a known, accepted drop. Rejected.
- **All three conditions soft.** Never blocks emit but defeats the guard's purpose — silent data loss remains possible. Rejected.

## Consequences

- Positive: data-integrity violations (drops, count skew) block the warp mutation by default; `schema_version` softness avoids brittle coupling to bd patch releases; the matrix applies in `--dry-run`, so the planner agent's dry-run gate catches drops before human approval.
- Negative: operators must explicitly pass `--allow-drop` for forward-compat (not config-scoped in this seam); `schema_version` softness means a genuinely breaking bd schema change is not blocked until a drop or count mismatch manifests.
- Neutral: exit 2 (data-integrity/system condition) is consistent with the engine's existing exit-code taxonomy (not a user-invocation error).
