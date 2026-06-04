<!-- markdownlint-disable MD013 -->
<!-- adr-render: source=bd:weft-lc7; do not edit manually; use `/adr update weft-lc7` -->

# Address non-default jj workspaces via <name>@ revset, not run.Runner cwd

**Date:** 2026-06-04
**Status:** Accepted
**Decision:** weft-lc7
**Deciders:** Sean Brandt

## Context

The seam-4 conflict verbs must run jj commands (squash, diff, log) against the resolution workspace's working-copy commit from within the default workspace where the weft engine runs. Two structural options exist: extend run.Runner (the injectable subprocess seam, ADR weft-re2) with a cwd/dir field so commands execute from the target workspace directory, or use jj's built-in <workspace-name>@ revset syntax to address any workspace's working copy as a revision from anywhere in the repo.

## Decision

The engine uses jj's <workspace-name>@ revset syntax (e.g. wsRev := name + "@") to reference any workspace's working-copy commit from the default workspace, leaving run.Runner a single-method, zero-field interface with no cwd support. conflict finalize targets the resolution workspace as `jj diff --git -r <name>@` and `jj squash --from <name>@ --into <change>`.

## Rationale

Preserves the single-method run.Runner contract from ADR weft-re2 — no ripple changes to seam-1/3 verb constructors, tests, or the routeRunner fake (cwd would require fake-filesystem plumbing). jj workspace-addressing is symmetrical and stable: any workspace's @ is addressable from any other workspace in the same repo (verified live: jj log -r 'default@' resolves). The cwd alternative adds interface complexity for a generality (arbitrary subprocess redirection) no current or planned seam needs.

## Alternatives Considered

Extend run.Runner with cwd support: general-purpose (any subprocess directed to any directory, familiar Unix model) but grows the interface, forces every cwd-agnostic verb to pass a zero value, and couples the seam to an OS concept rather than a jj concept. Rejected in favor of <name>@ revset addressing.

## Consequences

POSITIVE: run.Runner stays minimal and stable (no migration for existing verbs); conflict verbs remain unit-testable with the recording fake; the <name>@ convention is reusable for any future workspace-scoped verb. NEGATIVE: a future verb needing to run a NON-jj subprocess from within a workspace dir (e.g. a workspace-scoped build/verify) cannot use this pattern and would require revisiting the Runner interface. NEUTRAL: verb logic constructs the <name>@ string explicitly, a minor visible coupling to jj revset grammar.
