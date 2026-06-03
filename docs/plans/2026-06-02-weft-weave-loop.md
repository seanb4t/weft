<!--
  ~ SPDX-License-Identifier: Apache-2.0
  ~ Copyright 2026 Weft Contributors
-->

# Weft Weave Loop (Seam 1 coarse verbs) — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement the seam-1 weave loop on the merged foundation + workspace layer — `pick seal/verify/land/redo`, `shed integrate` (the dep-ordered rebase stack), and read-only `resume` — so a wave can be sealed, integrated, verified, landed, recovered, and observed.

**Architecture:** New cobra verbs in `internal/cli` wrap `jj`/`bd` through the existing injectable `run.Runner` (ADR `weft-re2`). A shared `bead.go` helper reads bead facts (`showBead`) and the `jj-change:<id>` spine label (`changeOf`, spec §5.1). `shed integrate` rebases each sealed change into a linear stack (`jj rebase -s … -o … --skip-emptied`) ordered by the bead-id-lexicographic tiebreaker, surfacing first-class conflicts as data (resolution is seam 4). `resume` is a read-only projection — a hard invariant.

**Tech Stack:** Go 1.26, `github.com/spf13/cobra` v1.9.x, stdlib `sort`/`strings`/`encoding/json`. Subprocesses: `jj`, `bd`.

**Spec:** `docs/seams/01-command-surface.md` (READY). Verbs §4.1 (`shed integrate`), §4.2 (`pick seal/verify/land/redo`), §4.5 (`resume`); the spine §5.1.

**Out of scope (follow-on ship plan):** `finish open/reconcile` (PR/gh), `shed abandon`, `shed status`; the seam-4 `conflict` verbs; promoting integrate's `conflicts` to a top-level envelope field with `paths`/`lowest_ancestor` enrichment (seam-4 era). `pick verify`'s gate is a configured command this round (no per-bead gate inference). `pick redo` reopens via `bd update --status open` uniformly (the `bd reopen`-for-closed nuance is a refinement).

---

## File Structure

| File | Responsibility |
|---|---|
| `internal/cli/bead.go` | `beadInfo` + `showBead` (one bd-show parse) + `changeOf` (the `jj-change:` spine label) + the label-prefix const. |
| `internal/cli/pick.go` | `weft pick` group: `seal`, `verify`, `land`, `redo`. |
| `internal/cli/shed.go` (modify) | Add `shed integrate`. |
| `internal/cli/resume.go` | `weft resume` — read-only state projection. |
| `internal/cli/root.go` (modify) | Register `pick` and `resume`. |
| `internal/config/config.go` (modify) | Add `[verify].command`. |

Tests live beside each file. `bead.go` (Task 1) is the shared helper; the verbs build on it and on the seam-1 `App`/`Emit`/`run` plumbing and the `routeRunner` test fake (defined in `shed_test.go`).

---

### Task 1: `internal/cli/bead.go` — shared bead facts + spine label

**Files:**
- Create: `internal/cli/bead.go`
- Test: `internal/cli/bead_test.go`

- [ ] **Step 1: Write the failing test**

```go
// internal/cli/bead_test.go
package cli

import (
	"testing"

	"github.com/seanb4t/weft/internal/run"
)

func TestShowBeadParsesFields(t *testing.T) {
	r := &routeRunner{fn: func(_ string, _ []string) run.Result {
		return run.Result{Stdout: `[{"title":"T","status":"in_progress","labels":["phase:run","jj-change:abc123"]}]`, Code: 0}
	}}
	info, err := showBead(r, "weft-hjx.1.1")
	if err != nil {
		t.Fatalf("showBead error: %v", err)
	}
	if info.Title != "T" || info.Status != "in_progress" {
		t.Errorf("fields = %+v", info)
	}
}

func TestChangeOfReadsSpineLabel(t *testing.T) {
	withLabels := func(labels string) run.Runner {
		return &routeRunner{fn: func(_ string, _ []string) run.Result {
			return run.Result{Stdout: `[{"title":"T","status":"in_progress","labels":` + labels + `}]`, Code: 0}
		}}
	}
	got, err := changeOf(withLabels(`["jj-change:abc123","phase:run"]`), "b")
	if err != nil || got != "abc123" {
		t.Fatalf("changeOf = %q, %v", got, err)
	}
	got, err = changeOf(withLabels(`["phase:run"]`), "b")
	if err != nil || got != "" {
		t.Fatalf("unsealed changeOf = %q, %v (want empty)", got, err)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/cli/ -run 'TestShowBead|TestChangeOf'`
Expected: FAIL — `undefined: showBead` / `undefined: changeOf`.

- [ ] **Step 3: Write the implementation**

