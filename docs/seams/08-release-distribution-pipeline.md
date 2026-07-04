<!--
  ~ SPDX-License-Identifier: Apache-2.0
  ~ Copyright 2026 Weft Contributors
-->

# Seam 8 — Release & distribution pipeline

> Status: **shipped**. Sub-spec of
> [`docs/design.md`](../design.md) §8 (repo conventions; the deferred
> Taskfile / `.golangci.yaml` / `.goreleaser.yaml`) and the release follow-up
> noted in [seam 7](07-weft-install.md) §4.2 / §9. Tracked as bead
> `weft-hjx.11` (child of `weft-hjx`). **Supersedes the *mechanism* of ADR
> `weft-1nt`** (binary↔plugin version lockstep) while preserving its goal — see
> §9. The release pipeline is live: release-please maintains the release PR and
> merging it cuts the `vX.Y.Z` tag; GoReleaser builds the binary; strict plugin
> validation runs in CI (`.github/workflows/`, `.goreleaser.yml` shipped).

## 1. Scope

Make a released `weft` build installable. Seam 7 shipped `weft install`, which
pins the plugin marketplace to a release tag — but **no release process exists
to produce that tag**, so the marquee install feature only works against a
released build once this seam lands. This seam builds the whole release /
distribution pipeline:

- **Versioning** — one source of truth (the git tag), surfaced into the binary
  and the plugin manifest without a hand-maintained version literal (§3).
