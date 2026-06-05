<!--
  ~ SPDX-License-Identifier: Apache-2.0
  ~ Copyright 2026 Weft Contributors
-->

# Weft install (seam 7) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use dev-flow:subagent-driven-development (recommended) or dev-flow:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Distribute the `weft/` prompt tree as a native Claude Code plugin via an in-repo marketplace, installed by a thin version-pinned `weft install` verb.

**Architecture:** Two independent deliverables. (B) A Go verb `weft install` that drives Claude Code's own `claude plugin` CLI (new `run.Claude` wrapper; logic in `internal/install`, cobra wiring in `internal/cli`); it pins the install to the binary's release tag (`weft--vX.Y.Z`) so engine verbs and prompts stay in lockstep. (A) The `weft/` markdown tree re-authored in-repo as a plugin under `plugin/` (command+workflow pairs collapse into skills, references become shared skill files, two manifests) — markdown authoring gated by `claude plugin validate --strict`, not `go test`.

**Tech Stack:** Go 1.26, cobra, the existing `internal/run` Runner abstraction + `internal/exit` + `internal/envelope`; Claude Code v2.1.165 plugin model (`claude plugin` CLI).

**Spec:** [`docs/seams/07-weft-install.md`](../seams/07-weft-install.md) · **Design bead:** `weft-hjx.10`.

**Grounding note:** all Claude Code plugin claims (manifest schema, `/<plugin>:<name>` namespacing, `claude plugin` CLI surface, `{name}--v{version}` tag, `--scope`, `--strict`) are verified against v2.1.165 and recorded as `bd note`s on `weft-hjx.10` (context7 + claude-code-guide live-docs sweep + the local CLI). Re-verify the CLI surface with `claude plugin --help` before starting Group B.

---

## File structure

**Group B — the verb (Go):**

- `internal/run/run.go` — add `Claude(r Runner, args ...string)` (mirrors `GH`).
- `internal/install/install.go` — `Options`, `Result`, `Install`, `resolveSource`, validators (`validateScope`/`validateRef`/`validateLocal`). Depends only on `run.Runner` + `internal/exit`.
- `internal/install/install_test.go` — table + fake-runner tests.
- `internal/cli/install.go` — `newInstallCmd()` (flags, RunE → `install.Install` → `Emit` envelope).
- `internal/cli/install_test.go` — verb tests via the `newTestCmd` harness.
- `internal/cli/root.go:48-57` — register `app.newInstallCmd()`.

**Group A — the plugin (markdown):**

- `plugin/.claude-plugin/plugin.json`, `.claude-plugin/marketplace.json` — manifests.
- `plugin/skills/execute/SKILL.md`, `plugin/skills/new-project/SKILL.md` — collapsed command+workflow.
- `plugin/agents/{executor,planner,resolver,reviewer}.md` — from `weft/agents/`.
- `plugin/references/{jj-agent-safety,bead-change-spine,tdd-verify-discipline}.md` — from `weft/references/`.
- `plugin/README.md`, `plugin/NOTICE`.

The two groups are independent (no ordering dependency between them). Within Group B, Tasks 1→5 are ordered. Within Group A, Task 6 (manifests) precedes the final validation in Task 11; Tasks 7–10 are independent.

---

## Task 1: `run.Claude` wrapper

**Files:**

- Modify: `internal/run/run.go` (add after `GH`)
- Modify: `internal/run/run_test.go` (add a test; the file already exists with `fakeRunner` + `equal` helpers and `TestGHInvokesRunnerWithNameAndArgs` — reuse them, do NOT redefine types)

- [ ] **Step 1: Write the failing test**