```go
// internal/cli/bead.go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Weft Contributors

package cli

import (
	"encoding/json"
	"strings"

	"github.com/seanb4t/weft/internal/exit"
	"github.com/seanb4t/weft/internal/run"
)

// jjChangeLabelPrefix pins a bead's jj change-id — the bead↔change spine
// (spec §5.1). `pick seal` writes it; integrate/land/redo/resume read it.
const jjChangeLabelPrefix = "jj-change:"

// beadInfo is the subset of `bd show --json` the weave verbs need.
type beadInfo struct {
	Title  string   `json:"title"`
	Status string   `json:"status"`
	Labels []string `json:"labels"`
}

// showBead reads one bead's facts. `bd show --json` returns a single-element
// array.
func showBead(r run.Runner, bead string) (beadInfo, error) {
	res, err := run.BD(r, "show", bead, "--json")
	if err != nil {
		return beadInfo{}, exit.Hardf("bd show could not run: %v", err)
	}
	if res.Code != 0 {
		return beadInfo{}, exit.Hardf("bd show %s failed: %s", bead, strings.TrimSpace(res.Stderr))
	}
	var arr []beadInfo
	if err := json.Unmarshal([]byte(res.Stdout), &arr); err != nil || len(arr) == 0 {
		return beadInfo{}, exit.Hardf("parse bd show json for %s", bead)
	}
	return arr[0], nil
}

// changeOf returns the bead's pinned jj change-id (from its jj-change:<id>
// label), or "" if the bead has not been sealed yet.
func changeOf(r run.Runner, bead string) (string, error) {
	info, err := showBead(r, bead)
	if err != nil {
		return "", err
	}
	return changeFromLabels(info.Labels), nil
}

// changeFromLabels extracts the jj-change:<id> value from a label set, or "".
func changeFromLabels(labels []string) string {
	for _, l := range labels {
		if strings.HasPrefix(l, jjChangeLabelPrefix) {
			return strings.TrimPrefix(l, jjChangeLabelPrefix)
		}
	}
	return ""
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/cli/ -run 'TestShowBead|TestChangeOf'`
Expected: PASS.

- [ ] **Step 5: Commit**

Run: `jj commit -m "feat(cli): shared bead facts + jj-change spine helper"`

---

### Task 2: `pick seal` — jj commit + pin the spine label

**Files:**
- Create: `internal/cli/pick.go`
- Modify: `internal/cli/root.go` (register `pick`)
- Test: `internal/cli/pick_test.go`

- [ ] **Step 1: Write the failing test**

```go
// internal/cli/pick_test.go
package cli

import (
	"strings"
	"testing"

	"github.com/seanb4t/weft/internal/run"
)

func TestPickSealCommitsAndLabels(t *testing.T) {
	r := &routeRunner{fn: func(name string, args []string) run.Result {
		j := strings.Join(append([]string{name}, args...), " ")
		switch {
		case strings.Contains(j, "bd show"):
			return run.Result{Stdout: `[{"title":"Add X","status":"in_progress","labels":[]}]`, Code: 0}
		case strings.Contains(j, "log -r @-"):
			return run.Result{Stdout: "ch4ng3id000\n", Code: 0}
		default: // jj commit, bd update --add-label
			return run.Result{Code: 0}
		}
	}}
	out, err := newTestCmd(r, "pick", "seal", "weft-hjx.1.1", "--json")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	var sawCommit, sawLabel bool
	for _, c := range r.calls {
		j := strings.Join(c, " ")
		if strings.Contains(j, `jj --no-pager commit -m feat(weft-hjx.1.1): Add X`) {
			sawCommit = true
		}
		if strings.Contains(j, "bd update weft-hjx.1.1 --add-label jj-change:ch4ng3id000") {
			sawLabel = true
		}
	}
	if !sawCommit {
		t.Errorf("expected conventional-commit jj commit; calls=%v", r.calls)
	}
	if !sawLabel {
		t.Errorf("expected jj-change label write; calls=%v", r.calls)
	}
	if !strings.Contains(out.String(), "ch4ng3id000") {
		t.Errorf("output missing change-id: %q", out.String())
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/cli/ -run TestPickSeal`
Expected: FAIL — `pick` command not registered.

- [ ] **Step 3: Write `pick.go` with the `seal` verb**

