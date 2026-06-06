<!-- markdownlint-disable MD013 -->
<!-- adr-render: source=bd:weft-roc; do not edit manually; use `/adr update weft-roc` -->

# Use release-please for release orchestration over cocogitto

**Date:** 2026-06-06
**Status:** Accepted
**Decision:** weft-roc
**Deciders:** Sean Brandt

## Context

Weft needs a release pipeline that bumps version files, maintains a changelog, creates a GitHub Release, and triggers GoReleaser — on a protected trunk (land via PR + human merge) without a bypass token. `cog.toml` was a stub referencing GoReleaser/CI that did not exist; cocogitto commits the version bump directly to the branch, which does not fit protected `main` without a privileged token. Four release-PR / version tools were evaluated against a single-package Go-binary + Claude-Code-plugin repo.

## Decision

Adopt release-please as the release orchestrator and remove `cog.toml`. release-please maintains an always-open release PR — bumping `CHANGELOG.md` and `plugin/.claude-plugin/plugin.json` (`$.version`, via the GenericJson updater) from conventional commits. Merging that PR (the human gate) cuts the bare `vX.Y.Z` tag and the GitHub Release; GoReleaser runs in the **same workflow run**, gated on release-please's `release_created` output, and uploads the binary. Only the default `GITHUB_TOKEN` is used.

## Rationale

- A release-PR workflow (bot maintains the PR, human merges) is the idiomatic way to land in-tree version bumps on a protected trunk without a bypass token — the merge is the protected-main write.
- cocogitto's direct-commit model cannot honor branch protection without a PAT/App token.
- release-please's GenericJson updater co-bumps `plugin.json` in the same PR, eliminating a separate manual plugin-version step.
- Running GoReleaser in the same workflow run sidesteps the "GITHUB_TOKEN-pushed tag does not trigger another workflow" limitation (no PAT needed for triggering).

## Alternatives Considered

- **cocogitto** (the stub): handles conventional-commit version computation, but commits directly to `main` (wrong model for protected trunk) and cannot emit a changelog body to a GitHub Release. Rejected.
- **knope** (Rust): supports a release-PR recipe, but mandates per-package tag prefixes and cannot emit the bare `vX.Y.Z` the Go root module + GoReleaser require. Disqualifying.
- **changesets** (TS): mature release-PR pattern, but JS-native — needs a synthetic `package.json` per Go module and a JS toolchain in the release path. Rejected.
- **monorel** (Go-native, 2026): changesets-style for Go, but monorepo-focused, early-stage, single-author — overkill and bus-factor risk for a single-package repo. Rejected.

## Consequences

**Positive:** merging the release PR is the only required human action (no bypass token, no direct push to `main`); `CHANGELOG.md` is maintained automatically; `plugin.json` version is bumped atomically with the binary version in one PR. **Negative:** `cog.toml` and its conventions are removed (`CLAUDE.md` and `docs/design.md` updated to match); `CHANGELOG.md` becomes a committed file on `main` (cog.toml's old stance was no in-repo changelog). **Neutral:** the conventional-commit discipline is unchanged — only the parsing tool changes; CI still lint-checks the PR title as the squashed subject release-please parses.