Add to the **existing** `internal/run/run_test.go` (modeled on the existing `TestGHInvokesRunnerWithNameAndArgs`; reuses the file's `fakeRunner` + `equal`):

```go
func TestClaudeInvokesRunnerWithNameAndArgs(t *testing.T) {
	f := &fakeRunner{}
	if _, err := Claude(f, "plugin", "marketplace", "add", "seanb4t/weft@weft--v1.4.0"); err != nil {
		t.Fatalf("Claude: %v", err)
	}
	if f.name != "claude" {
		t.Errorf("binary = %q, want claude", f.name)
	}
	want := []string{"plugin", "marketplace", "add", "seanb4t/weft@weft--v1.4.0"}
	if !equal(f.args, want) {
		t.Errorf("args = %v, want %v", f.args, want)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/run/ -run TestClaudeInvokesRunnerWithNameAndArgs`
Expected: FAIL — `undefined: Claude`.

- [ ] **Step 3: Add the wrapper**

In `internal/run/run.go`, after the `GH` function:

```go
// Claude runs the Claude Code CLI (introduced for `weft install`: it drives
// `claude plugin marketplace add` / `install`). Like bd/jj/gh it is a
// deterministic CLI wrapper, not agent dispatch.
func Claude(r Runner, args ...string) (Result, error) {
	return r.Run("claude", args...)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/run/ -run TestClaudeInvokesRunnerWithNameAndArgs`
Expected: PASS.

- [ ] **Step 5: Commit**

`jj commit -m "feat(weft-hjx.10): run.Claude — 4th wrapped CLI for weft install"` (per `references/vcs-preamble.md`; jj repo).

---

## Task 2: install option validation (scope / ref / local)

**Files:**

- Create: `internal/install/install.go`
- Test: `internal/install/install_test.go`

- [ ] **Step 1: Write the failing test**

```go
// internal/install/install_test.go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Weft Contributors

package install

import (
	"testing"

	"github.com/seanb4t/weft/internal/exit"
)

func TestValidateScope(t *testing.T) {
	for _, ok := range []string{"user", "project", "local"} {
		if err := validateScope(ok); err != nil {
			t.Errorf("scope %q should be valid: %v", ok, err)
		}
	}
	for _, bad := range []string{"global", "", "User", "-x"} {
		if err := validateScope(bad); exit.Code(err) != 1 {
			t.Errorf("scope %q must be invocation error (exit 1), got %v", bad, err)
		}
	}
}

func TestValidateRefAllowlist(t *testing.T) {
	for _, ok := range []string{"main", "weft--v1.2.3", "v0.4.0", "0123456789abcdef0123456789abcdef01234567"} {
		if err := validateRef(ok); err != nil {
			t.Errorf("ref %q should be valid: %v", ok, err)
		}
	}
	for _, bad := range []string{"-rf", "a b", "a&b", "a|b", "a;b", "a$(x)", ".."} {
		if err := validateRef(bad); exit.Code(err) != 1 {
			t.Errorf("ref %q must be rejected (exit 1), got %v", bad, err)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/install/`
Expected: FAIL — package/symbols undefined.

- [ ] **Step 3: Write the validators**

```go
// internal/install/install.go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Weft Contributors

// Package install drives Claude Code's `claude plugin` CLI to register the weft
// repo as a marketplace and install the weft plugin, pinned to the running
// binary's release (spec docs/seams/07-weft-install.md). It depends only on the
// run.Runner interface so it is unit-testable with the engine's fake runner.
package install

import (
	"os"
	"regexp"
	"strings"

	"github.com/seanb4t/weft/internal/exit"
)

// validScopes are the Claude Code install scopes (claude plugin install --scope).
var validScopes = map[string]bool{"user": true, "project": true, "local": true}

// refPattern allowlists a git ref before it is interpolated into the
// `claude plugin marketplace add <source>@<ref>` argument. Leading char is
// alphanumeric (rejects a leading '-' that bd/claude could read as a flag); the
// rest is the git-ref-safe set [A-Za-z0-9._/-]. This excludes every shell/
// revset metacharacter and rejects ".." traversal. Mirrors the guard idiom on
// changeIDPattern/epicIDPattern (conflict.go/finish.go); see the engram memory
// weft-cli-validate-user-id-before-revset-or-gh-api.
var refPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._/-]*$`)

func validateScope(scope string) error {
	if !validScopes[scope] {
		return exit.Invocationf("invalid --scope %q — must be user, project, or local", scope)
	}
	return nil
}

func validateRef(ref string) error {
	if !refPattern.MatchString(ref) || strings.Contains(ref, "..") {
		return exit.Invocationf("invalid --ref %q", ref)
	}
	return nil
}