```go
// internal/cli/pick.go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Weft Contributors

package cli

import (
	"fmt"
	"strings"

	"github.com/seanb4t/weft/internal/exit"
	"github.com/seanb4t/weft/internal/run"
	"github.com/spf13/cobra"
)

func (a *App) newPickCmd() *cobra.Command {
	pick := &cobra.Command{Use: "pick", Short: "Bead-level pick lifecycle (spec §4.2)"}
	pick.AddCommand(a.newPickSealCmd())
	return pick
}

func (a *App) newPickSealCmd() *cobra.Command {
	var ctype string
	c := &cobra.Command{
		Use:   "seal <bead>",
		Short: "Seal the executor's work: jj commit + pin the jj-change spine label",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			bead := args[0]
			info, err := showBead(a.Runner, bead)
			if err != nil {
				return err
			}
			msg := fmt.Sprintf("%s(%s): %s", ctype, bead, info.Title)
			if res, err := run.JJ(a.Runner, "commit", "-m", msg); err != nil {
				return exit.Hardf("jj commit could not run: %v", err)
			} else if res.Code != 0 {
				return exit.Hardf("jj commit failed: %s", strings.TrimSpace(res.Stderr))
			}
			// The sealed change is now @- (jj commit describes @ and opens a new empty @).
			res, err := run.JJ(a.Runner, "log", "-r", "@-", "--no-graph", "-T", "change_id.short(12)")
			if err != nil {
				return exit.Hardf("read sealed change-id could not run: %v", err)
			}
			if res.Code != 0 {
				return exit.Hardf("read sealed change-id failed: %s", strings.TrimSpace(res.Stderr))
			}
			change := strings.TrimSpace(res.Stdout)
			if res, err := run.BD(a.Runner, "update", bead, "--add-label", jjChangeLabelPrefix+change); err != nil {
				return exit.Hardf("bd add-label could not run: %v", err)
			} else if res.Code != 0 {
				return exit.Hardf("bd add-label failed: %s", strings.TrimSpace(res.Stderr))
			}
			data := map[string]any{"bead": bead, "change": change}
			return Emit(cmd, "pick.seal", data, fmt.Sprintf("sealed %s as '%s' (change %s)", bead, msg, change))
		},
	}
	c.Flags().StringVar(&ctype, "type", "feat", "conventional-commit type for the message")
	return c
}
```

- [ ] **Step 4: Register `pick` in `root.go`**

In `internal/cli/root.go`'s `NewRootCmd`, add the line:

