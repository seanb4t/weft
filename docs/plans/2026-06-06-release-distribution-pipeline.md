<!--
  ~ SPDX-License-Identifier: Apache-2.0
  ~ Copyright 2026 Weft Contributors
-->

# Release & Distribution Pipeline Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use dev-flow:subagent-driven-development (recommended) or dev-flow:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make a released `weft` build installable — produce the `vX.Y.Z` tag + GitHub Release that `weft install` pins, with the binary and plugin version-locked, on a protected trunk with no privileged token.

**Architecture:** [release-please](https://github.com/googleapis/release-please) maintains a release PR (bumps `CHANGELOG.md` + `plugin/.claude-plugin/plugin.json` `$.version` from conventional commits); merging it (the human gate) cuts a bare `vX.Y.Z` tag + GitHub Release; in the same workflow run GoReleaser builds the binary (`cli.Version` injected via ldflags) and uploads it. The single `vX.Y.Z` tag is the sole version truth and pins both artifacts; `weft install` pins `seanb4t/weft@v<version>`. cocogitto is removed.

**Tech Stack:** Go 1.26, GitHub Actions, release-please-action@v5, GoReleaser v2 (goreleaser-action@v7), go-task (Taskfile), `claude` CLI (plugin validation).

**Spec:** `docs/seams/08-release-distribution-pipeline.md`. **Design bead:** `weft-hjx.11`.

---

## File Structure

| Path | Action | Responsibility |
| --- | --- | --- |
| `internal/cli/version.go` | modify | `const Version` → `var Version`, ldflags/build-info-derived |
| `internal/cli/version_test.go` | create | unit-test the version resolution logic |
| `internal/install/install.go` | modify | pin `v<version>` (was `weft--v<version>`) |
| `internal/install/install_test.go` | modify | flip pin assertions to `v<version>` |
| `.goreleaser.yml` | create | binary build + ldflags + release upload |
| `release-please-config.json` | create | release-type `go` + GenericJson plugin.json bump |
| `.release-please-manifest.json` | create | tracked version (`{ ".": "0.0.0" }`) |
| `CHANGELOG.md` | create | release-please-maintained changelog seed |
| `.github/workflows/release.yml` | create | release-please + GoReleaser (release path) |
| `.github/workflows/ci.yml` | create | PR/push gate (test, build, validate, lint) |
| `Taskfile.yml` | create | dev ergonomics (build/test/validate/snapshot) |
| `cog.toml` | delete | cocogitto removed |
| `CLAUDE.md` | modify | Conventions: cocogitto → release-please |
| `docs/design.md` | modify | §8: cocogitto + deferred tooling → release-please |

Tasks are ordered so each produces a self-contained, independently-sensible change. Tasks 1–2 are TDD Go code; Tasks 3–8 are config/docs verified with `goreleaser check`, `actionlint`, `jq`, `task --list`, `claude plugin validate`, and `go build`.

---

### Task 1: `cli.Version` — ldflags-injectable with build-info fallback

**Files:**

- Modify: `internal/cli/version.go`
- Test: `internal/cli/version_test.go` (create)

- [ ] **Step 1: Write the failing test**

Create `internal/cli/version_test.go`:

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Weft Contributors

package cli

import (
	"runtime/debug"
	"testing"
)

func TestResolveVersion(t *testing.T) {
	bi := func(v string) func() (*debug.BuildInfo, bool) {
		return func() (*debug.BuildInfo, bool) {
			if v == "" {
				return nil, false
			}
			return &debug.BuildInfo{Main: debug.Module{Version: v}}, true
		}
	}
	tests := []struct {
		name, ldflags, buildInfo, want string
	}{
		{"ldflags clean", "0.1.0", "", "0.1.0"},
		{"ldflags v-prefixed", "v0.1.0", "", "0.1.0"},
		{"go install module version", "", "v0.2.0", "0.2.0"},
		{"local devel placeholder", "", "(devel)", "0.0.0-dev"},
		{"no build info", "", "", "0.0.0-dev"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := resolveVersion(tt.ldflags, bi(tt.buildInfo)); got != tt.want {
				t.Errorf("resolveVersion(%q, bi=%q) = %q, want %q", tt.ldflags, tt.buildInfo, got, tt.want)
			}
		})
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/cli/ -run TestResolveVersion -v`
Expected: FAIL — `undefined: resolveVersion`.

- [ ] **Step 3: Write the minimal implementation**

Replace the entire contents of `internal/cli/version.go` with:

```go
// internal/cli/version.go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Weft Contributors

package cli

import (
	"runtime/debug"
	"strings"

	"github.com/spf13/cobra"
)

// Version is the engine version. Release builds set it via
//   -ldflags "-X github.com/seanb4t/weft/internal/cli.Version=<X.Y.Z>"
// (GoReleaser injects the tag without its leading "v"). When unset — a
// `go install …@vX.Y.Z` or a local `go build` — it is derived from the module
// build info. The result is a clean "X.Y.Z" for any released build and the
// "0.0.0-dev" sentinel otherwise, so internal/install.semverPattern correctly
// refuses to pin a release tag for dev builds.
var Version string

func init() { Version = resolveVersion(Version, debug.ReadBuildInfo) }

// resolveVersion picks the version: an explicit ldflags value wins (leading "v"
// stripped); else the module version from build info (skipping the "(devel)"
// placeholder); else the dev sentinel.
func resolveVersion(ldflagsVal string, readBuildInfo func() (*debug.BuildInfo, bool)) string {
	if ldflagsVal != "" {
		return strings.TrimPrefix(ldflagsVal, "v")
	}
	if bi, ok := readBuildInfo(); ok && bi.Main.Version != "" && bi.Main.Version != "(devel)" {
		return strings.TrimPrefix(bi.Main.Version, "v")
	}
	return "0.0.0-dev"
}

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the weft version",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return Emit(cmd, "version", map[string]string{"version": Version}, "weft "+Version)
		},
	}
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/cli/ -run TestResolveVersion -v && go build ./...`
Expected: PASS; build succeeds. (`go build` with no ldflags yields `Version == "0.0.0-dev"` via the build-info fallback — verify with `go run ./cmd/weft version`, which prints `weft 0.0.0-dev`.)

- [ ] **Step 5: Commit**

`jj commit -m "feat(weft-hjx.11): make cli.Version ldflags-injectable with build-info fallback"`

---

### Task 2: `weft install` pins the binary's `vX.Y.Z` release tag

**Files:**

- Modify: `internal/install/install.go:91` (and comments at `:70`, `:75-77`, `:96`)
- Test: `internal/install/install_test.go` (lines 41–48, 114, 192, 243, 299)

- [ ] **Step 1: Update the tests to expect the new pin (red)**

In `internal/install/install_test.go`, rename and rewrite the default-pin test:

```go
func TestResolveSourceDefaultPinsReleaseTag(t *testing.T) {
	src, ref, err := resolveSource("1.4.0", "", "")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if src != "seanb4t/weft" || ref != "v1.4.0" {
		t.Errorf("default must pin seanb4t/weft@v1.4.0, got %q@%q", src, ref)
	}
}
```

Then replace the marketplace-add string in the four other assertions (lines ~114, ~192, ~243, ~299) — change `seanb4t/weft@weft--v1.4.0` to `seanb4t/weft@v1.4.0` in each. Concretely:

- Line ~114: `if strings.Contains(j, "plugin marketplace add seanb4t/weft@v1.4.0") {`
- Line ~192: `const addArg = "seanb4t/weft@v1.4.0"`
- Line ~243: `const addArg = "seanb4t/weft@v1.4.0"`
- Line ~299: `{"seanb4t/weft@v1.4.0", "seanb4t/weft@v1.4.0"}, // clean → unquoted`

**Do NOT change line ~29** — `"weft--v1.2.3"` there is a `--ref` *allowlist input* fixture (an arbitrary ref a user may pass via `--ref`); it must still be accepted, so it stays.

- [ ] **Step 2: Run the install tests to verify they fail**

Run: `go test ./internal/install/ -v`
Expected: FAIL — assertions expect `@v1.4.0` but `resolveSource` still returns `weft--v1.4.0`.

- [ ] **Step 3: Change the pin in the implementation**

In `internal/install/install.go`, change the default-pin return (line ~91):

```go
	return repoSlug, "v" + version, nil
```

(was `return repoSlug, "weft--v" + version, nil`). Update the surrounding comments that say "plugin tag `weft--v<version>`" to read "the binary's release tag `v<version>`" — specifically the doc comment block at lines ~74–77 and the `Version` field comment at line ~96 (`// the binary's version (cli.Version); pins v<Version>`).

- [ ] **Step 4: Run the install tests to verify they pass**

Run: `go test ./internal/install/ -v && go build ./...`
Expected: PASS; build succeeds. The dev-refuses test (`TestResolveSourceDevVersionRefuses`, unchanged) still passes because `0.0.0-dev` fails `semverPattern`.

- [ ] **Step 5: Commit**

`jj commit -m "feat(weft-hjx.11): pin weft install to the binary's vX.Y.Z release tag"`

---

### Task 3: `.goreleaser.yml`

**Files:**

- Create: `.goreleaser.yml`

- [ ] **Step 1: Write the config**

Create `.goreleaser.yml`:

```yaml
# SPDX-License-Identifier: Apache-2.0
# Copyright 2026 Weft Contributors
version: 2

before:
  hooks:
    - go mod tidy

builds:
  - id: weft
    main: ./cmd/weft
    binary: weft
    env:
      - CGO_ENABLED=0
    goos: [linux, darwin]
    goarch: [amd64, arm64]
    flags:
      - -trimpath
    mod_timestamp: "{{ .CommitTimestamp }}"
    ldflags:
      - -s -w -X github.com/seanb4t/weft/internal/cli.Version={{ .Version }}

gomod:
  proxy: true

archives:
  - formats: [tar.gz]
    name_template: "{{ .ProjectName }}_{{ .Version }}_{{ .Os }}_{{ .Arch }}"

checksum:
  name_template: "checksums.txt"

release:
  # release-please creates the GitHub Release (changelog body); GoReleaser only
  # uploads artifacts to that already-created, tag-keyed release. There is no
  # `release.mode` field in GoReleaser v2 FOSS; replace_existing_artifacts makes
  # re-runs idempotent.
  replace_existing_artifacts: true
```

- [ ] **Step 2: Validate the config**

Run: `goreleaser check`
Expected: `command finished successfully` (no schema errors). If `goreleaser` is not installed locally: `go install github.com/goreleaser/goreleaser/v2@latest` first, or run `task snapshot` after Task 7.

- [ ] **Step 3: Verify ldflags injection with a snapshot build**

Run: `goreleaser build --snapshot --clean --single-target -o /tmp/weft-snap && /tmp/weft-snap version --json`
Expected: JSON envelope whose `data.version` is the snapshot-decorated string GoReleaser injects (e.g. `0.1.0-SNAPSHOT-<shortsha>`) — confirming the `-X …cli.Version` path reaches the binary. (A snapshot version is intentionally non-clean-semver; `weft install` would treat it as dev.)

- [ ] **Step 4: Commit**

`jj commit -m "build(weft-hjx.11): add GoReleaser config (binary build + cli.Version ldflags)"`

---

### Task 4: release-please configuration

**Files:**

- Create: `release-please-config.json`
- Create: `.release-please-manifest.json`
- Create: `CHANGELOG.md`

- [ ] **Step 1: Write the release-please config**

Create `release-please-config.json`:

```json
{
  "$schema": "https://raw.githubusercontent.com/googleapis/release-please/main/schemas/config.json",
  "packages": {
    ".": {
      "release-type": "go",
      "bump-minor-pre-major": true,
      "bump-patch-for-minor-pre-major": true,
      "extra-files": [
        {
          "type": "json",
          "path": "plugin/.claude-plugin/plugin.json",
          "jsonpath": "$.version"
        }
      ]
    }
  }
}
```

- [ ] **Step 2: Write the manifest**

Create `.release-please-manifest.json`:

```json
{
  ".": "0.0.0"
}
```

- [ ] **Step 3: Seed the changelog**

Create `CHANGELOG.md`:

```markdown
# Changelog
```

- [ ] **Step 4: Validate the JSON**

Run: `jq -e '.packages["."]["release-type"] == "go"' release-please-config.json && jq -e '."." == "0.0.0"' .release-please-manifest.json`
Expected: both print `true` (valid JSON, expected shape). (release-please itself validates the full config on its first Action run — the open release PR is the integration proof.)

- [ ] **Step 5: Commit**

`jj commit -m "ci(weft-hjx.11): add release-please config, manifest, and changelog seed"`

---

### Task 5: `.github/workflows/release.yml`

**Files:**

- Create: `.github/workflows/release.yml` (creates the `.github/workflows/` directory)

- [ ] **Step 1: Write the workflow**

Create `.github/workflows/release.yml`:

```yaml
# SPDX-License-Identifier: Apache-2.0
# Copyright 2026 Weft Contributors
name: release

on:
  push:
    branches: [main]

permissions:
  contents: write
  pull-requests: write

jobs:
  release:
    runs-on: ubuntu-latest
    steps:
      - uses: googleapis/release-please-action@v5
        id: release
        with:
          config-file: release-please-config.json
          manifest-file: .release-please-manifest.json
      # Build + upload the binary only when a release was just created — check
      # out AT THE TAG release-please cut (not the triggering main HEAD).
      - if: ${{ steps.release.outputs.release_created }}
        uses: actions/checkout@v6
        with:
          ref: ${{ steps.release.outputs.tag_name }}
          fetch-depth: 0
      - if: ${{ steps.release.outputs.release_created }}
        uses: actions/setup-go@v6
        with:
          go-version-file: go.mod
      - if: ${{ steps.release.outputs.release_created }}
        uses: goreleaser/goreleaser-action@v7
        with:
          distribution: goreleaser
          version: "~> v2"
          args: release --clean
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

- [ ] **Step 2: Lint the workflow**

Run: `actionlint .github/workflows/release.yml`
Expected: no output (valid). If `actionlint` is absent: `go install github.com/rhysd/actionlint/cmd/actionlint@latest` first.

- [ ] **Step 3: Verify action major versions are current**

Confirm against the marketplace (per the GitHub-Actions-version rule) the current majors — `googleapis/release-please-action@v5` (v5.0.0, 2026-04, Node-24 bump; inputs/outputs unchanged from v4), `actions/checkout@v6`, `actions/setup-go@v6`, `goreleaser/goreleaser-action@v7` (all verified 2026-06) — then pin each `uses:` to a vetted commit SHA with the version as a trailing comment. **Verify the latest *release* tag, not just the README's example snippets** (the release-please README still shows `@v4`, but v5 is the current major).

- [ ] **Step 4: Commit**

`jj commit -m "ci(weft-hjx.11): add release workflow (release-please + GoReleaser)"`

---

### Task 6: `.github/workflows/ci.yml`

**Files:**

- Create: `.github/workflows/ci.yml`

- [ ] **Step 1: Write the workflow**

Create `.github/workflows/ci.yml`:

```yaml
# SPDX-License-Identifier: Apache-2.0
# Copyright 2026 Weft Contributors
name: ci

on:
  pull_request:
  push:
    branches: [main]

permissions:
  contents: read

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v6
      - uses: actions/setup-go@v6
        with:
          go-version-file: go.mod
      - run: go build ./...
      - run: go test ./...
      - uses: goreleaser/goreleaser-action@v7
        with:
          distribution: goreleaser
          version: "~> v2"
          args: check

  plugin-validate:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v6
      - uses: actions/setup-node@v6
        with:
          node-version: "20"
      # claude.ai/install.sh returns HTTP 403 in CI behind Cloudflare
      # (anthropics/claude-code#36306); the npm package is the CI-safe install.
      - run: npm install -g @anthropic-ai/claude-code
      - run: claude plugin validate ./plugin --strict
      - run: claude plugin validate . --strict
      # ${CLAUDE_PLUGIN_ROOT} discipline: no stale weft/ intra-tree paths in the
      # installed plugin tree (the weft/ tree does not exist in the plugin cache).
      - name: grep-discipline
        run: |
          if grep -RnE 'weft/(agents|references|workflows)/' plugin/; then
            echo "::error::stale weft/ intra-tree path in plugin/ (use \${CLAUDE_PLUGIN_ROOT})"
            exit 1
          fi

  commit-lint:
    if: github.event_name == 'pull_request'
    runs-on: ubuntu-latest
    steps:
      - uses: amannn/action-semantic-pull-request@v6
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

- [ ] **Step 2: Lint the workflow**

Run: `actionlint .github/workflows/ci.yml`
Expected: no output (valid).

- [ ] **Step 3: Verify the grep-discipline guard fires correctly**

Run locally: `grep -RnE 'weft/(agents|references|workflows)/' plugin/ ; echo "exit=$?"`
Expected: `exit=1` (grep found nothing → exit 1 in the shell, meaning the guard PASSES — no stale paths). This confirms the plugin tree is clean today; the CI step inverts grep's exit so a *found* path fails the job.

- [ ] **Step 4: Verify action majors + pin SHAs**

As Task 5 Step 3, plus confirm `actions/setup-node@v6` and `amannn/action-semantic-pull-request@v6` are current majors; pin each to a vetted SHA.

- [ ] **Step 5: Commit**

`jj commit -m "ci(weft-hjx.11): add CI gate (test, build, goreleaser check, plugin validate, commit-lint)"`

---

### Task 7: `Taskfile.yml`

**Files:**

- Create: `Taskfile.yml`

- [ ] **Step 1: Write the Taskfile**

Create `Taskfile.yml`:

```yaml
# SPDX-License-Identifier: Apache-2.0
# Copyright 2026 Weft Contributors
version: "3"

tasks:
  build:
    desc: Build the weft binary
    cmds:
      - go build ./...

  test:
    desc: Run the test suite
    cmds:
      - go test ./...

  validate:
    desc: Strict-validate the plugin and marketplace manifests
    cmds:
      - claude plugin validate ./plugin --strict
      - claude plugin validate . --strict

  snapshot:
    desc: Build a local snapshot release (no publish)
    cmds:
      - goreleaser release --snapshot --clean

  check:
    desc: Validate the GoReleaser config
    cmds:
      - goreleaser check

  release:preview:
    desc: Show the open release-please PR (or note none open)
    cmds:
      - 'gh pr list --search "chore(main): release" --state open --json number,title || echo "no open release PR"'
```

- [ ] **Step 2: Verify the Taskfile parses**

Run: `task --list`
Expected: lists `build`, `check`, `release:preview`, `snapshot`, `test`, `validate` with their descriptions. (If `task` is absent: `go install github.com/go-task/task/v3/cmd/task@latest`.)

- [ ] **Step 3: Commit**

`jj commit -m "build(weft-hjx.11): add Taskfile (build/test/validate/snapshot/check)"`

---

### Task 8: Remove cocogitto; update project docs

**Files:**

- Delete: `cog.toml`
- Modify: `CLAUDE.md:33-34`
- Modify: `docs/design.md:219`, `docs/design.md:221`

- [ ] **Step 1: Delete the cocogitto stub**

Run: `rm cog.toml`

- [ ] **Step 2: Update `CLAUDE.md` Conventions**

In `CLAUDE.md`, replace the two-line convention (currently lines 33–34):

```
- Conventional commits; cocogitto (`cog.toml`) tag-only releases; validated in
  CI, not via local hooks (jj does not fire git hooks reliably).
```

with:

```
- Conventional commits; release-please maintains a release PR (bumps
  `CHANGELOG.md` + `plugin/.claude-plugin/plugin.json`), merging it cuts the
  `vX.Y.Z` tag and GitHub Release; GoReleaser builds the binary. Conventional
  commits validated in CI, not via local hooks (jj does not fire git hooks
  reliably).
```

- [ ] **Step 3: Update `docs/design.md` §8**

In `docs/design.md`, replace line 219:

```
- cocogitto (`cog.toml`) tag-only releases; conventional commits enforced in CI
```

with:

```
- release-please (release-PR workflow) + GoReleaser for releases; conventional
  commits enforced in CI (see `docs/seams/08-release-distribution-pipeline.md`)
```

and replace line 221:

```
- (deferred until build starts: Taskfile, `.golangci.yaml`, `.goreleaser.yaml`)
```

with:

```
- `Taskfile.yml` + `.goreleaser.yml` added in seam 8; `.golangci.yaml` still
  deferred
```

- [ ] **Step 4: Verify the cocogitto references are gone and the build is green**

Run: `! test -f cog.toml && ! grep -rn 'cocogitto\|cog\.toml' CLAUDE.md docs/design.md && go build ./...`
Expected: prints nothing and exits 0 (file deleted, no stale cocogitto references in the two docs, build succeeds).

- [ ] **Step 5: Commit**

`jj commit -m "chore(weft-hjx.11): remove cocogitto; point docs at release-please pipeline"`

---

## Done criteria

- `go test ./...` and `go build ./...` green.
- `goreleaser check` passes; `task snapshot` produces a binary whose `weft version` reports the injected version.
- `actionlint` passes on both workflows; `claude plugin validate` ×2 `--strict` pass.
- `cog.toml` gone; `CLAUDE.md` + `docs/design.md` reference release-please; no stale cocogitto references.
- On merge to `main`, release-please opens a release PR proposing `0.1.0` that bumps `CHANGELOG.md` + `plugin/.claude-plugin/plugin.json`; merging it cuts `v0.1.0` and GoReleaser uploads the binary. (Cutting the first real release is the human act — out of scope per spec §1.)
<!-- adr-capture: sha256=49b7b0c42d4a1418; session=cli; ts=2026-06-06T16:06:22Z; adrs=weft-roc,weft-1hf,weft-19y -->
