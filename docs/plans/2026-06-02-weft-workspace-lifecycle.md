<!--
  ~ SPDX-License-Identifier: Apache-2.0
  ~ Copyright 2026 Weft Contributors
-->

# Weft Workspace Lifecycle (Seam 3) — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the jj workspace lifecycle on the seam-1 engine foundation — `.weft/config.toml` loading, the `<repo>_worktrees/<bead-id>` path/identity layer, the thin `ws add`/`ws forget` verbs, the coarse `shed isolate` (status-first ordering) and `shed cleanup`, and the bead-state-reconciliation `weft reap`.

**Architecture:** Two new pure packages — `internal/config` (TOML) and `internal/workspace` (sanitization + path derivation) — feed new cobra verbs in `internal/cli` that wrap `jj`/`bd` through the existing injectable `run.Runner` (ADR `weft-re2`). Filesystem teardown uses `os.RemoveAll`; verbs are unit-tested with a recording fake runner and `t.TempDir()`. The `executor_live` liveness guard (spec §5/§10) is deferred — v1 `reap` collects any workspace whose owning bead is not `in_progress`.

**Tech Stack:** Go 1.26, `github.com/spf13/cobra` v1.9.x, `github.com/BurntSushi/toml` v1.x, stdlib `os`/`path/filepath`/`encoding/json`. Subprocesses: `jj`, `bd`.

**Spec:** `docs/seams/03-workspace-lifecycle.md` (design-reviewer READY). Layout §3, ordering §4, reaping §5, stale §6 (jj-stale policy is design-only this round — see Out of scope), config §7–§8.

---

## File Structure

| File | Responsibility |
|---|---|
| `internal/config/config.go` | `Config` struct + `Load` (TOML, missing-file → defaults) + `ShedMax`. |
| `internal/workspace/workspace.go` | `Sanitize`/`Desanitize`, `Name`, `Resolve` (kind-aware), `Root`, `Path`. |
| `internal/cli/root.go` (modify) | `App` gains a `Config` field; `NewRootCmd` registers `reap`. |
| `cmd/weft/main.go` (modify) | Load `.weft/config.toml` into `App.Config`. |
| `internal/cli/shed.go` (modify) | `shed form` default `--max` from config; add `shed isolate`, `shed cleanup`. |
| `internal/cli/ws.go` (modify) | Add `ws add`, `ws forget`; add the shared `jjRoot` helper. |
| `internal/cli/reap.go` | `weft reap` — reconcile `jj workspace list` ↔ bead status. |

Tests live beside each file. `internal/config` and `internal/workspace` are pure and built first; the cli verbs build on them and on the seam-1 `App`/`Emit`/`run` plumbing.

---

### Task 1: `internal/config` — `.weft/config.toml` loader

**Files:**
- Create: `internal/config/config.go`
- Test: `internal/config/config_test.go`

- [ ] **Step 1: Add the TOML dependency**

Run: `go get github.com/BurntSushi/toml@latest && go mod tidy`
Expected: `go.mod` gains `require github.com/BurntSushi/toml v1.x.x`.

- [ ] **Step 2: Write the failing test**

```go
// internal/config/config_test.go
package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadMissingFileReturnsDefaults(t *testing.T) {
	cfg, err := Load(filepath.Join(t.TempDir(), "does-not-exist.toml"))
	if err != nil {
		t.Fatalf("missing file must not error, got %v", err)
	}
	if cfg.ShedMax() != DefaultShedMax {
		t.Errorf("ShedMax() = %d, want default %d", cfg.ShedMax(), DefaultShedMax)
	}
	if cfg.Workspace.Root != "" {
		t.Errorf("Workspace.Root = %q, want empty", cfg.Workspace.Root)
	}
}

func TestLoadParsesValues(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	body := "[shed]\nmax = 7\n\n[workspace]\nroot = \"../wt\"\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if cfg.ShedMax() != 7 {
		t.Errorf("ShedMax() = %d, want 7", cfg.ShedMax())
	}
	if cfg.Workspace.Root != "../wt" {
		t.Errorf("Workspace.Root = %q, want ../wt", cfg.Workspace.Root)
	}
}

func TestShedMaxFallsBackWhenUnsetOrInvalid(t *testing.T) {
	var c Config // zero value
	if c.ShedMax() != DefaultShedMax {
		t.Errorf("zero ShedMax() = %d, want %d", c.ShedMax(), DefaultShedMax)
	}
	c.Shed.Max = -1
	if c.ShedMax() != DefaultShedMax {
		t.Errorf("negative ShedMax() = %d, want %d", c.ShedMax(), DefaultShedMax)
	}
}
```

- [ ] **Step 3: Run test to verify it fails**