```go
	root.AddCommand(newVersionCmd())
	root.AddCommand(app.newShedCmd())
	root.AddCommand(app.newWsCmd())
	root.AddCommand(app.newReapCmd())
	root.AddCommand(app.newPickCmd())
	return root
```

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./internal/cli/ -run TestPickSeal`
Expected: PASS.

- [ ] **Step 6: Commit**

Run: `jj commit -m "feat(cli): pick seal — commit + pin the jj-change label"`

---

### Task 3: `pick verify` — run the configured gate (verdict is data)

**Files:**
- Modify: `internal/config/config.go` (add `[verify].command`)
- Modify: `internal/cli/pick.go` (add `verify`)
- Test: `internal/cli/pick_test.go`, `internal/config/config_test.go`

- [ ] **Step 1: Write the failing config test**

Append to `internal/config/config_test.go`:

```go
func TestLoadParsesVerifyCommand(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte("[verify]\ncommand = \"go test ./...\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if cfg.Verify.Command != "go test ./..." {
		t.Errorf("Verify.Command = %q", cfg.Verify.Command)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/config/ -run TestLoadParsesVerify`
Expected: FAIL — `cfg.Verify` undefined.

- [ ] **Step 3: Add `Verify` to `Config`**

In `internal/config/config.go`, extend the `Config` struct:

```go
type Config struct {
	Shed struct {
		Max int `toml:"max"`
	} `toml:"shed"`
	Workspace struct {
		Root string `toml:"root"`
	} `toml:"workspace"`
	Verify struct {
		Command string `toml:"command"`
	} `toml:"verify"`
}
```

- [ ] **Step 4: Write the failing verb test**

Append to `internal/cli/pick_test.go`:

```go
func TestPickVerifyVerdictIsData(t *testing.T) {
	// Gate exits non-zero → pass:false, but the VERB still exits 0 (verdict is data).
	r := &routeRunner{fn: func(name string, args []string) run.Result {
		if name == "sh" {
			return run.Result{Code: 1} // gate fails
		}
		return run.Result{Code: 0}
	}}
	app := &App{Runner: r}
	app.Config.Verify.Command = "false"
	root := NewRootCmd(app)
	out := &bytes.Buffer{}
	root.SetOut(out)
	root.SetArgs([]string{"pick", "verify", "weft-hjx.1.1", "--json"})
	if err := root.Execute(); err != nil {
		t.Fatalf("verify must exit 0 even when the gate fails, got %v", err)
	}
	if !strings.Contains(out.String(), `"pass": false`) {
		t.Errorf("expected pass:false in data: %q", out.String())
	}
}

func TestPickVerifyRequiresConfiguredGate(t *testing.T) {
	app := &App{Runner: &routeRunner{fn: func(string, []string) run.Result { return run.Result{} }}}
	root := NewRootCmd(app) // no Verify.Command
	root.SetArgs([]string{"pick", "verify", "weft-hjx.1.1"})
	if got := exit.Code(root.Execute()); got != 1 {
		t.Fatalf("missing gate must be exit 1, got %d", got)
	}
}
```

(Add `bytes` and `github.com/seanb4t/weft/internal/exit` to `pick_test.go`'s imports.)

- [ ] **Step 5: Add the `verify` verb to `pick.go`**

Register it in `newPickCmd`:

```go
func (a *App) newPickCmd() *cobra.Command {
	pick := &cobra.Command{Use: "pick", Short: "Bead-level pick lifecycle (spec §4.2)"}
	pick.AddCommand(a.newPickSealCmd(), a.newPickVerifyCmd())
	return pick
}
```

Add the verb:

```go
func (a *App) newPickVerifyCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "verify <bead>",
		Short: "Run the configured verify gate; the verdict is data (spec §4.2)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			bead := args[0]
			gate := strings.TrimSpace(a.Config.Verify.Command)
			if gate == "" {
				return exit.Invocationf("no verify gate configured ([verify].command in .weft/config.toml)")
			}
			// The engine ran the gate fine, so this verb exits 0 regardless of the
			// gate's own exit — the pass/fail verdict is DATA (spec §3).
			res, err := a.Runner.Run("sh", "-c", gate)
			if err != nil {
				return exit.Hardf("verify gate could not run: %v", err)
			}
			pass := res.Code == 0
			data := map[string]any{"bead": bead, "pass": pass}
			verdict := "FAIL"
			if pass {
				verdict = "PASS"
			}
			return Emit(cmd, "pick.verify", data, fmt.Sprintf("verify %s: %s", bead, verdict))
		},
	}
}
```

- [ ] **Step 6: Run tests to verify they pass**

Run: `go test ./internal/config/ ./internal/cli/ -run 'Verify'`
Expected: PASS.

- [ ] **Step 7: Commit**

Run: `jj commit -m "feat(cli): pick verify — configured gate, verdict as data"`

---

### Task 4: `shed integrate` — dep-ordered rebase stack

**Files:**
- Modify: `internal/cli/shed.go`
- Test: `internal/cli/shed_test.go`

- [ ] **Step 1: Write the failing test**

Append to `internal/cli/shed_test.go`:

```go
func TestShedIntegrateBuildsLinearStack(t *testing.T) {
	// Two sealed picks; integrate orders them lexicographically and rebases each
	// onto the previous tip, then reports stack + (no) conflicts.
	r := &routeRunner{fn: func(name string, args []string) run.Result {
		j := strings.Join(append([]string{name}, args...), " ")
		switch {
		case strings.Contains(j, "bd show weft-hjx.1.2"):
			return run.Result{Stdout: `[{"title":"b","status":"in_progress","labels":["jj-change:chB"]}]`, Code: 0}
		case strings.Contains(j, "bd show weft-hjx.1.1"):
			return run.Result{Stdout: `[{"title":"a","status":"in_progress","labels":["jj-change:chA"]}]`, Code: 0}
		case strings.Contains(j, "log -r conflicts()"):
			return run.Result{Stdout: "", Code: 0} // clean
		default: // jj rebase
			return run.Result{Code: 0}
		}
	}}
	// Pass members out of lexical order to prove sorting.
	out, err := newTestCmd(r, "shed", "integrate", "weft-hjx.1.2", "weft-hjx.1.1", "--json")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	// First rebase: chA onto trunk(); second: chB onto chA (lexicographic order).
	var rebases [][]string
	for _, c := range r.calls {
		if len(c) >= 2 && c[0] == "jj" && contains(c, "rebase") {
			rebases = append(rebases, c)
		}
	}
	if len(rebases) != 2 {
		t.Fatalf("want 2 rebases, got %d: %v", len(rebases), rebases)
	}
	if !contains(rebases[0], "chA") || !contains(rebases[0], "trunk()") {
		t.Errorf("first rebase should be chA onto trunk(): %v", rebases[0])
	}
	if !contains(rebases[1], "chB") || !contains(rebases[1], "chA") {
		t.Errorf("second rebase should be chB onto chA: %v", rebases[1])
	}
	if !strings.Contains(out.String(), "chA") || !strings.Contains(out.String(), "chB") {
		t.Errorf("stack missing in output: %q", out.String())
	}
}

