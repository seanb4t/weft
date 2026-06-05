<!-- markdownlint-disable MD013 -->
<!-- adr-render: source=bd:weft-tzn; do not edit manually; use `/adr update weft-tzn` -->

# Add run.Claude as the engine's fourth wrapped external CLI

**Date:** 2026-06-05
**Status:** Accepted
**Decision:** weft-tzn
**Deciders:** Sean Brandt

## Context

The weft engine wraps external CLIs (`jj`, `bd`, `gh`) via the injectable `run.Runner` interface (ADR `weft-re2`). ADR `weft-yuj` established `gh` as the third wrapped CLI for the finish verbs. The `weft install` verb needs to drive Claude Code's own `claude plugin` CLI to register a marketplace and install the plugin. `design.md` §7 prohibits the engine from dispatching agents — the engine is the deterministic layer.

## Decision

`internal/run` gains `Claude(r Runner, args ...string) (Result, error)`, mirroring `JJ`/`BD`/`GH`, wrapping the `claude` binary — the engine's fourth wrapped external CLI. `internal/install` depends only on `run.Runner`, so it is unit-testable with the existing fake runner.

## Rationale

- Extends ADR `weft-yuj`: the same rationale applies — deterministic CLI wrapping over agent dispatch or a library dependency.
- The existing `fakeRunner` in `run_test.go` mocks `claude` identically to `jj`/`bd`/`gh` with zero new test infrastructure.
- `design.md` §7 prohibits agent dispatch from the engine; shelling `claude` as a CLI is the only compliant path.

## Alternatives Considered

- **Agent dispatch (invoke a Claude Code agent sub-call).** Zero engine code — but explicitly prohibited by `design.md` §7, and nondeterministic / untestable. Rejected.
- **Direct filesystem plugin installation (copy files).** No dependency on the `claude` CLI, works offline — but bypasses Claude Code's install registry (no update/uninstall lifecycle), requires knowing and maintaining the cache path (`~/.claude/plugins/cache/…`), and is brittle against Claude Code version changes. Rejected.

## Consequences

- **Positive:** all `claude plugin` subcommand calls are unit-testable via the existing fake (arg-construction bugs caught at unit level); pattern-consistent, so onboarding cost is near-zero given the `run.GH` precedent.
- **Negative:** `claude` must be installed and discoverable on PATH; the preflight (`run.Claude` probe) fails clearly but is itself a subprocess call.
- **Neutral:** the `run.Runner` interface contract does not change (`weft-re2` stands).
