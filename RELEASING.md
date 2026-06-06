<!--
  ~ SPDX-License-Identifier: Apache-2.0
  ~ Copyright 2026 Weft Contributors
-->

# Releasing weft

Weft releases are driven by [release-please](https://github.com/googleapis/release-please)
and [GoReleaser](https://goreleaser.com), wired in `.github/workflows/release.yml`.
A single `vX.Y.Z` tag versions both the binary and the plugin
(ADR `weft-1hf`); `weft install` pins `seanb4t/weft@vX.Y.Z`.

## Prerequisite (one-time repo setting)

release-please opens the release PR using `GITHUB_TOKEN`. This requires the repo
setting **Settings → Actions → General → Workflow permissions → "Allow GitHub
Actions to create and approve pull requests"** to be **enabled**. The workflow's
own `permissions: pull-requests: write` is *not* sufficient on its own — the
repo-level toggle overrides it.

If it is off, the `release` workflow fails with:

```
release-please failed: GitHub Actions is not permitted to create or approve pull requests.
```

(API check / fix: `gh api repos/{owner}/{repo}/actions/permissions/workflow`
→ `can_approve_pull_request_reviews` must be `true`.)

## How a release works

1. Every push to `main` runs the `release` workflow → `release-please` maintains
   an always-open **release PR** that bumps `CHANGELOG.md` and
   `plugin/.claude-plugin/plugin.json` (`$.version`) from the conventional
   commits since the last release.
2. **Merging the release PR** is the release: release-please cuts the bare
   `vX.Y.Z` tag + GitHub Release.
3. In the same workflow run, **GoReleaser** builds the binary (injecting
   `cli.Version` via `-ldflags`) and uploads it to that release.

Merging the release PR is the only human action, and it is the protected-`main`
gate — no privileged token, no direct push to `main`.

## First / explicit version

release-please defaults the **first** release to `1.0.0` when the manifest
starts at `0.0.0` — the `bump-*-pre-major` flags are not applied to the
bootstrap release ([release-please#2087](https://github.com/googleapis/release-please/issues/2087)).
To choose a different first (or any) version, land a commit on `main` whose body
carries a `Release-As:` footer, e.g.:

```
chore(release): start versioning at 0.1.0

Release-As: 0.1.0
```

release-please then re-cuts the open release PR at that version. (Use the
`Release-As:` footer — *not* `initial-version`, which sets the *last-released*
baseline. Ensure the footer survives squash-merge: keep it in the squashed
commit body.)

## Local checks

`task validate` (plugin strict gates), `task check` (`goreleaser check`),
`task snapshot` (local `goreleaser` dry build). CI (`.github/workflows/ci.yml`)
runs `go test`/`build`, `goreleaser check`, both `claude plugin validate
--strict` gates, the `${CLAUDE_PLUGIN_ROOT}` grep-discipline, and `go mod tidy`
drift detection on every PR.