func contains(ss []string, want string) bool {
	for _, s := range ss {
		if strings.Contains(s, want) {
			return true
		}
	}
	return false
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/cli/ -run TestShedIntegrate`
Expected: FAIL — `shed` has no `integrate` subcommand.

- [ ] **Step 3: Write the implementation**

In `internal/cli/shed.go`: add `"sort"` to the import block, register the subcommand in `newShedCmd`:

```go
func (a *App) newShedCmd() *cobra.Command {
	shed := &cobra.Command{Use: "shed", Short: "Wave-level orchestration (spec §4.1)"}
	shed.AddCommand(a.newShedFormCmd(), a.newShedIsolateCmd(), a.newShedCleanupCmd(), a.newShedIntegrateCmd())
	return shed
}
```

Add the verb:

```go
func (a *App) newShedIntegrateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "integrate <bead-id>...",
		Short: "Rebase the wave's sealed changes into a dep-ordered linear stack",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Wave members are mutually independent (spec §4.1), so the dep graph
			// imposes no intra-wave order; the deterministic tiebreaker is bead-id
			// lexicographic.
			beads := append([]string{}, args...)
			sort.Strings(beads)

			// Resolve each pick's sealed change-id (the spine).
			changes := make([]string, 0, len(beads))
			for _, b := range beads {
				ch, err := changeOf(a.Runner, b)
				if err != nil {
					return err
				}
				if ch == "" {
					return exit.Invocationf("bead %s has no jj-change label (not sealed)", b)
				}
				changes = append(changes, ch)
			}

			// Rebase into a linear stack: trunk() <- chA <- chB <- ...
			prev := "trunk()"
			stack := make([]string, 0, len(changes))
			for _, ch := range changes {
				if res, err := run.JJ(a.Runner, "rebase", "-s", ch, "-o", prev, "--skip-emptied"); err != nil {
					return exit.Hardf("jj rebase could not run: %v", err)
				} else if res.Code != 0 {
					return exit.Hardf("jj rebase %s failed: %s", ch, strings.TrimSpace(res.Stderr))
				}
				prev = ch
				stack = append(stack, ch)
			}

			// First-class conflicts are surfaced as data; resolution is seam 4.
			res, err := run.JJ(a.Runner, "log", "-r", "conflicts()", "--no-graph", "-T", `change_id.short(12) ++ "\n"`)
			if err != nil {
				return exit.Hardf("jj log conflicts() could not run: %v", err)
			}
			if res.Code != 0 {
				return exit.Hardf("jj log conflicts() failed: %s", strings.TrimSpace(res.Stderr))
			}
			conflicts := []string{}
			for _, ln := range strings.Split(strings.TrimSpace(res.Stdout), "\n") {
				if ln = strings.TrimSpace(ln); ln != "" {
					conflicts = append(conflicts, ln)
				}
			}

			data := map[string]any{"stack": stack, "conflicts": conflicts}
			text := fmt.Sprintf("integrated %d picks: %s", len(stack), strings.Join(stack, " -> "))
			if len(conflicts) > 0 {
				text += fmt.Sprintf("  [%d conflicted: %s]", len(conflicts), strings.Join(conflicts, " "))
			}
			return Emit(cmd, "shed.integrate", data, text)
		},
	}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/cli/ -run TestShedIntegrate`
Expected: PASS.

- [ ] **Step 5: Commit**

Run: `jj commit -m "feat(cli): shed integrate — dep-ordered rebase stack + conflicts"`

---

### Task 5: `pick land` — assert conflict-free, then close

**Files:**
- Modify: `internal/cli/pick.go`
- Test: `internal/cli/pick_test.go`

- [ ] **Step 1: Write the failing test**

Append to `internal/cli/pick_test.go`:

```go
func TestPickLandRefusesConflictedChange(t *testing.T) {
	r := &routeRunner{fn: func(name string, args []string) run.Result {
		j := strings.Join(append([]string{name}, args...), " ")
		switch {
		case strings.Contains(j, "bd show"):
			return run.Result{Stdout: `[{"title":"t","status":"in_progress","labels":["jj-change:chX"]}]`, Code: 0}
		case strings.Contains(j, "conflicts()"):
			return run.Result{Stdout: "chX\n", Code: 0} // chX IS conflicted
		default:
			return run.Result{Code: 0}
		}
	}}
	if got := exit.Code(runRoot(r, "pick", "land", "weft-hjx.1.1")); got != 1 {
		t.Fatalf("landing a conflicted change must be exit 1, got %d", got)
	}
	for _, c := range r.calls {
		if strings.Contains(strings.Join(c, " "), "bd close") {
			t.Fatalf("must NOT bd close a conflicted pick: %v", r.calls)
		}
	}
}

func TestPickLandClosesCleanChange(t *testing.T) {
	r := &routeRunner{fn: func(name string, args []string) run.Result {
		j := strings.Join(append([]string{name}, args...), " ")
		switch {
		case strings.Contains(j, "bd show"):
			return run.Result{Stdout: `[{"title":"t","status":"in_progress","labels":["jj-change:chX"]}]`, Code: 0}
		case strings.Contains(j, "conflicts()"):
			return run.Result{Stdout: "", Code: 0} // clean
		default:
			return run.Result{Code: 0}
		}
	}}
	if err := runRoot(r, "pick", "land", "weft-hjx.1.1"); err != nil {
		t.Fatalf("clean land error: %v", err)
	}
	if !contains2(r.calls, "bd close weft-hjx.1.1 --suggest-next") {
		t.Errorf("expected bd close --suggest-next: %v", r.calls)
	}
}

