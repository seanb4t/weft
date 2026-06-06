<!-- markdownlint-disable MD013 -->
<!-- adr-render: source=bd:weft-19y; do not edit manually; use `/adr update weft-19y` -->

# Derive the binary version from the git tag (ldflags + build-info), not an in-tree literal

**Date:** 2026-06-06
**Status:** Accepted
**Decision:** weft-19y
**Deciders:** Sean Brandt

## Context

`cli.Version` was a `const` dev sentinel. A committed `VERSION` file or hand-maintained constant is a second source of truth that drifts from the git tag (`git describe`). The binary must report a clean `X.Y.Z` for released builds whether built by GoReleaser (ldflags) or `go install` (module build-info), and a clearly-non-semver string for dev builds so `weft install`'s `semverPattern` gate behaves correctly on every build path.

## Decision

Change `internal/cli.Version` from a `const` to a `var` resolved at startup by a precedence chain: (1) ldflags value set by GoReleaser (`-X …cli.Version={{.Version}}`, the tag without its leading `v`); (2) `debug.ReadBuildInfo().Main.Version` for `go install …@vX.Y.Z` builds, with the leading `v` stripped (skipping the `(devel)` placeholder); (3) a `0.0.0-dev` fallback. The git tag is the single version source of truth; the only permitted in-tree version literal is `plugin.json.version`, which is tool-written by release-please.

## Rationale

- The git tag is the single version truth; any in-tree literal for the binary creates drift risk between releases.
- Both GoReleaser and `go install` are first-class release paths; the resolution chain yields a clean `X.Y.Z` on both without special-casing at call sites.
- The non-semver dev fallback preserves `weft install`'s `semverPattern` guard with no change to the install logic.
- `plugin.json.version` is forced by `claude plugin validate --strict` and cannot be omitted; tool-writing it via release-please is the only safe approach.

## Alternatives Considered

- **Committed `VERSION` file:** simple and human-readable, but a second source of truth that drifts from the tag and needs a separate bump step. Rejected.
- **Hard-coded `const` (status quo):** zero cost, but must be hand-bumped each release; a forgotten bump is a silent lie, and there is no `go install` path support. Rejected.

## Consequences

**Positive:** a released binary always reports the correct version regardless of build method; no manual source bump per release; the `weft install` dev-guard works across all build paths unchanged. **Negative:** `version.go` needs a unit test for the leading-`v` strip and dev-fallback shape; a local `go build` without ldflags surfaces a dev string (expected, documented). **Neutral:** `weft --version` output is unchanged; `plugin.json.version` remains the one in-tree literal, written by tooling not humans.