Run: `go test ./internal/config/`
Expected: FAIL — `undefined: Load` / `undefined: DefaultShedMax`.

- [ ] **Step 4: Write minimal implementation**

```go
// internal/config/config.go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Weft Contributors

// Package config loads .weft/config.toml (spec §8). A missing file yields a
// zero-value Config; callers apply their own defaults via the accessors.
package config

import (
	"errors"
	"io/fs"

	"github.com/BurntSushi/toml"
)

// DefaultShedMax is the conservative wave-size cap when none is configured
// (spec §7: "Conservative default (≈3)").
const DefaultShedMax = 3

// Config is the project-local engine config (spec §8).
type Config struct {
	Shed struct {
		Max int `toml:"max"`
	} `toml:"shed"`
	Workspace struct {
		Root string `toml:"root"`
	} `toml:"workspace"`
}

// Load reads the TOML config at path. A missing file is not an error — it
// returns a zero-value Config. Malformed TOML returns the decode error.
func Load(path string) (Config, error) {
	var c Config
	_, err := toml.DecodeFile(path, &c)
	if errors.Is(err, fs.ErrNotExist) {
		return Config{}, nil
	}
	if err != nil {
		return Config{}, err
	}
	return c, nil
}

// ShedMax returns the configured wave cap, falling back to DefaultShedMax when
// unset or non-positive.
func (c Config) ShedMax() int {
	if c.Shed.Max < 1 {
		return DefaultShedMax
	}
	return c.Shed.Max
}
```

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./internal/config/`
Expected: PASS.

- [ ] **Step 6: Commit**

Run: `jj commit -m "feat(config): load .weft/config.toml (shed.max, workspace.root)"`

---

### Task 2: `internal/workspace` — identity & path derivation

**Files:**
- Create: `internal/workspace/workspace.go`
- Test: `internal/workspace/workspace_test.go`

- [ ] **Step 1: Write the failing test**

```go
// internal/workspace/workspace_test.go
package workspace

import "testing"

func TestSanitizeRoundTrip(t *testing.T) {
	for _, id := range []string{"weft-hjx", "weft-hjx.1", "weft-hjx.1.3"} {
		if got := Desanitize(Sanitize(id)); got != id {
			t.Errorf("round trip %q -> %q -> %q", id, Sanitize(id), got)
		}
	}
	if Sanitize("weft-hjx.1") != "weft-hjx__1" {
		t.Errorf("Sanitize dots wrong: %q", Sanitize("weft-hjx.1"))
	}
}

func TestResolveKind(t *testing.T) {
	bead, kind := Resolve("weft-hjx__1")
	if bead != "weft-hjx.1" || kind != "executor" {
		t.Errorf("executor: got (%q,%q)", bead, kind)
	}
	bead, kind = Resolve("weft-hjx__1-resolve")
	if bead != "weft-hjx.1" || kind != "resolve" {
		t.Errorf("resolve: got (%q,%q)", bead, kind)
	}
}