// runRoot executes a command with a fresh root over the given runner.
func runRoot(r run.Runner, args ...string) error {
	root := NewRootCmd(&App{Runner: r})
	root.SetArgs(args)
	return root.Execute()
}

func contains2(calls [][]string, want string) bool {
	for _, c := range calls {
		if strings.Contains(strings.Join(c, " "), want) {
			return true
		}
	}
	return false
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/cli/ -run TestPickLand`
Expected: FAIL — `pick` has no `land` subcommand.

- [ ] **Step 3: Add the `land` verb**

Register it in `newPickCmd` (now `seal`, `verify`, `land`):

```go
	pick.AddCommand(a.newPickSealCmd(), a.newPickVerifyCmd(), a.newPickLandCmd())
```

Add the verb:

```go
func (a *App) newPickLandCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "land <bead>",
		Short: "Land a pick: assert its change is conflict-free, then bd close (spec §4.2)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			bead := args[0]
			change, err := changeOf(a.Runner, bead)
			if err != nil {
				return err
			}
			if change == "" {
				return exit.Invocationf("bead %s has no jj-change label (not sealed/integrated)", bead)
			}
			// Never land a conflicted change (seam 4 §6): the gate is concrete —
			// the change must not be in conflicts().
			res, err := run.JJ(a.Runner, "log", "-r", "conflicts() & "+change, "--no-graph", "-T", `change_id.short(12) ++ "\n"`)
			if err != nil {
				return exit.Hardf("jj conflicts check could not run: %v", err)
			}
			if res.Code != 0 {
				return exit.Hardf("jj conflicts check failed: %s", strings.TrimSpace(res.Stderr))
			}
			if strings.TrimSpace(res.Stdout) != "" {
				return exit.Invocationf("refusing to land %s: change %s is conflicted (resolve first)", bead, change)
			}
			if res, err := run.BD(a.Runner, "close", bead, "--suggest-next"); err != nil {
				return exit.Hardf("bd close could not run: %v", err)
			} else if res.Code != 0 {
				return exit.Hardf("bd close %s failed: %s", bead, strings.TrimSpace(res.Stderr))
			}
			data := map[string]any{"bead": bead, "change": change}
			return Emit(cmd, "pick.land", data, fmt.Sprintf("landed %s (change %s)", bead, change))
		},
	}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/cli/ -run TestPickLand`
Expected: PASS.

- [ ] **Step 5: Commit**

Run: `jj commit -m "feat(cli): pick land — conflict-free gate then close"`

---

### Task 6: `pick redo` — abandon the change and reopen

**Files:**
- Modify: `internal/cli/pick.go`
- Test: `internal/cli/pick_test.go`

- [ ] **Step 1: Write the failing test**

Append to `internal/cli/pick_test.go`:

```go
func TestPickRedoAbandonsAndReopens(t *testing.T) {
	r := &routeRunner{fn: func(name string, args []string) run.Result {
		if strings.Contains(strings.Join(append([]string{name}, args...), " "), "bd show") {
			return run.Result{Stdout: `[{"title":"t","status":"in_progress","labels":["jj-change:chX"]}]`, Code: 0}
		}
		return run.Result{Code: 0}
	}}
	if err := runRoot(r, "pick", "redo", "weft-hjx.1.1"); err != nil {
		t.Fatalf("redo error: %v", err)
	}
	if !contains2(r.calls, "jj --no-pager abandon chX") {
		t.Errorf("expected jj abandon chX: %v", r.calls)
	}
	if !contains2(r.calls, "bd update weft-hjx.1.1 --status open") {
		t.Errorf("expected bd update --status open: %v", r.calls)
	}
}