- **Release orchestration** — [release-please](https://github.com/googleapis/release-please)
  maintains a release PR; merging it (the human gate, on protected `main`,
  with **no privileged token**) cuts the tag + GitHub Release; GoReleaser builds
  and uploads the binary (§4).
- **CI gate** — a PR/push workflow: `go` test/build, the plugin's strict
  validation gates, and conventional-commit linting (§5).
- **Two small code/manifest touches** — `cli.Version` becomes ldflags- and
  build-info-derived (§3.2); `weft install`'s pin changes from the seam-7
  `weft--v<version>` plugin tag to the binary's own `v<version>` tag (§6).

cocogitto (`cog.toml`) is **removed**, not wired: release-please subsumes its
conventional-commit version computation and adds the release-PR, changelog,
file-bump, and GitHub-Release steps cog does not do (§4.4, §9).

**Out of scope:** cutting the actual first release (`v0.1.0`) is a
human-triggered act (merge the release PR), not part of this seam's
deliverable; `golangci-lint` config is deferred (§11).

## 2. Grounding

All traces are recorded as `bd note` lines on `weft-hjx.11`. Summary of what was
confirmed (vs. assumed) during brainstorming:

- **`claude plugin tag` / plugin versioning (claude-code-guide, then verified
  locally against `claude` 2.1.167):**
  - `claude plugin tag` creates a **git tag only** (`{name}--v{version}`), reads
    `plugin.json.version` from the committed clean tree, validates
    plugin.json↔marketplace agreement, is auth-free and headless-safe.
  - The `plugin.json` `version` field is **optional** *for install* (omitted →
    commit SHA is the implicit version; static → no auto-update) — **but**
    `claude plugin validate --strict` **fails** without it. Verified locally:
    stripping `version` produces `⚠ version: No version specified … ✘
    Validation failed (--strict treats warnings as errors)` on **both** the
    plugin-path and marketplace-path gates. So a real `version` literal must
    exist in `plugin.json`; it cannot be omitted, and a frozen sentinel is a
    lie. → release-please writes the real version (§4.2).
  - Marketplace `@<ref>` is confirmed for **branch and tag** refs; full-SHA refs
    are **not documented** — so the pin uses a **tag** (§6), not a SHA.
  - **CI install of the `claude` CLI (exa, corrected):** use
    `npm install -g @anthropic-ai/claude-code` (same native binary, Node 18+).
    The `curl …claude.ai/install.sh | bash` script returns **HTTP 403 in CI**
    behind a Cloudflare challenge (anthropics/claude-code#36306, 2026-03); it
    stays the local/dev method only (§5).
- **GoReleaser (context7 `/goreleaser/goreleaser`):** `goreleaser-action@v7`,
  `version: "~> v2"`, `on: push: tags`, ldflags template
  `-s -w -X <pkg>.Version={{.Version}}`, `permissions: contents: write` +
  `GITHUB_TOKEN`, `gomod.proxy` + `-trimpath` for reproducibility.
- **release-please (exa + `googleapis/release-please` docs):** the canonical
  release-PR tool; maintains an always-open PR, kept current; merge (squash or
  merge commit both work) → bumps files, tags, creates the GitHub Release.
  `release-type: go` = `CHANGELOG.md` + a **bare `vX.Y.Z`** tag (what `go
  install` and GoReleaser require). The **GenericJson** updater bumps an
  arbitrary JSON field (`{type: json, path, jsonpath: "$.version"}`) — confirmed
  it can bump `plugin/.claude-plugin/plugin.json`. release-please's well-known
  frictions (squash strips `Release-As:`/`BREAKING CHANGE:` footers;
  full-history footer leaks onto *new* packages; per-path attribution) are
  **monorepo** frictions; weft is a single package, so they do not apply.
- **Tooling survey (exa):** knope mandates per-package tag prefixes → cannot emit
  the bare `vX.Y.Z` the Go root module needs (disqualifying); changesets is
  JS-native (synthetic `package.json` per Go module); monorel is Go-native but
  monorepo-focused and early-stage/single-author (overkill + bus-factor risk for
  a single-package repo); cocogitto is not a release-PR tool (commits directly).
  → release-please.

## 3. Version model — one truth, no in-tree literal for the binary

### 3.1 The single source of truth is the git tag

A committed `VERSION` file (or any hand-maintained version constant) is a
*second* truth that drifts from `git describe`. There is exactly one version
truth: the **`vX.Y.Z` git tag**, computed by release-please from conventional
commits. Everything else either derives from it at build time or is written to
equal it by the release tool.

### 3.2 Binary version — `cli.Version` (derived)

`internal/cli/version.go` changes `const Version` → `var Version`, resolved once
at startup with this precedence:

1. **ldflags** — GoReleaser sets `-X github.com/seanb4t/weft/internal/cli.Version={{.Version}}`
   (the tag **without** the `v`, e.g. `0.1.0`). Release builds.
2. **`debug.ReadBuildInfo().Main.Version`** — populated when built via
   `go install github.com/seanb4t/weft@vX.Y.Z` (yields `v0.1.0` **with** the
   `v`). The resolver **strips a leading `v`** so the value is clean `0.1.0`.
3. **dev fallback** — local `go build`/`go run`: `Main.Version` is `(devel)`;
   the resolver surfaces a clearly-dev string (`(devel)` or, if VCS info is
   present, `0.0.0-dev+<shortsha>`). Never a clean semver.

The contract: **`cli.Version` is a clean `X.Y.Z` for any released build (both
GoReleaser and `go install`) and a non-semver dev string otherwise.** This is
what `weft install`'s existing `semverPattern` dev-detection relies on (§6), so
that gate needs no change beyond the leading-`v` normalization living in
`version.go`. `weft --version` reports `cli.Version` unchanged.

### 3.3 Plugin version — `plugin.json.version` (tool-written, real)

`plugin/.claude-plugin/plugin.json` `version` is the **only** in-tree version
literal, and `--strict` requires it (§2). It is **not** hand-maintained and
**not** a frozen sentinel: release-please's GenericJson updater rewrites
`$.version` to the release version **in the release PR**, so it always equals
the tag the same PR cuts. Between releases it sits at the last released version
(a real, honest value — `main` is prod and carries the version it last shipped).
`marketplace.json` carries **no** per-plugin version (plugin.json wins per ADR
`weft-1nt`), so there is no second field to bump and no skew.

## 4. Release orchestration

### 4.1 The flow

1. **release-please** (GitHub Action) runs on push to `main`. It keeps an
   always-open release PR up to date: bumps `CHANGELOG.md` and
   `plugin/.claude-plugin/plugin.json` (`$.version`) from the conventional
   commits since the last release.
2. **A human (you) merges the release PR.** This is the only human gate, it is
   the protected-`main` write (done by your merge, not a bot pushing to `main`),
   and it needs **no bypass token**.
3. On merge, release-please cuts the bare **`vX.Y.Z`** tag and creates the
   GitHub Release with the changelog body.
4. In the **same workflow run**, gated on release-please's `release_created`
   output, **GoReleaser** checks out the tag, builds the binary with
   `-X …/cli.Version={{.Version}}`, and uploads the archives/checksums to the
   release. Running in the same run sidesteps the "`GITHUB_TOKEN`-pushed tag does
   not trigger another workflow" gotcha — no PAT needed for triggering.

`main` always reflects the released version; dev/untagged builds report a dev
string and `weft install` refuses to float (preserving ADR `weft-1nt`'s goal,
§9).

### 4.2 `release.yml`

`on: push: branches: [main]`. `permissions: { contents: write,
pull-requests: write }`. Default `GITHUB_TOKEN` only.

```yaml
jobs:
  release:
    runs-on: ubuntu-latest
    permissions: { contents: write, pull-requests: write }
    steps:
      - uses: googleapis/release-please-action@v5   # pin to a vetted SHA in impl
        id: release
        with: { config-file: release-please-config.json, manifest-file: .release-please-manifest.json }
      # binary build only when a release was just created — check out AT THE TAG
      # release-please just cut (not the triggering main HEAD), so GoReleaser
      # builds the exact released commit.
      - if: ${{ steps.release.outputs.release_created }}
        uses: actions/checkout@v6
        with: { ref: "${{ steps.release.outputs.tag_name }}", fetch-depth: 0 }
      - if: ${{ steps.release.outputs.release_created }}
        uses: actions/setup-go@v6
        with: { go-version-file: go.mod }
      - if: ${{ steps.release.outputs.release_created }}
        uses: goreleaser/goreleaser-action@v7
        with: { distribution: goreleaser, version: "~> v2", args: release --clean }
        env: { GITHUB_TOKEN: "${{ secrets.GITHUB_TOKEN }}" }
```

release-please **owns** the GitHub Release object (its changelog body is the
point); GoReleaser only **uploads binaries to the already-created, tag-keyed
release**. GoReleaser v2 (FOSS) has **no `release.mode` field** (an earlier draft
of this spec wrongly used `mode: append` — corrected here after checking current
docs). The verified `release:` knobs are `replace_existing_artifacts: true` (for
idempotent re-runs) and, optionally, the draft path `draft: true` +
`use_existing_draft: true` (v2.5+). The exact body-vs-artifact ownership wiring
(ensuring GoReleaser does not clobber release-please's notes) is pinned at impl
against the current GoReleaser release docs (§4.4).

Action major versions are **current as of 2026-06** (`checkout@v6`,
`setup-go@v6`, `goreleaser-action@v7` driving GoReleaser `~> v2`,
`release-please-action@v5` — v5.0.0 shipped 2026-04 as a Node-24 runtime bump;
its `config-file`/`manifest-file` inputs and `release_created`/`tag_name`
outputs are unchanged from v4); each is pinned to a vetted commit SHA and
re-verified against the marketplace at implementation time per the
GitHub-Actions-version rule.

### 4.3 release-please config

- `release-please-config.json`: one package at `.`, `release-type: "go"`,
  `extra-files: [{ type: "json", path: "plugin/.claude-plugin/plugin.json",
  jsonpath: "$.version" }]`. `bump-minor-pre-major` / `bump-patch-for-minor-pre-major`
  set so `0.y.z` increments stay pre-1.0 sane.
- `.release-please-manifest.json`: `{ ".": "0.0.0" }` initially (first release PR
  proposes `0.1.0` from the accumulated `feat:` history).
- `CHANGELOG.md` is created/maintained by release-please (a normal committed
  file on `main`; this is the one in-repo changelog — note `cog.toml`'s old
  "no in-repo CHANGELOG" stance is dropped along with cog).

### 4.4 `.goreleaser.yml`

Single Go binary `cmd/weft`. `builds`: `goos` darwin+linux, `goarch`
amd64+arm64, `CGO_ENABLED=0`, `flags: [-trimpath]`, `mod_timestamp:
{{.CommitTimestamp}}`, `ldflags: -s -w -X github.com/seanb4t/weft/internal/cli.Version={{.Version}}`.
`gomod.proxy: true`. `archives` + `checksum`. `release:
{ replace_existing_artifacts: true }` so GoReleaser uploads artifacts to the
release-please-created, tag-keyed Release idempotently rather than owning it
(§4.2; there is **no** `release.mode` field in GoReleaser v2 FOSS — the exact
keys that preserve release-please's notes are pinned at impl). SPDX header in a
leading comment.

### 4.5 `Taskfile.yml`

Thin developer ergonomics (not the release trigger — the trigger is merging the
release PR):

- `task build` / `task test` — local `go build` / `go test ./...`.
- `task validate` — `claude plugin validate ./plugin --strict` **and**
  `claude plugin validate . --strict` (both gates; the marketplace gate is the
  only one that catches the `owner{}` class of error — see project memory).
- `task release:preview` — `gh pr view` the open release PR (or note none open).
- `task snapshot` — `goreleaser release --snapshot --clean` for a local dry build.

## 5. CI gate — `ci.yml`

`on: pull_request` + `push: branches: [main]`. `permissions: { contents: read }`.

- `go build ./...` and `go test ./...` (matrix optional; single linux runner is
  enough for v1).
- `goreleaser check` (validates `.goreleaser.yml` — via `goreleaser-action@v7`
  with `args: check`, or `task` — so a config error fails the PR, not the
  release).
- `claude plugin validate ./plugin --strict` **and** `claude plugin validate .
  --strict` (auth-free for `validate`). **Install the `claude` CLI in CI via
  `npm install -g @anthropic-ai/claude-code`** (`setup-node`, Node 18+) — **not**
  the `curl …claude.ai/install.sh | bash` script, which returns HTTP 403 in CI
  behind a Cloudflare challenge (anthropics/claude-code#36306). The npm package
  installs the same native binary; the install-script remains the local/dev
  method.
- The seam-5/7 grep-discipline check (no stale `weft/…` intra-tree paths in the
  plugin tree; the `${CLAUDE_PLUGIN_ROOT}` contract — see project memory).
- **Conventional-commit lint** on the PR **title** (the squashed subject becomes
  the `main` commit release-please parses). A lightweight title-lint action; no
  local commit-msg hook (jj does not fire git hooks reliably — design.md §8).

`golangci-lint` is **deferred** (§11).

## 6. `weft install` pin change (touches seam 7)

`internal/install/install.go` `resolveSource` currently pins
`"weft--v" + version` (the seam-7 plugin-tag namespace). This seam changes it to
**`"v" + version`** — the binary's own release tag, the single tag namespace.
Rationale and consequences are the ADR work in §9 (supersede `weft-1nt`'s
mechanism). The marketplace add becomes `seanb4t/weft@vX.Y.Z`.

- `semverPattern` (`^[0-9]+\.[0-9]+\.[0-9]+$`) and the dev-refuses-to-float
  behavior are **unchanged** — the `version.go` normalization (§3.2) guarantees a
  released `cli.Version` is clean `X.Y.Z` on every build path, so a dev build
  still fails the pattern and is told to pass `--ref`/`--local`.
- The seam-7 install tests that assert `weft--v<version>` update to `v<version>`;
  the injection-guard and `--ref`/`--local` allowlist tests are unaffected.
- No `claude plugin tag` step anywhere in the release path; the plugin is pinned
  to the binary tag, validated in CI (§5), not at tag time.

## 7. Error handling & exit codes

No new engine exit-code surface (this seam is CI/config + two small touches).
The `weft install` contract from seam 7 (`CodeInvocation` for bad
`--scope`/`--ref`/dev-no-ref; `CodeHard` for `claude` absent / non-zero `claude
plugin` subprocess) is unchanged. CI failures (validate, test, lint) fail the
workflow loudly; release-please / GoReleaser non-zero exits fail the release
workflow (no swallowed errors — silent-failure discipline).

## 8. Files / deliverables

| Path | New? | Role |
| --- | --- | --- |
| `internal/cli/version.go` | edit | `const Version` → `var Version` (ldflags / build-info / dev, §3.2) |
| `internal/install/install.go` | edit | pin `v<version>` (was `weft--v<version>`, §6) |
| `internal/install/install_test.go` | edit | update pin assertions `weft--v` → `v` |
| `.goreleaser.yml` | new | binary build + ldflags + release (§4.4) |
| `.github/workflows/release.yml` | new | release-please + GoReleaser (§4.2) |
| `.github/workflows/ci.yml` | new | PR/push gate (§5) |
| `release-please-config.json` | new | release-type go + GenericJson plugin.json bump (§4.3) |
| `.release-please-manifest.json` | new | `{ ".": "0.0.0" }` (§4.3) |
| `CHANGELOG.md` | new | release-please-maintained |
| `Taskfile.yml` | new | dev ergonomics (§4.5) |
| `cog.toml` | **delete** | cocogitto removed (§4, §9) |
| `CLAUDE.md` | edit | Conventions block says "cocogitto (`cog.toml`) tag-only releases … validated in CI" — rewrite to release-please; drop the cocogitto reference |
| `docs/design.md` | edit | §8 names cocogitto + the deferred `.goreleaser.yaml`/Taskfile — update to record release-please as the chosen release tool and that `.goreleaser.yml`/`Taskfile.yml` now exist |
| `plugin/.claude-plugin/plugin.json` | unchanged on `main` | release-please bumps `$.version` in the release PR (§3.3) |

All new source + functional markdown/config carry SPDX headers (Apache-2.0).

## 9. Decisions to capture as ADRs

- **release-please over cocogitto for release orchestration.** A release-PR
  workflow (bot maintains the PR, human merges to release) is the idiomatic
  answer to in-tree version bumps on a protected trunk without a bypass token;
  cocogitto commits directly (wrong model) and knope/changesets/monorel each
  misfit a single-package Go + plugin repo (§2). `cog.toml` is removed.
- **Single `vX.Y.Z` tag; the plugin is pinned to the binary's release tag —
  supersedes the *mechanism* of ADR `weft-1nt`.** weft-1nt decided a separate
  `weft--vX.Y.Z` plugin tag cut by `claude plugin tag`, lockstepped to the
  binary. This seam keeps weft-1nt's **goal** (no silent prompt↔verb drift —
  strengthened: the pinned plugin tree is the exact commit the binary was built
  from) but drops the second tag namespace, `claude plugin tag`, and the
  separate plugin-version bump. `weft install` pins `@vX.Y.Z`. Filed as an ADR
  supersession of `weft-1nt`.
- **The git tag is the single version source of truth; no in-tree version
  literal for the binary.** `cli.Version` is ldflags-/build-info-derived; the
  one required in-tree literal (`plugin.json.version`, forced by `--strict`) is
  tool-written to equal the tag, never hand-maintained. (May fold into the
  supersession ADR.)

## 10. Testing / validation

- **`weft install` unit tests** (existing fake `run.Runner`): pin assertions
  flip `weft--v<version>` → `v<version>`; dev-version-refuses and
  `--ref`/`--local` allowlist tests unchanged (assert via the envelope-field
  convention, not whole-output `strings.Contains`).
- **`cli.Version` resolution:** a unit test for the leading-`v` strip
  (`v0.1.0` → `0.1.0`) and the dev fallback shape (non-semver) so the
  `semverPattern` contract holds across build paths.
- **`.goreleaser.yml`:** `goreleaser check` in CI; `task snapshot` builds a local
  binary and asserts `weft version --json` reports the **snapshot-decorated**
  version GoReleaser injects (e.g. `0.1.0-SNAPSHOT-<shortsha>`, **not** a clean
  semver) — and, correspondingly, that such a build is treated as dev by
  `semverPattern` so a snapshot `weft install` refuses to float (consistent with
  §3.2 / §6).
- **Plugin manifests:** `claude plugin validate` ×2 `--strict` (CI gate, §5).
- **release-please config:** validated by the action on first run; the release PR
  it opens is the integration proof (it proposes `0.1.0`, bumps `plugin.json` +
  `CHANGELOG.md`). No `go test` for the YAML/JSON config.
- No attempt to test "cut a real release" in CI (that is the human merge, §1).

## 11. Out of scope / deferred

- Cutting the first real release (`v0.1.0`) — human-merges the release PR.
- `golangci-lint` + `.golangci.yaml` (design.md §8 deferred item); add as a CI
  step in a later seam if wanted.
- Multi-platform / multi-arch beyond darwin+linux × amd64+arm64; signing,
  notarization, Homebrew tap, container images — none required for the lockstep
  and all addable to `.goreleaser.yml` later (YAGNI).
- Any second host runtime's release (Claude Code is the only host; seam 7).
<!-- adr-capture: sha256=8de63868f54105b9; session=cli; ts=2026-06-06T16:06:22Z; adrs=weft-roc,weft-1hf,weft-19y -->