func TestRootAndPath(t *testing.T) {
	// Default: sibling of the repo dir.
	if got := Root("/a/b/weft", ""); got != "/a/b/weft_worktrees" {
		t.Errorf("default Root = %q", got)
	}
	// Relative override resolves against the repo root.
	if got := Root("/a/b/weft", "../wt"); got != "/a/b/wt" {
		t.Errorf("relative Root = %q", got)
	}
	// Absolute override used as-is.
	if got := Root("/a/b/weft", "/tmp/wt"); got != "/tmp/wt" {
		t.Errorf("absolute Root = %q", got)
	}
	if got := Path("/a/b/weft", "", "weft-hjx.1"); got != "/a/b/weft_worktrees/weft-hjx__1" {
		t.Errorf("Path = %q", got)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/workspace/`
Expected: FAIL — `undefined: Sanitize`.

- [ ] **Step 3: Write minimal implementation**

```go
// internal/workspace/workspace.go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Weft Contributors

// Package workspace derives jj workspace identity and on-disk paths for a bead
// (spec §3). The jj workspace name is a sanitized bead-id; the inverse mapping
// is the reaper's join key (spec §5).
package workspace

import (
	"path/filepath"
	"strings"
)

// resolveSuffix marks a conflict-resolution workspace (seam 4 §4.1). Bead-ids
// never end in "-resolve", so it is an unambiguous kind discriminant.
const resolveSuffix = "-resolve"

// Sanitize maps a bead-id to a jj-safe workspace name (dots → "__"), bijective.
func Sanitize(beadID string) string { return strings.ReplaceAll(beadID, ".", "__") }

// Desanitize is the inverse of Sanitize.
func Desanitize(name string) string { return strings.ReplaceAll(name, "__", ".") }

// Name returns the executor-workspace name for a bead.
func Name(beadID string) string { return Sanitize(beadID) }

// Resolve classifies a workspace name and returns its owning bead-id and kind
// ("executor" or "resolve").
func Resolve(name string) (beadID, kind string) {
	if strings.HasSuffix(name, resolveSuffix) {
		return Desanitize(strings.TrimSuffix(name, resolveSuffix)), "resolve"
	}
	return Desanitize(name), "executor"
}

// Root returns the sibling worktrees directory for the repo at jjRoot. A
// non-empty cfgRoot overrides the default ../<repo>_worktrees; a relative
// cfgRoot resolves against jjRoot.
func Root(jjRoot, cfgRoot string) string {
	if cfgRoot != "" {
		if filepath.IsAbs(cfgRoot) {
			return filepath.Clean(cfgRoot)
		}
		return filepath.Clean(filepath.Join(jjRoot, cfgRoot))
	}
	return filepath.Join(filepath.Dir(jjRoot), filepath.Base(jjRoot)+"_worktrees")
}

// Path returns the absolute workspace directory for a bead.
func Path(jjRoot, cfgRoot, beadID string) string {
	return filepath.Join(Root(jjRoot, cfgRoot), Name(beadID))
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/workspace/`
Expected: PASS.

- [ ] **Step 5: Commit**

Run: `jj commit -m "feat(workspace): bead-id sanitization + worktrees path derivation"`

---

### Task 3: Wire `Config` into `App`; default `shed form --max` from config

**Files:**
- Modify: `internal/cli/root.go` (the `App` struct)
- Modify: `cmd/weft/main.go` (load config)
- Modify: `internal/cli/shed.go` (`--max` default)
- Test: `internal/cli/shed_test.go` (add a config-default test)

- [ ] **Step 1: Write the failing test**

Append to `internal/cli/shed_test.go` (this test builds the root manually to inject `Config`, so add `"bytes"` to that file's import block — the seam-1 `shed_test.go` imports `strings`, `testing`, `exit`, `run` but not `bytes`):

```go
func TestShedFormMaxDefaultsFromConfig(t *testing.T) {
	fake := &scriptedRunner{res: run.Result{Stdout: `[]`, Code: 0}}
	app := &App{Runner: fake}
	app.Config.Shed.Max = 9 // config supplies the cap
	root := NewRootCmd(app)
	out := &bytes.Buffer{}
	root.SetOut(out)
	root.SetArgs([]string{"shed", "form", "--epic", "weft-hjx", "--json"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	// The wrapped `bd ready` call must carry --limit 9 (the config max).
	joined := strings.Join(fake.gotArgs, " ")
	if !strings.Contains(joined, "--limit 9") {
		t.Errorf("expected --limit 9 from config, got args: %v", fake.gotArgs)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/cli/ -run TestShedFormMaxDefaultsFromConfig`
Expected: FAIL — `app.Config` undefined (App has no Config field yet).

- [ ] **Step 3: Add the `Config` field to `App`**

In `internal/cli/root.go`, add the import and field:

```go
import (
	"github.com/seanb4t/weft/internal/config"
	"github.com/seanb4t/weft/internal/exit"
	"github.com/seanb4t/weft/internal/run"
	"github.com/spf13/cobra"
)

// App holds the engine's injectable dependencies.
type App struct {
	Runner run.Runner
	Config config.Config
}
```

- [ ] **Step 4: Default `--max` from config in `shed form`**

In `internal/cli/shed.go`, replace the `--max` flag registration line:

```go
	c.Flags().StringVar(&epic, "epic", "", "epic bead-id scoping the ready set (required)")
	// --max is the parallelism dial; its default comes from .weft/config.toml
	// [shed].max (falling back to config.DefaultShedMax). --max overrides it.
	c.Flags().IntVar(&max, "max", a.Config.ShedMax(), "max wave size (parallelism dial)")
	return c
```

(The enclosing function is already an `*App` method `(a *App) newShedFormCmd()`, so `a.Config` is in scope. The existing `--max < 1` guard stays.)

- [ ] **Step 5: Load config in `main.go`**

Replace `cmd/weft/main.go` body:

```go
// cmd/weft/main.go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Weft Contributors

package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/seanb4t/weft/internal/cli"
	"github.com/seanb4t/weft/internal/config"
	"github.com/seanb4t/weft/internal/exit"
	"github.com/seanb4t/weft/internal/run"
)

func main() {
	cfg, err := config.Load(filepath.Join(".weft", "config.toml"))
	if err != nil {
		fmt.Fprintln(os.Stderr, "weft: invalid .weft/config.toml:", err)
		os.Exit(exit.Code(exit.Hard(err)))
	}
	root := cli.NewRootCmd(&cli.App{Runner: run.Exec{}, Config: cfg})
	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(exit.Code(err))
	}
}
```

- [ ] **Step 6: Run tests to verify they pass**

Run: `go test ./internal/cli/ && go build ./cmd/weft`
Expected: PASS (the new test plus the existing `version`/`shed`/`ws` tests — the seam-1 `shed form` tests still pass because zero-config `App` yields `ShedMax() == 3 ≥` their fixture sizes), binary builds.

- [ ] **Step 7: Commit**

Run: `jj commit -m "feat(cli): App carries Config; shed form --max defaults from config"`

---

### Task 4: `ws add` / `ws forget` (+ the `jjRoot` helper)

**Files:**
- Modify: `internal/cli/ws.go`
- Test: `internal/cli/ws_test.go`

- [ ] **Step 1: Write the failing test**

Append to `internal/cli/ws_test.go`:

```go
import (
	"os"
	"path/filepath"
	// (existing imports: strings, testing, run)
)

func TestWsAddCreatesWorkspaceAtTrunk(t *testing.T) {
	fake := &scriptedRunner{res: run.Result{Stdout: "/repo/weft", Code: 0}}
	out, err := newTestCmd(fake, "ws", "add", "weft-hjx.1")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	// The last jj call is `workspace add <path> --name <sanitized> -r trunk()`.
	joined := strings.Join(fake.gotArgs, " ")
	if !strings.Contains(joined, "workspace add") ||
		!strings.Contains(joined, "weft_worktrees/weft-hjx__1") ||
		!strings.Contains(joined, "--name weft-hjx__1") ||
		!strings.Contains(joined, "trunk()") {
		t.Errorf("unexpected jj workspace add args: %v", fake.gotArgs)
	}
	if !strings.Contains(out.String(), "weft-hjx__1") {
		t.Errorf("output missing workspace name: %q", out.String())
	}
}

func TestWsForgetRemovesDir(t *testing.T) {
	root := t.TempDir()
	wsDir := filepath.Join(root+"_worktrees", "weft-hjx__1")
	if err := os.MkdirAll(wsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	fake := &scriptedRunner{res: run.Result{Stdout: root, Code: 0}}
	if _, err := newTestCmd(fake, "ws", "forget", "weft-hjx.1"); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if _, err := os.Stat(wsDir); !os.IsNotExist(err) {
		t.Errorf("workspace dir should be removed, stat err = %v", err)
	}
	if !strings.Contains(strings.Join(fake.gotArgs, " "), "workspace forget weft-hjx__1") {
		t.Errorf("expected jj workspace forget, got %v", fake.gotArgs)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/cli/ -run TestWsAdd`
Expected: FAIL — the `ws` command has no `add` subcommand.

- [ ] **Step 3: Write the implementation**

Replace `internal/cli/ws.go` with:

```go
// internal/cli/ws.go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Weft Contributors

package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/seanb4t/weft/internal/exit"
	"github.com/seanb4t/weft/internal/run"
	"github.com/seanb4t/weft/internal/workspace"
	"github.com/spf13/cobra"
)

func (a *App) newWsCmd() *cobra.Command {
	ws := &cobra.Command{Use: "ws", Short: "Workspace escape hatches (spec §4.3)"}
	ws.AddCommand(a.newWsListCmd(), a.newWsAddCmd(), a.newWsForgetCmd())
	return ws
}

func (a *App) newWsListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List jj workspaces",
		RunE: func(cmd *cobra.Command, _ []string) error {
			res, err := run.JJ(a.Runner, "workspace", "list", "-T", `name ++ "\n"`)
			if err != nil {
				return exit.Hardf("jj workspace list could not run: %v", err)
			}
			if res.Code != 0 {
				return exit.Hardf("jj workspace list failed: %s", strings.TrimSpace(res.Stderr))
			}
			names := []string{} // non-nil so empty output serializes as [] not null
			for _, ln := range strings.Split(strings.TrimSpace(res.Stdout), "\n") {
				if ln = strings.TrimSpace(ln); ln != "" {
					names = append(names, ln)
				}
			}
			data := map[string]any{"workspaces": names}
			return Emit(cmd, "ws.list", data, fmt.Sprintf("workspaces: %s", strings.Join(names, " ")))
		},
	}
}

func (a *App) newWsAddCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "add <bead-id>",
		Short: "Create a jj workspace for a bead on trunk()",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			bead := args[0]
			root, err := jjRoot(a.Runner)
			if err != nil {
				return err
			}
			path := workspace.Path(root, a.Config.Workspace.Root, bead)
			name := workspace.Name(bead)
			res, err := run.JJ(a.Runner, "workspace", "add", path, "--name", name, "-r", "trunk()")
			if err != nil {
				return exit.Hardf("jj workspace add could not run: %v", err)
			}
			if res.Code != 0 {
				return exit.Hardf("jj workspace add failed: %s", strings.TrimSpace(res.Stderr))
			}
			data := map[string]any{"bead": bead, "workspace": name, "path": path}
			return Emit(cmd, "ws.add", data, fmt.Sprintf("workspace %s at %s", name, path))
		},
	}
}

func (a *App) newWsForgetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "forget <bead-id>",
		Short: "Forget a bead's jj workspace and remove its directory",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			bead := args[0]
			root, err := jjRoot(a.Runner)
			if err != nil {
				return err
			}
			name := workspace.Name(bead)
			res, err := run.JJ(a.Runner, "workspace", "forget", name)
			if err != nil {
				return exit.Hardf("jj workspace forget could not run: %v", err)
			}
			if res.Code != 0 {
				return exit.Hardf("jj workspace forget failed: %s", strings.TrimSpace(res.Stderr))
			}
			path := workspace.Path(root, a.Config.Workspace.Root, bead)
			if err := os.RemoveAll(path); err != nil {
				return exit.Hardf("rm workspace dir %s: %v", path, err)
			}
			data := map[string]any{"bead": bead, "workspace": name}
			return Emit(cmd, "ws.forget", data, fmt.Sprintf("forgot workspace %s", name))
		},
	}
}