func TestPickRedoSkipsAbandonWhenUnsealed(t *testing.T) {
	r := &routeRunner{fn: func(name string, args []string) run.Result {
		if strings.Contains(strings.Join(append([]string{name}, args...), " "), "bd show") {
			return run.Result{Stdout: `[{"title":"t","status":"in_progress","labels":[]}]`, Code: 0} // no jj-change
		}
		return run.Result{Code: 0}
	}}
	if err := runRoot(r, "pick", "redo", "weft-hjx.1.1"); err != nil {
		t.Fatalf("redo error: %v", err)
	}
	for _, c := range r.calls {
		if strings.Contains(strings.Join(c, " "), "abandon") {
			t.Fatalf("must NOT jj abandon when unsealed: %v", r.calls)
		}
	}
	if !contains2(r.calls, "bd update weft-hjx.1.1 --status open") {
		t.Errorf("expected reopen even when unsealed: %v", r.calls)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/cli/ -run TestPickRedo`
Expected: FAIL — `pick` has no `redo` subcommand.

- [ ] **Step 3: Add the `redo` verb**

Register it in `newPickCmd` (now `seal`, `verify`, `land`, `redo`):

```go
	pick.AddCommand(a.newPickSealCmd(), a.newPickVerifyCmd(), a.newPickLandCmd(), a.newPickRedoCmd())
```

Add the verb:

```go
func (a *App) newPickRedoCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "redo <bead>",
		Short: "Recovery: abandon the pick's change (if any) and reopen the bead (spec §4.1)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			bead := args[0]
			change, err := changeOf(a.Runner, bead)
			if err != nil {
				return err
			}
			if change != "" {
				// jj abandon is a no-op when the crash preceded pick seal.
				if res, err := run.JJ(a.Runner, "abandon", change); err != nil {
					return exit.Hardf("jj abandon could not run: %v", err)
				} else if res.Code != 0 {
					return exit.Hardf("jj abandon %s failed: %s", change, strings.TrimSpace(res.Stderr))
				}
				// Drop the now-dangling spine label (best-effort).
				_, _ = run.BD(a.Runner, "update", bead, "--remove-label", jjChangeLabelPrefix+change)
			}
			// Reopen to open so the next shed form re-picks it (in_progress → open).
			if res, err := run.BD(a.Runner, "update", bead, "--status", "open"); err != nil {
				return exit.Hardf("bd update could not run: %v", err)
			} else if res.Code != 0 {
				return exit.Hardf("bd update %s failed: %s", bead, strings.TrimSpace(res.Stderr))
			}
			data := map[string]any{"bead": bead, "abandoned": change}
			return Emit(cmd, "pick.redo", data, fmt.Sprintf("redo %s (abandoned %q, reopened)", bead, change))
		},
	}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/cli/ -run TestPickRedo`
Expected: PASS.

- [ ] **Step 5: Commit**

Run: `jj commit -m "feat(cli): pick redo — abandon change + reopen bead"`

---

### Task 7: `weft resume` — read-only state projection

**Files:**
- Create: `internal/cli/resume.go`
- Modify: `internal/cli/root.go` (register `resume`)
- Test: `internal/cli/resume_test.go`

- [ ] **Step 1: Write the failing test**

```go
// internal/cli/resume_test.go
package cli

import (
	"strings"
	"testing"

	"github.com/seanb4t/weft/internal/run"
)

func TestResumeProjectsState(t *testing.T) {
	r := &routeRunner{fn: func(name string, args []string) run.Result {
		j := strings.Join(append([]string{name}, args...), " ")
		switch {
		case strings.Contains(j, "list --parent weft-hjx.1 --status closed"):
			return run.Result{Stdout: `[{"id":"weft-hjx.1.1"}]`, Code: 0}
		case strings.Contains(j, "list --parent weft-hjx.1 --status in_progress"):
			return run.Result{Stdout: `[{"id":"weft-hjx.1.5"}]`, Code: 0}
		case strings.Contains(j, "list --parent weft-hjx.1 --status blocked"):
			return run.Result{Stdout: `[]`, Code: 0}
		case strings.Contains(j, "ready --parent weft-hjx.1"):
			return run.Result{Stdout: `[{"id":"weft-hjx.1.6"}]`, Code: 0}
		case strings.Contains(j, "conflicts()"):
			return run.Result{Stdout: "", Code: 0}
		default:
			return run.Result{Code: 0}
		}
	}}
	out, err := newTestCmd(r, "resume", "--epic", "weft-hjx.1", "--json")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	s := out.String()
	for _, want := range []string{`"epic": "weft-hjx.1"`, "weft-hjx.1.1", "weft-hjx.1.5", "weft-hjx.1.6"} {
		if !strings.Contains(s, want) {
			t.Errorf("resume output missing %q: %q", want, s)
		}
	}
	// Hard invariant: resume must NOT mutate.
	for _, c := range r.calls {
		j := strings.Join(c, " ")
		if strings.Contains(j, "update") || strings.Contains(j, "close") || strings.Contains(j, "rebase") || strings.Contains(j, "abandon") {
			t.Fatalf("resume must be read-only, saw mutating call: %v", c)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/cli/ -run TestResume`
Expected: FAIL — `resume` not registered.

- [ ] **Step 3: Write `resume.go`**

```go
// internal/cli/resume.go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Weft Contributors

package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/seanb4t/weft/internal/exit"
	"github.com/seanb4t/weft/internal/run"
	"github.com/spf13/cobra"
)

func (a *App) newResumeCmd() *cobra.Command {
	var epic string
	c := &cobra.Command{
		Use:   "resume",
		Short: "Read-only projection of durable epic state (spec §4.5)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if epic == "" {
				return exit.Invocationf("--epic is required")
			}
			landed, err := beadIDsByStatus(a.Runner, epic, "closed")
			if err != nil {
				return err
			}
			inflight, err := beadIDsByStatus(a.Runner, epic, "in_progress")
			if err != nil {
				return err
			}
			blocked, err := beadIDsByStatus(a.Runner, epic, "blocked")
			if err != nil {
				return err
			}
			ready, err := readyIDs(a.Runner, epic)
			if err != nil {
				return err
			}
			conflicts, err := conflictChanges(a.Runner)
			if err != nil {
				return err
			}
			data := map[string]any{
				"epic": epic, "landed": landed, "in_flight": inflight,
				"blocked": blocked, "ready": ready, "conflicts": conflicts,
			}
			text := fmt.Sprintf("epic %s — landed %d, in-flight %d, ready %d, blocked %d, conflicts %d",
				epic, len(landed), len(inflight), len(ready), len(blocked), len(conflicts))
			return Emit(cmd, "resume", data, text)
		},
	}
	c.Flags().StringVar(&epic, "epic", "", "epic to project (required)")
	return c
}

