<!-- markdownlint-disable MD013 -->
<!-- adr-render: source=bd:weft-yuj; do not edit manually; use `/adr update weft-yuj` -->

# Wrap gh as the engine's third external CLI for finish verbs

**Date:** 2026-06-05
**Status:** Accepted
**Decision:** weft-yuj
**Deciders:** Sean

## Context

The weft engine is a Go binary that wraps external CLIs (`bd`, `jj`) via the injectable `run.Runner` interface. The `finish` verbs need to push bookmarks to GitHub and open or inspect PRs. GSD's `gsd-ship` is a deterministic shell script that calls `gh` directly rather than delegating to an agent. design.md §7 prohibits the engine from dispatching agents; the engine is the deterministic layer equivalent to `gsd-ship`.

## Decision

The engine adds `run.GH`, mirroring `run.JJ` and `run.BD`, wrapping the `gh` CLI for all GitHub interactions in the finish verbs (`gh auth status`, `gh pr view`, `gh pr create`, and `gh api -X DELETE` for stale-branch cleanup). `gh` becomes the engine's third external CLI dependency.

## Rationale

- design.md §7 defines the engine as the deterministic layer equivalent to GSD's `gsd-ship` script, which calls `gh` directly — not via an agent.
- The existing `run.Runner` interface and its test fake mock `gh` identically to `jj` and `bd`, keeping all subprocess calls behind one testable seam.
- No new library dependencies are introduced; `gh` is already a required tool in the GSD-derived workflow.
- The REST-API alternative would require managing auth tokens explicitly and adds HTTP-client complexity that `gh` already handles.
- Agent delegation is architecturally prohibited (design.md §7) and is nondeterministic/untestable.

## Alternatives Considered

**Wrap `gh` as a third `run.Runner` CLI (`run.GH`) (chosen)** — consistent with the established `run.JJ`/`run.BD` pattern; fully mockable via the existing fake; faithfully translates GSD's gsd-ship deterministic-script model; no new library dependencies. Cost: a third external binary that must be installed + authenticated (`gh auth status` preflight).

**GitHub REST API via a Go HTTP client** — rejected. No `gh` binary dependency, but requires separate token management, adds an HTTP-client dependency and auth/serialization/error-handling code, and diverges from both the GSD model and the project's CLI-wrapping pattern.

**Delegate PR creation to an agent sub-call** — rejected. Zero engine code for GitHub, but explicitly prohibited by design.md §7 and is nondeterministic/untestable.

## Consequences

**Positive:** `gh` calls are unit-testable with the existing fake (arg-construction bugs caught at the unit level); consistent with the `run.JJ`/`run.BD` pattern; no new runtime library dependencies.

**Negative:** `gh` must be installed and authenticated wherever `finish` runs (preflight fails clearly but adds a subprocess call); `gh api -X DELETE` for remote-branch cleanup is a workaround for `gh pr merge --delete-branch` unreliability (PR #18 evidence) — fragility inherited from the CLI.

**Neutral:** `run.GH` is the only addition to `internal/run/run.go`; the `Runner` interface contract itself does not change.