// validateLocal requires an existing directory carrying a marketplace manifest,
// and rejects a leading dash (flag confusion).
func validateLocal(path string) error {
	if path == "" || strings.HasPrefix(path, "-") {
		return exit.Invocationf("invalid --local path %q", path)
	}
	if fi, err := os.Stat(path + "/.claude-plugin/marketplace.json"); err != nil || fi.IsDir() {
		return exit.Invocationf("--local %q has no .claude-plugin/marketplace.json", path)
	}
	return nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/install/`
Expected: PASS.

- [ ] **Step 5: Commit**

`jj commit -m "feat(weft-hjx.10): install option validation (scope/ref/local guard)"`

---

## Task 3: source resolution (default tag / dev refusal / ref / local)

**Files:**

- Modify: `internal/install/install.go`
- Test: `internal/install/install_test.go`

- [ ] **Step 1: Write the failing test**

```go
func TestResolveSourceDefaultPinsPluginTag(t *testing.T) {
	src, ref, err := resolveSource("1.4.0", "", "")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if src != "seanb4t/weft" || ref != "weft--v1.4.0" {
		t.Errorf("default must pin seanb4t/weft@weft--v1.4.0, got %q@%q", src, ref)
	}
}

func TestResolveSourceDevVersionRefuses(t *testing.T) {
	if _, _, err := resolveSource("0.0.0-dev", "", ""); exit.Code(err) != 1 {
		t.Errorf("dev/untagged version with no --ref/--local must refuse (exit 1), got %v", err)
	}
}

func TestResolveSourceRefOverride(t *testing.T) {
	src, ref, err := resolveSource("0.0.0-dev", "main", "")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if src != "seanb4t/weft" || ref != "main" {
		t.Errorf("--ref must override to seanb4t/weft@main, got %q@%q", src, ref)
	}
}

func TestResolveSourceLocalUsesPathNoRef(t *testing.T) {
	src, ref, err := resolveSource("0.0.0-dev", "", "/tmp/weft-clone")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if src != "/tmp/weft-clone" || ref != "" {
		t.Errorf("--local must use the path with no ref, got %q@%q", src, ref)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/install/ -run TestResolveSource`
Expected: FAIL — `undefined: resolveSource`.

- [ ] **Step 3: Implement `resolveSource`**

Add to `internal/install/install.go`:

```go
const repoSlug = "seanb4t/weft"

// semverPattern matches a clean release version X.Y.Z only (no pre-release or
// build suffix). The dev sentinel "0.0.0-dev" — and any other suffixed build —
// deliberately fails it, so the default pin path refuses to float; pre-release
// or dev builds must pass --ref (or --local).
var semverPattern = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+$`)

// resolveSource picks the marketplace source + ref. Precedence: --local (a clone
// path, no ref) > --ref (override the ref on the repo) > default (pin the plugin
// tag weft--v<version> for a released binary). A dev/untagged version with no
// --ref/--local refuses rather than silently floating to a branch (spec §4.2).
func resolveSource(version, ref, local string) (source, refArg string, err error) {
	if local != "" {
		return local, "", nil
	}
	if ref != "" {
		return repoSlug, ref, nil
	}
	if !semverPattern.MatchString(version) {
		return "", "", exit.Invocationf(
			"weft %s is not a released build — pass --ref <git-ref> or --local <path> to install", version)
	}
	return repoSlug, "weft--v" + version, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/install/ -run TestResolveSource`
Expected: PASS.

- [ ] **Step 5: Commit**

`jj commit -m "feat(weft-hjx.10): install source resolution + dev-build refusal"`

---

## Task 4: install orchestration (prereq, marketplace add+fallback, install, uninstall)

**Files:**

- Modify: `internal/install/install.go`
- Test: `internal/install/install_test.go`

- [ ] **Step 1: Write the failing test**

```go
import (
	"strings"
	// ...existing imports...
	"github.com/seanb4t/weft/internal/run"
)

// scriptRunner records calls and returns scripted results keyed on the joined
// arg string. (Named distinctly from the cli package's routeRunner — different
// package, different fn signature: this one keys on the pre-joined string.)
type scriptRunner struct {
	fn    func(j string) run.Result
	calls [][]string
}

func (r *scriptRunner) Run(name string, args ...string) (run.Result, error) {
	r.calls = append(r.calls, append([]string{name}, args...))
	if r.fn == nil {
		return run.Result{Code: 0}, nil
	}
	return r.fn(strings.Join(append([]string{name}, args...), " ")), nil
}

func okRunner() *scriptRunner {
	return &scriptRunner{fn: func(j string) run.Result {
		if strings.Contains(j, "--version") { // the `claude` prereq probe
			return run.Result{Stdout: "2.1.165", Code: 0}
		}
		return run.Result{Code: 0}
	}}
}

func TestInstallDefaultDrivesMarketplaceThenInstall(t *testing.T) {
	r := okRunner()
	res, err := Install(r, Options{Version: "1.4.0", Scope: "user"})
	if err != nil {
		t.Fatalf("install: %v", err)
	}
	joined := make([]string, len(r.calls))
	for i, c := range r.calls {
		joined[i] = strings.Join(c, " ")
	}
	add, ins := -1, -1
	for i, j := range joined {
		if strings.Contains(j, "plugin marketplace add seanb4t/weft@weft--v1.4.0") {
			add = i
		}
		if strings.Contains(j, "plugin install weft@weft --scope user") {
			ins = i
		}
	}
	if add < 0 || ins < 0 || add > ins {
		t.Fatalf("must add marketplace then install (add=%d ins=%d): %v", add, ins, joined)
	}
	if !res.Registered || !res.Installed {
		t.Errorf("result must report registered+installed: %+v", res)
	}
}

func TestInstallClaudeAbsentIsHardError(t *testing.T) {
	// errRunner fails to start (simulates `claude` missing from PATH) → exit 2.
	if _, err := Install(&errRunner{}, Options{Version: "1.4.0", Scope: "user"}); exit.Code(err) != 2 {
		t.Errorf("claude absent must be hard error (exit 2), got %v", err)
	}
}

func TestInstallSubprocessFailureIsHardError(t *testing.T) {
	r := &scriptRunner{fn: func(j string) run.Result {
		if strings.Contains(j, "--version") {
			return run.Result{Stdout: "2.1.165", Code: 0}
		}
		if strings.Contains(j, "plugin install") {
			return run.Result{Code: 1, Stderr: "boom"}
		}
		return run.Result{Code: 0}
	}}
	if _, err := Install(r, Options{Version: "1.4.0", Scope: "user"}); exit.Code(err) != 2 {
		t.Errorf("non-zero claude plugin exit must be hard (exit 2), got %v", err)
	}
}

func TestInstallUninstallRunsUninstallOnly(t *testing.T) {
	r := okRunner()
	if _, err := Install(r, Options{Version: "1.4.0", Scope: "user", Uninstall: true}); err != nil {
		t.Fatalf("uninstall: %v", err)
	}
	var sawUninstall, sawInstall bool
	for _, c := range r.calls {
		j := strings.Join(c, " ")
		if strings.Contains(j, "plugin uninstall weft --scope user -y") {
			sawUninstall = true
		}
		if strings.Contains(j, "plugin install weft@") {
			sawInstall = true
		}
	}
	if !sawUninstall || sawInstall {
		t.Errorf("uninstall path must run uninstall -y and not install (un=%v in=%v)", sawUninstall, sawInstall)
	}
}

// errRunner fails to start (simulates `claude` missing from PATH).
type errRunner struct{}

func (errRunner) Run(string, ...string) (run.Result, error) {
	return run.Result{}, errStub
}

var errStub = stubErr("claude: not found")

type stubErr string

func (e stubErr) Error() string { return string(e) }
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/install/ -run TestInstall`
Expected: FAIL — `undefined: Install`, `Options`.

- [ ] **Step 3: Implement `Options`, `Result`, `Install`**

Add to `internal/install/install.go`:

```go
import (
	// add to the existing import block:
	"github.com/seanb4t/weft/internal/run"
)

// Options drives one weft install invocation.
type Options struct {
	Version   string // the binary's version (cli.Version); pins weft--v<Version>
	Scope     string // user | project | local
	Ref       string // optional ref override (branch/tag/sha)
	Local     string // optional local clone path (marketplace source)
	Uninstall bool   // remove instead of install
	DryRun    bool   // report the commands without running them
}

// Result is the install outcome (becomes the envelope data).
type Result struct {
	Plugin      string
	Marketplace string
	Source      string
	Ref         string
	Scope       string
	Uninstall   bool
	Registered  bool
	Installed   bool
	Commands    []string
}

const pluginName = "weft"

// Install validates options, ensures the claude CLI is reachable, then drives
// `claude plugin` to register the marketplace and install (or uninstall) the
// weft plugin. Best-effort nothing here: any non-zero claude exit is surfaced.
func Install(r run.Runner, o Options) (Result, error) {
	if err := validateScope(o.Scope); err != nil {
		return Result{}, err
	}
	if o.Ref != "" {
		if err := validateRef(o.Ref); err != nil {
			return Result{}, err
		}
	}
	if o.Local != "" {
		if err := validateLocal(o.Local); err != nil {
			return Result{}, err
		}
	}
	res := Result{Plugin: pluginName, Marketplace: pluginName, Scope: o.Scope, Uninstall: o.Uninstall, Commands: []string{}}

	if o.Uninstall {
		res.Commands = []string{"claude plugin uninstall " + pluginName + " --scope " + o.Scope + " -y"}
		if o.DryRun {
			return res, nil
		}
		if err := claudeCheck(r); err != nil {
			return res, err
		}
		if err := runClaude(r, "plugin", "uninstall", pluginName, "--scope", o.Scope, "-y"); err != nil {
			return res, err
		}
		return res, nil
	}

	source, refArg, err := resolveSource(o.Version, o.Ref, o.Local)
	if err != nil {
		return Result{}, err
	}
	res.Source, res.Ref = source, refArg
	addArg := source
	if refArg != "" {
		addArg = source + "@" + refArg
	}
	res.Commands = []string{
		"claude plugin marketplace add " + addArg,
		"claude plugin install " + pluginName + "@" + pluginName + " --scope " + o.Scope,
	}
	if o.DryRun {
		return res, nil
	}
	if err := claudeCheck(r); err != nil {
		return res, err
	}
	if err := registerMarketplace(r, addArg); err != nil {
		return res, err
	}
	res.Registered = true
	if err := runClaude(r, "plugin", "install", pluginName+"@"+pluginName, "--scope", o.Scope); err != nil {
		return res, err
	}
	res.Installed = true
	return res, nil
}

// claudeCheck probes that the claude CLI is reachable; a runner error means it is
// not on PATH (a hard failure — the verb cannot proceed without the host CLI).
func claudeCheck(r run.Runner) error {
	if _, err := run.Claude(r, "--version"); err != nil {
		return exit.Hardf("claude CLI not found on PATH — install Claude Code, or run the printed commands by hand: %v", err)
	}
	return nil
}

// runClaude runs one `claude plugin …` step, mapping both failure modes
// (could-not-start, non-zero exit) to a hard error with the stderr surfaced.
func runClaude(r run.Runner, args ...string) error {
	res, err := run.Claude(r, args...)
	if err != nil {
		return exit.Hardf("claude %s could not run: %v", strings.Join(args, " "), err)
	}
	if res.Code != 0 {
		return exit.Hardf("claude %s failed: %s", strings.Join(args, " "), strings.TrimSpace(res.Stderr))
	}
	return nil
}

// registerMarketplace adds the marketplace, tolerating an already-registered name
// by removing and re-adding (the live CLI's duplicate-add semantic is unconfirmed
// — spec §4.4; an integration test pins the real behavior).
func registerMarketplace(r run.Runner, addArg string) error {
	if res, err := run.Claude(r, "plugin", "marketplace", "add", addArg); err == nil && res.Code == 0 {
		return nil
	}
	// Fallback: remove then re-add.
	_, _ = run.Claude(r, "plugin", "marketplace", "remove", pluginName)
	return runClaude(r, "plugin", "marketplace", "add", addArg)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/install/`
Expected: PASS.

- [ ] **Step 5: Commit**

`jj commit -m "feat(weft-hjx.10): install orchestration — marketplace add + install/uninstall"`

---

## Task 5: `weft install` cobra verb + envelope + root registration

**Files:**

- Create: `internal/cli/install.go`
- Test: `internal/cli/install_test.go`
- Modify: `internal/cli/root.go` (add registration after the `app.newFinishCmd()` line)

- [ ] **Step 1: Write the failing test**

```go
// internal/cli/install_test.go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Weft Contributors

package cli

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/seanb4t/weft/internal/exit"
	"github.com/seanb4t/weft/internal/run"
)

func installRunner() *routeRunner {
	return &routeRunner{fn: func(name string, args []string) run.Result {
		j := strings.Join(append([]string{name}, args...), " ")
		if strings.Contains(j, "--version") {
			return run.Result{Stdout: "2.1.165", Code: 0}
		}
		return run.Result{Code: 0}
	}}
}

// NOTE: cli.Version is the dev sentinel "0.0.0-dev" during tests, which
// resolveSource refuses (it can't derive a release tag). So every cli install
// test that reaches resolveSource passes --ref to supply a resolvable source.
// (The validation/uninstall tests below short-circuit before resolveSource.)
func TestInstallDryRunRunsNoSubprocess(t *testing.T) {
	r := installRunner()
	out, err := newTestCmd(r, "install", "--ref", "main", "--dry-run", "--json")
	if err != nil {
		t.Fatalf("dry-run: %v", err)
	}
	if len(r.calls) != 0 {
		t.Errorf("dry-run must run no subprocess; saw %v", r.calls)
	}
	var env struct {
		Data struct {
			Commands []string `json:"commands"`
			DryRun   bool     `json:"dry_run"`
		} `json:"data"`
	}
	if e := json.Unmarshal(out.Bytes(), &env); e != nil {
		t.Fatalf("envelope: %v\n%s", e, out.String())
	}
	if !env.Data.DryRun || len(env.Data.Commands) != 2 {
		t.Errorf("dry-run envelope must carry dry_run:true + 2 commands: %s", out.String())
	}
}

func TestInstallRejectsBadScope(t *testing.T) {
	r := installRunner()
	_, err := newTestCmd(r, "install", "--scope", "global")
	if exit.Code(err) != 1 {
		t.Errorf("bad scope must be exit 1, got %v", err)
	}
	if len(r.calls) != 0 {
		t.Errorf("no subprocess before validation; saw %v", r.calls)
	}
}

func TestInstallRejectsInjectionRef(t *testing.T) {
	for _, bad := range []string{"-rf", "a b", "a&all()", ".."} {
		r := installRunner()
		_, err := newTestCmd(r, "install", "--ref", bad)
		if exit.Code(err) != 1 {
			t.Errorf("ref %q must be exit 1, got %v", bad, err)
		}
		if len(r.calls) != 0 {
			t.Errorf("ref %q: no subprocess before validation; saw %v", bad, r.calls)
		}
	}
}

func TestInstallEnvelopeCommandsNeverNull(t *testing.T) {
	r := installRunner()
	out, err := newTestCmd(r, "install", "--ref", "main", "--dry-run", "--json")
	if err != nil {
		t.Fatalf("dry-run: %v", err)
	}
	if !strings.Contains(out.String(), `"commands": [`) {
		t.Errorf("commands must serialize as a JSON array, never null: %s", out.String())
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/cli/ -run TestInstall`
Expected: FAIL — `unknown command "install"`.

- [ ] **Step 3: Write the cobra verb**

```go
// internal/cli/install.go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Weft Contributors

package cli

import (
	"github.com/spf13/cobra"

	"github.com/seanb4t/weft/internal/install"
)

func (a *App) newInstallCmd() *cobra.Command {
	var scope, ref, local string
	var uninstall, dryRun bool
	c := &cobra.Command{
		Use:   "install",
		Short: "Install the weft Claude Code plugin (pinned to this binary's release)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			res, err := install.Install(a.Runner, install.Options{
				Version:   Version,
				Scope:     scope,
				Ref:       ref,
				Local:     local,
				Uninstall: uninstall,
				DryRun:    dryRun,
			})
			if err != nil {
				return err
			}
			data := map[string]any{
				"plugin": res.Plugin, "marketplace": res.Marketplace,
				"source": res.Source, "ref": res.Ref, "scope": res.Scope,
				"uninstall": res.Uninstall, "registered": res.Registered,
				"installed": res.Installed, "commands": res.Commands,
				"dry_run": dryRun,
			}
			text := "installed weft plugin (" + res.Scope + ")"
			switch {
			case dryRun:
				text = "[dry-run] would run:\n  " + joinLines(res.Commands)
			case uninstall:
				text = "uninstalled weft plugin (" + res.Scope + ")"
			}
			return Emit(cmd, "install", data, text)
		},
	}
	c.Flags().StringVar(&scope, "scope", "user", "install scope: user | project | local")
	c.Flags().StringVar(&ref, "ref", "", "override the git ref (branch/tag/sha) instead of the version tag")
	c.Flags().StringVar(&local, "local", "", "install from a local clone path (offline) instead of the git marketplace")
	c.Flags().BoolVar(&uninstall, "uninstall", false, "uninstall the weft plugin")
	c.Flags().BoolVar(&dryRun, "dry-run", false, "print the claude plugin commands without running them")
	return c
}
```

Reuse the existing `splitTrimLines`/line helpers if a `joinLines` already exists; otherwise add a one-liner to `internal/cli/lines.go`:

```go
// joinLines renders a command list as newline-indented text for the next hint.
func joinLines(xs []string) string { return strings.Join(xs, "\n  ") }
```
(Confirm `internal/cli/lines.go` imports `strings`; it already uses it.)

- [ ] **Step 4: Register on the root command**

In `internal/cli/root.go`, immediately after the `root.AddCommand(app.newFinishCmd())` line (the last `AddCommand` before `return root`):

```go
	root.AddCommand(app.newInstallCmd())
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/cli/ -run TestInstall && go build ./...`
Expected: PASS + clean build.

- [ ] **Step 6: Full gate + commit**

Run: `go vet ./... && go test ./... && gofmt -l internal/`
Expected: clean, all packages OK, no gofmt drift.
`jj commit -m "feat(weft-hjx.10): weft install verb — cobra wiring + envelope"`

---

## Task 6: plugin + marketplace manifests

**Files:**

- Create: `plugin/.claude-plugin/plugin.json`
- Create: `.claude-plugin/marketplace.json`

- [ ] **Step 1: Write `plugin/.claude-plugin/plugin.json`**

```json
{
  "$schema": "https://json.schemastore.org/claude-code-plugin-manifest.json",
  "name": "weft",
  "version": "0.0.0",
  "description": "Spec-driven AI dev orchestration on jj + beads — woven waves of ready picks.",
  "author": { "name": "Weft Contributors" },
  "homepage": "https://github.com/seanb4t/weft",
  "repository": "https://github.com/seanb4t/weft",
  "license": "Apache-2.0",
  "keywords": ["jj", "beads", "orchestration", "gsd"]
}
```

- [ ] **Step 2: Write `.claude-plugin/marketplace.json`**

```json
{
  "$schema": "https://json.schemastore.org/claude-code-marketplace.json",
  "name": "weft",
  "description": "The Weft plugin marketplace.",
  "plugins": [
    {
      "name": "weft",
      "source": "./plugin",
      "description": "Spec-driven AI dev orchestration on jj + beads.",
      "license": "Apache-2.0",
      "category": "development",
      "strict": true
    }
  ]
}
```

- [ ] **Step 3: Commit** (validation deferred to Task 11, after components exist)

`jj commit -m "feat(weft-hjx.10): weft plugin + marketplace manifests"`

---

## Task 7: plugin agents

**Files:**

- Create: `plugin/agents/{executor,planner,resolver,reviewer}.md`

- [ ] **Step 1: Copy each agent, dropping only the filename prefix**

Copy verbatim (content unchanged — frontmatter `name`, SPDX header, and GSD provenance comment all stay; the `weft-` prefix is dropped from the **filename** only, because the plugin namespace `weft:` already supplies it):

- `weft/agents/weft-executor.md` → `plugin/agents/executor.md`
- `weft/agents/weft-planner.md` → `plugin/agents/planner.md`
- `weft/agents/weft-resolver.md` → `plugin/agents/resolver.md`
- `weft/agents/weft-reviewer.md` → `plugin/agents/reviewer.md`

The frontmatter `name:` field (`weft-executor`, …) is **kept verbatim** — skills dispatch these agents by that name via the Task tool, where a distinct collision-proof name matters more than slash ergonomics (spec §3.4).

- [ ] **Step 2: Verify**

Run: `for f in plugin/agents/*.md; do head -3 "$f"; done`
Expected: each shows `name: weft-<role>` frontmatter.
Run: `grep -L "SPDX-License-Identifier" plugin/agents/*.md`
Expected: empty (every file has the SPDX header).

- [ ] **Step 3: Commit**

`jj commit -m "feat(weft-hjx.10): weft plugin agents (executor/planner/resolver/reviewer)"`

---

## Task 8: plugin references

**Files:**

- Create: `plugin/references/{jj-agent-safety,bead-change-spine,tdd-verify-discipline}.md`

- [ ] **Step 1: Copy verbatim**

- `weft/references/jj-agent-safety.md` → `plugin/references/jj-agent-safety.md`
- `weft/references/bead-change-spine.md` → `plugin/references/bead-change-spine.md`
- `weft/references/tdd-verify-discipline.md` → `plugin/references/tdd-verify-discipline.md`

Content unchanged (SPDX header retained). Skills will cite these via `${CLAUDE_PLUGIN_ROOT}/references/<file>.md` (Task 9).

- [ ] **Step 2: Verify**

Run: `ls plugin/references/ && grep -L "SPDX-License-Identifier" plugin/references/*.md`
Expected: 3 files; empty grep output.

- [ ] **Step 3: Commit**

`jj commit -m "feat(weft-hjx.10): weft plugin shared references"`

---

## Task 9: skill — `execute` (collapse command + workflow)

**Files:**

- Create: `plugin/skills/execute/SKILL.md`
- Read: `weft/commands/weft-execute.md`, `weft/workflows/execute.md`

- [ ] **Step 1: Author `plugin/skills/execute/SKILL.md`**

Compose the file as:

1. **Frontmatter** (carry the command's `description` + `argument-hint`; the skill name comes from the directory `execute`, so no `name:` needed):

```markdown
---
description: Weave a wave of ready picks — form the shed, isolate, dispatch executors, integrate, resolve conflicts, land.
argument-hint: "[epic-id]"
---

<!--
  ~ SPDX-License-Identifier: Apache-2.0
  ~ Copyright 2026 Weft Contributors
-->

<!-- adapted from /gsd-execute-phase (GSD Core, MIT) -->
```

2. **Body** = the full content of `weft/workflows/execute.md` (the orchestrator body; the thin command's prose is dropped — the skill *is* both entrypoint and body), with these **rewrites applied**:
   - Drop any self-reference to `weft/workflows/execute.md` (the skill is that body now).
   - Each `weft/references/<file>.md` citation → `${CLAUDE_PLUGIN_ROOT}/references/<file>.md`.
   - Agent dispatch references stay by Task name (`weft-executor`, `weft-reviewer`, `weft-resolver`) — those are the agent frontmatter names (Task 7), unchanged.
   - Drop repo-internal `docs/seams/*.md` "See also" paths (they do not ship in the plugin and won't resolve on a user's machine); if a pointer is valuable, replace with the homepage URL.
   - `$ARGUMENTS` / `$1` is the epic-id (the workflow's driving scope).

- [ ] **Step 2: Verify discipline**

Run: `grep -nE '\.planning/|worktree|gsd-tools|ROADMAP|SUMMARY|weft/workflows/|docs/seams/' plugin/skills/execute/SKILL.md`
Expected: empty (no Layer-B/C artifacts, no unresolved intra-tree paths).
Run: `grep -c "SPDX-License-Identifier" plugin/skills/execute/SKILL.md`
Expected: `1`.

- [ ] **Step 3: Commit**

`jj commit -m "feat(weft-hjx.10): weft plugin skill /weft:execute (collapsed command+workflow)"`

---

## Task 10: skill — `new-project` (collapse command + workflow)

**Files:**

- Create: `plugin/skills/new-project/SKILL.md`
- Read: `weft/commands/weft-new-project.md`, `weft/workflows/new-project.md`

- [ ] **Step 1: Author `plugin/skills/new-project/SKILL.md`**

Same procedure as Task 9, sourced from the new-project pair:

1. Frontmatter carries `weft/commands/weft-new-project.md`'s `description` + `argument-hint`, plus the SPDX + GSD-provenance comments.
2. Body = `weft/workflows/new-project.md` content with the same four rewrites (drop the workflow self-ref; `weft/references/*` → `${CLAUDE_PLUGIN_ROOT}/references/*`; keep agent Task names; drop `docs/seams/*` paths).

- [ ] **Step 2: Verify discipline**

Run: `grep -nE '\.planning/|worktree|gsd-tools|ROADMAP|SUMMARY|weft/workflows/|docs/seams/' plugin/skills/new-project/SKILL.md`
Expected: empty.
Run: `grep -c "SPDX-License-Identifier" plugin/skills/new-project/SKILL.md`
Expected: `1`.

- [ ] **Step 3: Commit**

`jj commit -m "feat(weft-hjx.10): weft plugin skill /weft:new-project (collapsed command+workflow)"`

---

## Task 11: README + NOTICE + plugin validation gate

**Files:**

- Create: `plugin/README.md`, `plugin/NOTICE`

- [ ] **Step 1: Write `plugin/NOTICE`** (GSD attribution, consistent with the repo NOTICE and seam-5 §6)

```text
Weft plugin
Copyright 2026 Weft Contributors
Licensed under the Apache License, Version 2.0.

Portions of the skills and agents in this plugin are adapted from
GSD Core (https://github.com/open-gsd/gsd-core), MIT-licensed,
© its contributors.
```

- [ ] **Step 2: Write `plugin/README.md`** (short: what the plugin is, the `/weft:execute` + `/weft:new-project` skills, the `weft`/`bd`/`jj`/`gh` prerequisites, and `weft install` as the installer).

- [ ] **Step 3: Run the native validator (the CI gate)**

Run: `claude plugin validate ./plugin --strict`
Expected: PASS (no unrecognized fields, all component paths resolve).
Run: `claude plugin validate . --strict`
Expected: PASS (the marketplace manifest; `source: "./plugin"` resolves).

If `claude` is unavailable in the execution environment, note it and defer the validate step to CI — but still run the grep-discipline checks below.

- [ ] **Step 4: Repo-wide discipline check**

Run: `grep -rnE '\.planning/|worktree|gsd-tools|ROADMAP|SUMMARY' plugin/ ; grep -rL "SPDX-License-Identifier" plugin/ --include="*.md"`
Expected: first grep empty; second lists only files that legitimately lack SPDX (none of the `.md` should — investigate any hit).

- [ ] **Step 5: Commit**

`jj commit -m "feat(weft-hjx.10): weft plugin README + NOTICE + validation gate"`

---

## Done criteria

- `go build ./... && go vet ./... && go test ./...` clean; `gofmt -l internal/` empty.
- `claude plugin validate ./plugin --strict` and `claude plugin validate . --strict` PASS.
- `weft install --dry-run --json` reports the two pinned `claude plugin` commands; `weft install --scope project --dry-run` reflects the scope; `weft install --ref -rf` exits 1 with no subprocess.
- The grep-discipline checks (Tasks 9–11) are clean.
- A follow-up (out of this seam): wire `claude plugin tag` + the `weft--vX.Y.Z` tag into the release process (cog.toml / CI) and bump `plugin.json.version` in lockstep — captured as an ADR (§9) and a release-automation bead.
<!-- adr-capture: sha256=c1e3c5973157ae69; session=cli; ts=2026-06-05T23:52:09Z; adrs=weft-ea7,weft-88z,weft-tzn,weft-1nt -->