// beadIDsByStatus lists ids of the epic's children in a given status.
func beadIDsByStatus(r run.Runner, epic, status string) ([]string, error) {
	res, err := run.BD(r, "list", "--parent", epic, "--status", status, "--json")
	if err != nil {
		return nil, exit.Hardf("bd list could not run: %v", err)
	}
	if res.Code != 0 {
		return nil, exit.Hardf("bd list failed: %s", strings.TrimSpace(res.Stderr))
	}
	return idsFromJSON(res.Stdout)
}

// readyIDs lists ids of the epic's ready (unblocked) children.
func readyIDs(r run.Runner, epic string) ([]string, error) {
	res, err := run.BD(r, "ready", "--parent", epic, "--json")
	if err != nil {
		return nil, exit.Hardf("bd ready could not run: %v", err)
	}
	if res.Code != 0 {
		return nil, exit.Hardf("bd ready failed: %s", strings.TrimSpace(res.Stderr))
	}
	return idsFromJSON(res.Stdout)
}

// conflictChanges lists the change-ids of conflicted commits in the stack.
func conflictChanges(r run.Runner) ([]string, error) {
	res, err := run.JJ(r, "log", "-r", "conflicts()", "--no-graph", "-T", `change_id.short(12) ++ "\n"`)
	if err != nil {
		return nil, exit.Hardf("jj log conflicts() could not run: %v", err)
	}
	if res.Code != 0 {
		return nil, exit.Hardf("jj log conflicts() failed: %s", strings.TrimSpace(res.Stderr))
	}
	out := []string{}
	for _, ln := range strings.Split(strings.TrimSpace(res.Stdout), "\n") {
		if ln = strings.TrimSpace(ln); ln != "" {
			out = append(out, ln)
		}
	}
	return out, nil
}

// idsFromJSON parses a bd issue-array JSON and returns the ids.
func idsFromJSON(s string) ([]string, error) {
	var arr []struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal([]byte(s), &arr); err != nil {
		return nil, exit.Hardf("parse bd json: %v", err)
	}
	ids := []string{}
	for _, i := range arr {
		ids = append(ids, i.ID)
	}
	return ids, nil
}
```

Register it in `internal/cli/root.go`'s `NewRootCmd`:

```go
	root.AddCommand(app.newReapCmd())
	root.AddCommand(app.newPickCmd())
	root.AddCommand(app.newResumeCmd())
	return root
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/cli/ -run TestResume`
Expected: PASS.

- [ ] **Step 5: Run the full suite + vet**

Run: `go vet ./... && go test ./...`
Expected: PASS across all packages; no vet complaints.

- [ ] **Step 6: Commit**

Run: `jj commit -m "feat(cli): weft resume — read-only epic state projection"`

---

## Done criteria

- `go test ./...` passes; `go vet ./...` clean; `go build ./cmd/weft` builds.
- `weft pick seal <bead>` commits `<type>(<bead>): <title>` and pins `jj-change:<id>`.
- `weft shed integrate <bead>...` rebases the sealed changes into a linear stack ordered by bead-id, surfacing `conflicts` as data.
- `weft pick verify <bead>` runs `[verify].command` and reports `{pass}` as data on exit 0.
- `weft pick land <bead>` refuses a conflicted change and otherwise `bd close --suggest-next`.
- `weft pick redo <bead>` abandons the change (if sealed) and reopens the bead.
- `weft resume --epic E` projects landed / in-flight / ready / blocked / conflicts without mutating anything.

## Out of scope (follow-on)

- `finish open/reconcile` (PR + `gh` + post-squash reconcile), `shed abandon`, `shed status` — the **ship plan**.
- Seam-4 `conflict open/finalize` and promoting integrate's `conflicts` to a top-level envelope field with `paths`/`lowest_ancestor` — the **seam-4 plan**.
- Per-bead verify-gate inference (vs the single `[verify].command`); `bd reopen`-for-closed in `pick redo`; parallel rebase — refinements.
<!-- adr-capture: sha256=65b7cce640f89655; session=cli; ts=2026-06-03T01:22:59Z; adrs= -->