// jjRoot returns the repo root via `jj root`. Shared by ws/shed/reap verbs.
func jjRoot(r run.Runner) (string, error) {
	res, err := run.JJ(r, "root")
	if err != nil {
		return "", exit.Hardf("jj root could not run: %v", err)
	}
	if res.Code != 0 {
		return "", exit.Hardf("jj root failed: %s", strings.TrimSpace(res.Stderr))
	}
	return strings.TrimSpace(res.Stdout), nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/cli/ -run TestWs`
Expected: PASS (list, add, forget).

- [ ] **Step 5: Commit**

Run: `jj commit -m "feat(cli): ws add/forget verbs + jjRoot helper"`

---

### Task 5: `shed isolate` — status-first ordering invariant

**Files:**
- Modify: `internal/cli/shed.go`
- Test: `internal/cli/shed_test.go`

- [ ] **Step 1: Write the failing test** (introduces the shared `routeRunner` fake)

Append to `internal/cli/shed_test.go`:

```go
// routeRunner is a recording fake that dispatches each call through fn, so a
// test can return different results per command and assert call ordering.
type routeRunner struct {
	fn    func(name string, args []string) run.Result
	calls [][]string
}

func (r *routeRunner) Run(name string, args ...string) (run.Result, error) {
	r.calls = append(r.calls, append([]string{name}, args...))
	return r.fn(name, args), nil
}

func TestShedIsolateStatusBeforeWorkspaceAdd(t *testing.T) {
	fake := &routeRunner{fn: func(name string, args []string) run.Result {
		if name == "jj" && len(args) >= 2 && args[1] == "root" {
			return run.Result{Stdout: "/repo/weft", Code: 0}
		}
		return run.Result{Code: 0}
	}}
	out, err := newTestCmd(fake, "shed", "isolate", "weft-hjx.1.1", "--json")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	// Find the bd-update and jj-workspace-add call indices.
	upd, add := -1, -1
	for i, c := range fake.calls {
		j := strings.Join(c, " ")
		if strings.Contains(j, "bd update weft-hjx.1.1 --status in_progress") {
			upd = i
		}
		if strings.Contains(j, "workspace add") && strings.Contains(j, "weft-hjx__1__1") {
			add = i
		}
	}
	if upd < 0 || add < 0 {
		t.Fatalf("missing calls: upd=%d add=%d (%v)", upd, add, fake.calls)
	}
	if upd > add {
		t.Errorf("status-first violated: bd update (%d) must precede workspace add (%d)", upd, add)
	}
	if !strings.Contains(out.String(), "weft-hjx.1.1") {
		t.Errorf("output missing isolated bead: %q", out.String())
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/cli/ -run TestShedIsolate`
Expected: FAIL — `shed` has no `isolate` subcommand.

- [ ] **Step 3: Write the implementation**

In `internal/cli/shed.go`: add `"os"` is **not** needed here; add the `workspace` import, register the subcommand, and add the verb. Update `newShedCmd`:

```go
func (a *App) newShedCmd() *cobra.Command {
	shed := &cobra.Command{Use: "shed", Short: "Wave-level orchestration (spec §4.1)"}
	shed.AddCommand(a.newShedFormCmd(), a.newShedIsolateCmd())
	return shed
}
```

Add the imports `"github.com/seanb4t/weft/internal/workspace"` to the existing import block, then add:

```go
func (a *App) newShedIsolateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "isolate <bead-id>...",
		Short: "Isolate a wave: per bead set in_progress, then create its workspace on trunk()",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Trunk freshness (spec §7): fetch once per wave before isolating.
			if res, err := run.JJ(a.Runner, "git", "fetch"); err != nil {
				return exit.Hardf("jj git fetch could not run: %v", err)
			} else if res.Code != 0 {
				return exit.Hardf("jj git fetch failed: %s", strings.TrimSpace(res.Stderr))
			}
			root, err := jjRoot(a.Runner)
			if err != nil {
				return err
			}
			isolated := []string{}
			for _, bead := range args {
				// Status-first ordering invariant (spec §4): in_progress BEFORE
				// the workspace exists, so a crash never strands a reapable workspace.
				if res, err := run.BD(a.Runner, "update", bead, "--status", "in_progress"); err != nil {
					return exit.Hardf("bd update could not run: %v", err)
				} else if res.Code != 0 {
					return exit.Hardf("bd update %s failed: %s", bead, strings.TrimSpace(res.Stderr))
				}
				path := workspace.Path(root, a.Config.Workspace.Root, bead)
				name := workspace.Name(bead)
				if res, err := run.JJ(a.Runner, "workspace", "add", path, "--name", name, "-r", "trunk()"); err != nil {
					return exit.Hardf("jj workspace add could not run: %v", err)
				} else if res.Code != 0 {
					return exit.Hardf("jj workspace add %s failed: %s", bead, strings.TrimSpace(res.Stderr))
				}
				isolated = append(isolated, bead)
			}
			data := map[string]any{"wave": isolated}
			return Emit(cmd, "shed.isolate", data,
				fmt.Sprintf("isolated %d picks: %s", len(isolated), strings.Join(isolated, " ")))
		},
	}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/cli/ -run TestShedIsolate`
Expected: PASS.

- [ ] **Step 5: Commit**

Run: `jj commit -m "feat(cli): shed isolate (status-first ordering, trunk-fresh)"`

---

### Task 6: `shed cleanup` — per-wave teardown

**Files:**
- Modify: `internal/cli/shed.go`
- Test: `internal/cli/shed_test.go`

- [ ] **Step 1: Write the failing test**

Append to `internal/cli/shed_test.go`:

```go
func TestShedCleanupForgetsAndRemoves(t *testing.T) {
	root := t.TempDir()
	wsDir := filepath.Join(root+"_worktrees", "weft-hjx__1__2")
	if err := os.MkdirAll(wsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	fake := &routeRunner{fn: func(name string, args []string) run.Result {
		if name == "jj" && len(args) >= 2 && args[1] == "root" {
			return run.Result{Stdout: root, Code: 0}
		}
		return run.Result{Code: 0}
	}}
	if _, err := newTestCmd(fake, "shed", "cleanup", "weft-hjx.1.2"); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if _, err := os.Stat(wsDir); !os.IsNotExist(err) {
		t.Errorf("workspace dir should be removed, stat err = %v", err)
	}
	var forgot bool
	for _, c := range fake.calls {
		if strings.Contains(strings.Join(c, " "), "workspace forget weft-hjx__1__2") {
			forgot = true
		}
	}
	if !forgot {
		t.Errorf("expected jj workspace forget weft-hjx__1__2 in %v", fake.calls)
	}
}
```

(`os`, `path/filepath` must be in `shed_test.go`'s imports — add them.)

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/cli/ -run TestShedCleanup`
Expected: FAIL — `shed` has no `cleanup` subcommand.

- [ ] **Step 3: Write the implementation**

In `internal/cli/shed.go`, add `"os"` to the import block, register the subcommand in `newShedCmd`:

```go
func (a *App) newShedCmd() *cobra.Command {
	shed := &cobra.Command{Use: "shed", Short: "Wave-level orchestration (spec §4.1)"}
	shed.AddCommand(a.newShedFormCmd(), a.newShedIsolateCmd(), a.newShedCleanupCmd())
	return shed
}
```

Add the verb:

```go
func (a *App) newShedCleanupCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "cleanup <bead-id>...",
		Short: "Tear down a wave's workspaces (jj workspace forget + rm)",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := jjRoot(a.Runner)
			if err != nil {
				return err
			}
			cleaned := []string{}
			for _, bead := range args {
				name := workspace.Name(bead)
				if res, err := run.JJ(a.Runner, "workspace", "forget", name); err != nil {
					return exit.Hardf("jj workspace forget could not run: %v", err)
				} else if res.Code != 0 {
					return exit.Hardf("jj workspace forget %s failed: %s", bead, strings.TrimSpace(res.Stderr))
				}
				path := workspace.Path(root, a.Config.Workspace.Root, bead)
				if err := os.RemoveAll(path); err != nil {
					return exit.Hardf("rm workspace dir %s: %v", path, err)
				}
				cleaned = append(cleaned, bead)
			}
			data := map[string]any{"cleaned": cleaned}
			return Emit(cmd, "shed.cleanup", data, fmt.Sprintf("cleaned %d workspace(s)", len(cleaned)))
		},
	}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/cli/ -run TestShedCleanup`
Expected: PASS.

- [ ] **Step 5: Commit**

Run: `jj commit -m "feat(cli): shed cleanup (workspace forget + rm)"`

---

### Task 7: `weft reap` — bead-state reconciliation

**Files:**
- Create: `internal/cli/reap.go`
- Modify: `internal/cli/root.go` (register `reap`)
- Test: `internal/cli/reap_test.go`

- [ ] **Step 1: Write the failing test**

```go
// internal/cli/reap_test.go
package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/seanb4t/weft/internal/run"
)

func TestReapCollectsNonInProgressWorkspaces(t *testing.T) {
	root := t.TempDir()
	wtRoot := root + "_worktrees"
	// Two executor workspaces: .1.1 (closed → orphan) and .1.2 (in_progress → skip).
	orphanDir := filepath.Join(wtRoot, "weft-hjx__1__1")
	liveDir := filepath.Join(wtRoot, "weft-hjx__1__2")
	for _, d := range []string{orphanDir, liveDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	fake := &routeRunner{fn: func(name string, args []string) run.Result {
		j := strings.Join(append([]string{name}, args...), " ")
		switch {
		case name == "jj" && len(args) >= 2 && args[1] == "root":
			return run.Result{Stdout: root, Code: 0}
		case strings.Contains(j, "workspace list"):
			return run.Result{Stdout: "default\nweft-hjx__1__1\nweft-hjx__1__2\n", Code: 0}
		case strings.Contains(j, "bd show weft-hjx.1.1"):
			return run.Result{Stdout: `[{"status":"closed"}]`, Code: 0}
		case strings.Contains(j, "bd show weft-hjx.1.2"):
			return run.Result{Stdout: `[{"status":"in_progress"}]`, Code: 0}
		default:
			return run.Result{Code: 0} // forget, etc.
		}
	}}
	out, err := newTestCmd(fake, "reap", "--json")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	// Orphan dir reaped; live dir untouched; default never touched.
	if _, err := os.Stat(orphanDir); !os.IsNotExist(err) {
		t.Errorf("orphan workspace should be removed, stat err = %v", err)
	}
	if _, err := os.Stat(liveDir); err != nil {
		t.Errorf("in_progress workspace must be kept, stat err = %v", err)
	}
	if !strings.Contains(out.String(), "weft-hjx__1__1") || strings.Contains(out.String(), "weft-hjx__1__2") {
		t.Errorf("reaped set wrong: %q", out.String())
	}
	// `default` must never be queried or reaped.
	for _, c := range fake.calls {
		if strings.Contains(strings.Join(c, " "), "forget default") {
			t.Errorf("default workspace must never be reaped")
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/cli/ -run TestReap`
Expected: FAIL — `reap` command not registered.

- [ ] **Step 3: Write the implementation**

```go
// internal/cli/reap.go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Weft Contributors

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/seanb4t/weft/internal/exit"
	"github.com/seanb4t/weft/internal/run"
	"github.com/seanb4t/weft/internal/workspace"
	"github.com/spf13/cobra"
)

func (a *App) newReapCmd() *cobra.Command {
	var epic string
	c := &cobra.Command{
		Use:   "reap",
		Short: "Reconcile jj workspaces against bead state; reap orphans (spec §5)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			root, err := jjRoot(a.Runner)
			if err != nil {
				return err
			}
			res, err := run.JJ(a.Runner, "workspace", "list", "-T", `name ++ "\n"`)
			if err != nil {
				return exit.Hardf("jj workspace list could not run: %v", err)
			}
			if res.Code != 0 {
				return exit.Hardf("jj workspace list failed: %s", strings.TrimSpace(res.Stderr))
			}
			wtRoot := workspace.Root(root, a.Config.Workspace.Root)
			reaped := []string{}
			for _, ln := range strings.Split(strings.TrimSpace(res.Stdout), "\n") {
				name := strings.TrimSpace(ln)
				if name == "" || name == "default" {
					continue // never reap the orchestrator's own workspace
				}
				bead, _ := workspace.Resolve(name) // kind-aware: strips -resolve, desanitizes
				// --epic scope: bead-ids are hierarchical, so a descendant of the
				// epic has it as a dotted prefix (e.g. weft-hjx.1.3 under weft-hjx.1).
				if epic != "" && bead != epic && !strings.HasPrefix(bead, epic+".") {
					continue
				}
				status, err := beadStatus(a.Runner, bead)
				if err != nil {
					return err
				}
				// v1: the executor_live guard (spec §5/§10) is deferred — an
				// in_progress bead is assumed in-flight and kept. Everything else
				// (closed/open/missing) is an orphan and reaped (forget never loses
				// sealed work, spec §2).
				if status == "in_progress" {
					continue
				}
				if res, err := run.JJ(a.Runner, "workspace", "forget", name); err != nil {
					return exit.Hardf("jj workspace forget could not run: %v", err)
				} else if res.Code != 0 {
					return exit.Hardf("jj workspace forget %s failed: %s", name, strings.TrimSpace(res.Stderr))
				}
				if err := os.RemoveAll(filepath.Join(wtRoot, name)); err != nil {
					return exit.Hardf("rm workspace dir %s: %v", name, err)
				}
				reaped = append(reaped, name)
			}
			data := map[string]any{"reaped": reaped}
			return Emit(cmd, "reap", data, fmt.Sprintf("reaped %d orphan workspace(s)", len(reaped)))
		},
	}
	c.Flags().StringVar(&epic, "epic", "", "scope reconciliation to descendants of this epic")
	return c
}

// beadStatus returns the bead's status. A missing bead (bd show exits non-zero
// or returns nothing) yields "" — treated as an orphan by the caller.
func beadStatus(r run.Runner, bead string) (string, error) {
	res, err := run.BD(r, "show", bead, "--json")
	if err != nil {
		return "", exit.Hardf("bd show could not run: %v", err)
	}
	if res.Code != 0 {
		return "", nil // bead gone → its workspace is an orphan
	}
	var arr []struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal([]byte(res.Stdout), &arr); err != nil || len(arr) == 0 {
		return "", nil
	}
	return arr[0].Status, nil
}
```

Register it in `internal/cli/root.go`'s `NewRootCmd`:

```go
	root.AddCommand(newVersionCmd())
	root.AddCommand(app.newShedCmd())
	root.AddCommand(app.newWsCmd())
	root.AddCommand(app.newReapCmd())
	return root
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/cli/ -run TestReap`
Expected: PASS.

- [ ] **Step 5: Run the full suite + vet**

Run: `go vet ./... && go test ./...`
Expected: PASS across all packages; no vet complaints.

- [ ] **Step 6: Commit**

Run: `jj commit -m "feat(cli): weft reap — bead-state reconciliation of orphan workspaces"`

---

## Done criteria

- `go test ./...` passes; `go vet ./...` clean; `go build ./cmd/weft` produces the binary.
- `weft ws add <bead>` / `weft ws forget <bead>` create/tear down `../<repo>_worktrees/<sanitized-bead-id>`.
- `weft shed isolate <bead>...` sets each bead `in_progress` **before** creating its workspace (status-first), fetching once up front.
- `weft shed cleanup <bead>...` forgets + removes each wave member's workspace.
- `weft reap [--epic E]` removes every workspace whose owning bead is not `in_progress` (the `pick land` → cleanup-missed safety net), never touching `default`.
- `weft shed form` honors `[shed].max` from `.weft/config.toml` (default 3).

## Out of scope (follow-on)

- **jj-stale handling** (spec §6 / §6.1 tiered policy) — `jj workspace update-stale` vs abandon+re-isolate. Design-only this round; it needs `jj st` stale detection wiring and interacts with `shed integrate` (a seam-1 coarse verb not yet built). Defer to the integration plan.
- **`executor_live` liveness guard** (spec §5/§10) — v1 `reap` keeps all `in_progress` workspaces; reaping a *crashed* in_progress executor needs the liveness mechanism.
- **Parallel `jj workspace add`** (spec §7/§10) — isolation is serial; concurrency is gated on confirming jj concurrent-add safety.
- **Config precedence** (flag > env > file > default) and the `--epic` scope via true ancestry rather than the dotted-prefix heuristic — spec §10. (v1 note: `main.go` loads `.weft/config.toml` relative to the process CWD, so `weft` must be invoked from the repo root; jj-root-relative resolution is part of the deferred precedence work.)
- Seam-1 coarse verbs that consume this layer (`shed integrate`, `pick *`, `finish`, `resume`) — separate plans.
<!-- adr-capture: sha256=6eb5bdccb923e714; session=cli; ts=2026-06-02T23:22:14Z; adrs= -->
