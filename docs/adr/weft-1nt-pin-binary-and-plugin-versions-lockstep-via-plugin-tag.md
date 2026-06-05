<!-- markdownlint-disable MD013 -->
<!-- adr-render: source=bd:weft-1nt; do not edit manually; use `/adr update weft-1nt` -->

# Pin binary and plugin versions in lockstep via the plugin tag

**Date:** 2026-06-05
**Status:** Accepted
**Decision:** weft-1nt
**Deciders:** Sean Brandt

## Context

The weft binary ships engine verbs; the weft plugin ships the prompts that invoke those verbs. If these drift — e.g. a prompt references a flag the installed binary does not yet have — the user gets a silent runtime mismatch, not a build error. Claude Code's `claude plugin tag` cuts a `{name}--v{version}` git tag, distinct from the Go binary's cocogitto `vX.Y.Z` tag. The binary's version is `cli.Version` (the dev sentinel `0.0.0-dev` until a release sets it).

## Decision

Each release cuts two tags at the same version `X.Y.Z`: `vX.Y.Z` (cocogitto, binary) and `weft--vX.Y.Z` (`claude plugin tag`, plugin), with `plugin.json.version` bumped in the same release commit. `weft install` pins the marketplace source ref to `weft--v<binary-version>`. Dev/untagged builds (`0.0.0-dev` / suffixed) error with guidance to pass `--ref` or `--local` rather than silently floating to a branch.

## Rationale

- The prompt↔verb contract is semantic: a prompt can reference a `--flag` added in v1.3 that does not exist in v1.2; a version mismatch is a silent runtime failure, not a build error.
- Dev builds must not silently install from an unpinned ref — the error-with-guidance makes the constraint visible and actionable.
- `claude plugin tag` validates that `plugin.json` and the marketplace entry agree, as a side effect of the release.

## Alternatives Considered

- **Always install from `main` (no pin).** No release coordination, always-latest prompts — but `main` may carry prompts referencing unreleased verb flags, breaking a user on an older binary silently. Rejected.
- **Embed plugin bytes in the binary (`go:embed`).** Atomic co-distribution of binary + prompts — but requires a transform engine, prevents independent plugin updates, and is not a native Claude Code install. Rejected (see the plugin-distribution ADR).

## Consequences

- **Positive:** installed prompts are always authored against the running binary (no silent prompt/verb flag drift); the update story is deterministic — install the newer binary, run `weft install`, get the matched prompt version.
- **Negative:** release requires two coordinated tags, and `plugin.json.version` must be bumped in the same release commit as the Go module version; dev/CI installs must always pass `--ref` or `--local` (a bare `weft install` is not usable in pre-release workflows).
- **Neutral:** Claude Code's installed-plugin registry is the source of truth for what is installed; weft carries no hand-rolled install manifest.
