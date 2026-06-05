# weft finish Ship Verbs Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement the `weft finish open <epic>` and `weft finish reconcile <epic>` ship verbs that push a finished epic's woven stack, open a GitHub PR whose body is assembled from the epic's closed beads, and (after a human merges) reconcile the local jj topology by detecting the merge style.

**Architecture:** Two cobra subcommands under a new `finish` parent (`internal/cli/finish.go`), following the established verb pattern (cobra command → `run.JJ`/`run.BD`/new `run.GH` via `App.Runner` → `Emit`). `finish open` is the first engine verb to shell out to `gh`. `finish reconcile` gates on the PR's merged state via `gh`, then detects whether the epic's pushed tip is an ancestor of `main@origin` to choose between `jj rebase --skip-emptied` (true-merge) and `jj new main` + `jj abandon` (squash/rebase-merge).

**Tech Stack:** Go 1.26, cobra, the `internal/run` subprocess layer (`run.Runner` fake-injectable), `internal/exit` (exit 1 = `Invocationf`, exit 2 = `Hardf`), `internal/envelope` via `Emit`. Source spec: `docs/seams/06-finish-ship-verbs.md` (design bead `weft-hjx.9`).

---

## File Structure

- **Create** `internal/cli/finish.go` — `newFinishCmd` parent + `newFinishOpenCmd` + `newFinishReconcileCmd` + the `finishPick` type, `closedPicks` reader, `assemblePRBody`, preflight helpers, and `mergeStyle` detection.
- **Create** `internal/cli/finish_test.go` — table tests for all of the above, using the existing `routeRunner` fake (defined in `shed_test.go`, same package).
- **Modify** `internal/run/run.go` — add the `GH` wrapper (mirrors `BD`).
- **Modify** `internal/run/run_test.go` — test `GH` prepends `gh`.
- **Modify** `internal/cli/root.go:48-56` — register `app.newFinishCmd()` in the verb tree.

Reused as-is (no changes): `changeFromLabels` (`bead.go`), `writeTempPayload` (`plan.go`), `Emit` (`emit.go`), `exit.*`, `splitTrimLines` (`lines.go`).

---

## Task 1: Add the `run.GH` wrapper

**Files:**
- Modify: `internal/run/run.go:54-57`
- Test: `internal/run/run_test.go`

- [ ] **Step 1: Write the failing test**

Add to `internal/run/run_test.go`. The existing `fakeRunner` (run_test.go:82) captures the call into its `name`/`args` fields and there is an `equal([]string, []string) bool` helper (run_test.go:92) — mirror the existing `TestJJ…`/`TestBD…` tests exactly:

```go
func TestGHInvokesRunnerWithNameAndArgs(t *testing.T) {
	f := &fakeRunner{}
	if _, err := GH(f, "pr", "view", "weft-x", "--json", "state"); err != nil {
		t.Fatalf("GH: %v", err)
	}
	if f.name != "gh" {
		t.Errorf("binary = %q, want gh", f.name)
	}
	want := []string{"pr", "view", "weft-x", "--json", "state"}
	if !equal(f.args, want) {
		t.Errorf("args = %v, want %v", f.args, want)
	}
}
```

(Unlike `JJ`, `GH` prepends nothing — `gh`'s args pass through verbatim. If the existing JJ/BD tests live under different names, match their naming; do not invent a new fake.)

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/run/ -run TestGHPrependsBinary -v`
Expected: FAIL — `undefined: GH`.

- [ ] **Step 3: Add the wrapper**

In `internal/run/run.go`, after `BD` (line 57):

```go
// GH runs the GitHub CLI. Used only by the finish verbs (push/PR/merge-state);
// like bd/jj it is a deterministic CLI, not agent dispatch.
func GH(r Runner, args ...string) (Result, error) {
	return r.Run("gh", args...)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/run/ -run TestGHPrependsBinary -v`
Expected: PASS.

- [ ] **Step 5: Commit**

`jj describe -m "feat(weft-hjx.9): add run.GH wrapper for the finish verbs"` then `jj new` (per references/vcs-preamble.md). Keep each task on its own change.

---

## Task 2: `closedPicks` reader + `assemblePRBody`

These are the pure, easily-tested core of `finish open`'s body assembly (spec §5). Build them before the command so the command wiring is thin.

**Files:**
- Create: `internal/cli/finish.go`
- Test: `internal/cli/finish_test.go`

- [ ] **Step 1: Write the failing tests**

Create `internal/cli/finish_test.go`:

```go
// internal/cli/finish_test.go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Weft Contributors

package cli

import (
	"strings"
	"testing"

	"github.com/seanb4t/weft/internal/run"
)

func TestClosedPicksReadsBeadTitleAndChange(t *testing.T) {
	r := &routeRunner{fn: func(name string, args []string) run.Result {
		j := strings.Join(append([]string{name}, args...), " ")
		if strings.Contains(j, "bd list --parent weft-e --status closed") {
			return run.Result{Stdout: `[{"id":"weft-e.1","title":"feat: A","labels":["jj-change:cha"]},{"id":"weft-e.2","title":"fix: B","labels":["jj-change:chb"]}]`, Code: 0}
		}
		return run.Result{Code: 0}
	}}
	picks, err := closedPicks(r, "weft-e")
	if err != nil {
		t.Fatalf("closedPicks: %v", err)
	}
	if len(picks) != 2 {
		t.Fatalf("want 2 picks, got %d", len(picks))
	}
	if picks[0] != (finishPick{Bead: "weft-e.1", Title: "feat: A", Change: "cha"}) {
		t.Errorf("picks[0] = %+v", picks[0])
	}
}

func TestClosedPicksEmptyEpicReturnsEmptySlice(t *testing.T) {
	r := &routeRunner{fn: func(string, []string) run.Result {
		return run.Result{Stdout: `[]`, Code: 0}
	}}
	picks, err := closedPicks(r, "weft-e")
	if err != nil {
		t.Fatalf("closedPicks: %v", err)
	}
	if picks == nil || len(picks) != 0 {
		t.Errorf("want non-nil empty slice, got %#v", picks)
	}
}

func TestAssemblePRBodyListsPicksWithChangeIDs(t *testing.T) {
	picks := []finishPick{
		{Bead: "weft-e.1", Title: "feat: A", Change: "cha"},
		{Bead: "weft-e.2", Title: "fix: B", Change: "chb"},
	}
	body := assemblePRBody("weft-e", "E title", picks)
	for _, want := range []string{
		"2 picks woven for weft-e",
		"`weft-e.1` feat: A (`cha`)",
		"`weft-e.2` fix: B (`chb`)",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("body missing %q\n---\n%s", want, body)
		}
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/cli/ -run 'TestClosedPicks|TestAssemblePRBody' -v`
Expected: FAIL — `undefined: closedPicks`, `undefined: finishPick`, `undefined: assemblePRBody`.

- [ ] **Step 3: Implement the helpers**

Create `internal/cli/finish.go`:

```go
// internal/cli/finish.go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Weft Contributors

package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/seanb4t/weft/internal/exit"
	"github.com/seanb4t/weft/internal/run"
)

// finishPick is one closed pick: its bead id, conventional-commit subject
// (bead title), and woven jj change-id. The PR body is assembled from these
// (spec §5, design.md §5.1 audit — no SUMMARY.md).
type finishPick struct {
	Bead   string `json:"bead"`
	Title  string `json:"title"`
	Change string `json:"change"`
}

// closedPicks reads the epic's closed children in ONE bd list call, yielding
// bead id + title + jj-change label together. (beadIDsByStatus returns only
// ids; epicChanges returns only change-ids — neither alone carries the title,
// so finish open needs this combined reader; see spec §5.) Returns a non-nil
// empty slice for an epic with no closed children.
func closedPicks(r run.Runner, epic string) ([]finishPick, error) {
	res, err := run.BD(r, "list", "--parent", epic, "--status", "closed", "--json")
	if err != nil {
		return nil, exit.Hardf("bd list could not run: %v", err)
	}
	if res.Code != 0 {
		return nil, exit.Hardf("bd list failed: %s", strings.TrimSpace(res.Stderr))
	}
	var arr []struct {
		ID     string   `json:"id"`
		Title  string   `json:"title"`
		Labels []string `json:"labels"`
	}
	if err := json.Unmarshal([]byte(res.Stdout), &arr); err != nil {
		return nil, exit.Hardf("parse bd json: %v", err)
	}
	picks := make([]finishPick, 0, len(arr))
	for _, b := range arr {
		picks = append(picks, finishPick{Bead: b.ID, Title: b.Title, Change: changeFromLabels(b.Labels)})
	}
	return picks, nil
}

// assemblePRBody renders the PR body from the epic's closed picks (spec §5):
// a one-line summary, one bullet per pick tying its subject to its change-id,
// and the generated-by trailer. Deterministic — no LLM call.
func assemblePRBody(epic, title string, picks []finishPick) string {
	var b strings.Builder
	fmt.Fprintf(&b, "## Summary\n\n%d picks woven for %s — %s.\n\n## Picks\n\n", len(picks), epic, title)
	for _, p := range picks {
		fmt.Fprintf(&b, "- `%s` %s (`%s`)\n", p.Bead, p.Title, p.Change)
	}
	b.WriteString("\n🤖 Generated with [Claude Code](https://claude.com/claude-code)\n")
	return b.String()
}

// (newFinishCmd and the subcommands are added in Tasks 3–6, which add the
// cobra import at that point.)
```

NOTE: Task 2 deliberately does NOT import `cobra` — these helpers are pure. Task 3 adds the `cobra` import alongside the command constructors, so the file compiles cleanly at the end of each task.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/cli/ -run 'TestClosedPicks|TestAssemblePRBody' -v`
Expected: PASS (all three).

- [ ] **Step 5: Commit**

`jj describe -m "feat(weft-hjx.9): closedPicks reader + assemblePRBody for finish open"` then `jj new`.

---

## Task 3: `finish` command tree + `finish open` preflight

Preflight refusals (spec §4.1 step 1, §5 empty-epic guard). Each failure is `exit.Invocationf` (exit 1) with a specific message — never a cryptic mid-push `gh` error.

**Files:**
- Modify: `internal/cli/finish.go`
- Modify: `internal/cli/root.go:55`
- Test: `internal/cli/finish_test.go`

- [ ] **Step 1: Write the failing tests**

Add to `internal/cli/finish_test.go`:

```go
import "github.com/seanb4t/weft/internal/exit" // add to the import block

// finishPreflightRunner returns a routeRunner where EVERY finish-open preflight
// check passes and the epic has one closed pick. `over` overlays
// command-specific responses (matched FIRST) so a single test can fail exactly
// one check while all earlier checks still pass — otherwise a refusal test
// could trip an earlier check and pass for the wrong reason.
//
// Routing precision matters: the clean-tree probe is `jj --no-pager st`, whose
// joined form ENDS with " st". Do NOT route it on Contains(j,"st") — that also
// matches "git remote liST". Use HasSuffix(j," st").
func finishPreflightRunner(over func(j string) (run.Result, bool)) *routeRunner {
	return &routeRunner{fn: func(name string, args []string) run.Result {
		j := strings.Join(append([]string{name}, args...), " ")
		if over != nil {
			if res, ok := over(j); ok {
				return res
			}
		}
		switch {
		case strings.HasSuffix(j, " st"):
			return run.Result{Stdout: "The working copy has no changes.\n", Code: 0}
		case strings.Contains(j, "log -r trunk()..@"):
			return run.Result{Stdout: "cha\n", Code: 0}
		case strings.Contains(j, "git remote list"):
			return run.Result{Stdout: "origin https://github.com/o/r (git)\n", Code: 0}
		case strings.Contains(j, "auth status"):
			return run.Result{Code: 0}
		case strings.Contains(j, "bd list --parent weft-e --status closed"):
			return run.Result{Stdout: `[{"id":"weft-e.1","title":"feat: A","labels":["jj-change:cha"]}]`, Code: 0}
		}
		return run.Result{Code: 0}
	}}
}

func TestFinishOpenRefusesDirtyTree(t *testing.T) {
	r := finishPreflightRunner(func(j string) (run.Result, bool) {
		if strings.HasSuffix(j, " st") {
			return run.Result{Stdout: "Working copy changes:\nM internal/cli/finish.go\n", Code: 0}, true
		}
		return run.Result{}, false
	})
	_, err := newTestCmd(r, "finish", "open", "weft-e")
	if got := exit.Code(err); got != 1 {
		t.Fatalf("dirty working copy must be exit 1, got %d (err=%v)", got, err)
	}
	for _, c := range r.calls {
		if strings.Contains(strings.Join(c, " "), "git push") {
			t.Errorf("must not push with a dirty working copy: %v", c)
		}
	}
}

func TestFinishOpenRefusesEmptyStack(t *testing.T) {
	r := finishPreflightRunner(func(j string) (run.Result, bool) {
		if strings.Contains(j, "log -r trunk()..@") {
			return run.Result{Stdout: "", Code: 0}, true // nothing to ship
		}
		return run.Result{}, false
	})
	_, err := newTestCmd(r, "finish", "open", "weft-e", "--json")
	if got := exit.Code(err); got != 1 {
		t.Fatalf("empty stack must be exit 1, got %d (err=%v)", got, err)
	}
}

func TestFinishOpenRefusesNoRemote(t *testing.T) {
	r := finishPreflightRunner(func(j string) (run.Result, bool) {
		if strings.Contains(j, "git remote list") {
			return run.Result{Stdout: "", Code: 0}, true // no origin
		}
		return run.Result{}, false
	})
	_, err := newTestCmd(r, "finish", "open", "weft-e")
	if got := exit.Code(err); got != 1 {
		t.Fatalf("missing origin must be exit 1, got %d (err=%v)", got, err)
	}
}

func TestFinishOpenRefusesEmptyEpic(t *testing.T) {
	r := finishPreflightRunner(func(j string) (run.Result, bool) {
		if strings.Contains(j, "bd list --parent weft-e --status closed") {
			return run.Result{Stdout: `[]`, Code: 0}, true // nothing woven
		}
		return run.Result{}, false
	})
	_, err := newTestCmd(r, "finish", "open", "weft-e")
	if got := exit.Code(err); got != 1 {
		t.Fatalf("empty epic must be exit 1, got %d (err=%v)", got, err)
	}
	for _, c := range r.calls {
		if len(c) > 0 && (strings.Contains(strings.Join(c, " "), "git push") || strings.Contains(strings.Join(c, " "), "pr create")) {
			t.Errorf("must not push/create PR for an empty epic: %v", c)
		}
	}
}
```

This shared `finishPreflightRunner` is reused by Task 4's happy-path tests.

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/cli/ -run TestFinishOpenRefuses -v`
Expected: FAIL — `finish` command not registered (cobra "unknown command").

- [ ] **Step 3: Add the command tree + preflight**

In `internal/cli/finish.go`: add `"github.com/spf13/cobra"` to the import block, then add:

```go
func (a *App) newFinishCmd() *cobra.Command {
	finish := &cobra.Command{Use: "finish", Short: "Ship an epic: open a PR, then reconcile after merge (spec §6 / seam 6)"}
	finish.AddCommand(a.newFinishOpenCmd(), a.newFinishReconcileCmd())
	return finish
}

// finishOpenPreflight enforces the spec §4.1 step-1 / §5 guards. Returns the
// closed picks (so the caller need not re-read) on success.
func (a *App) finishOpenPreflight(epic string) ([]finishPick, error) {
	// 1. Working tree clean: in jj, the clean state is an EMPTY @ on top of the
	// described picks (post jj commit / jj new). jj prints this exact line.
	if res, err := run.JJ(a.Runner, "st"); err != nil {
		return nil, exit.Hardf("jj st could not run: %v", err)
	} else if res.Code != 0 {
		return nil, exit.Hardf("jj st failed: %s", strings.TrimSpace(res.Stderr))
	} else if !strings.Contains(res.Stdout, "no changes") {
		return nil, exit.Invocationf("working copy is not clean — commit your picks (jj commit) before finishing")
	}
	// 2. Stack non-empty: there is something between trunk() and @ to ship.
	res, err := run.JJ(a.Runner, "log", "-r", "trunk()..@", "--no-graph", "-T", `change_id.short(12) ++ "\n"`)
	if err != nil {
		return nil, exit.Hardf("jj log could not run: %v", err)
	}
	if res.Code != 0 {
		return nil, exit.Hardf("jj log failed: %s", strings.TrimSpace(res.Stderr))
	}
	if strings.TrimSpace(res.Stdout) == "" {
		return nil, exit.Invocationf("nothing to ship for %s — no changes between trunk() and @", epic)
	}
	// 3. origin remote configured.
	if res, err := run.JJ(a.Runner, "git", "remote", "list"); err != nil {
		return nil, exit.Hardf("jj git remote list could not run: %v", err)
	} else if res.Code != 0 {
		return nil, exit.Hardf("jj git remote list failed: %s", strings.TrimSpace(res.Stderr))
	} else if !strings.Contains(res.Stdout, "origin") {
		return nil, exit.Invocationf("no 'origin' remote configured — cannot push %s", epic)
	}
	// 4. gh authenticated.
	if res, err := run.GH(a.Runner, "auth", "status"); err != nil {
		return nil, exit.Hardf("gh auth status could not run (is gh installed?): %v", err)
	} else if res.Code != 0 {
		return nil, exit.Invocationf("gh is not authenticated — run `gh auth login`")
	}
	// 5. Empty-epic guard (§5): refuse rather than open an empty PR.
	picks, err := closedPicks(a.Runner, epic)
	if err != nil {
		return nil, err
	}
	if len(picks) == 0 {
		return nil, exit.Invocationf("nothing woven to ship for %s — no closed beads", epic)
	}
	return picks, nil
}
```

Add a stub `newFinishOpenCmd` that only runs preflight for now (the happy path lands in Task 4) and a stub `newFinishReconcileCmd` (filled in Tasks 5–6):

```go
func (a *App) newFinishOpenCmd() *cobra.Command {
	var dryRun, draft bool
	c := &cobra.Command{
		Use:   "open <epic>",
		Short: "Push the epic's stack and open a GitHub PR (body from closed beads)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			epic := args[0]
			picks, err := a.finishOpenPreflight(epic)
			if err != nil {
				return err
			}
			_ = picks
			_ = dryRun
			_ = draft
			return exit.Hardf("finish open happy path not yet implemented") // replaced in Task 4
		},
	}
	c.Flags().BoolVar(&dryRun, "dry-run", false, "emit the push plan + PR body + gh command without mutating")
	c.Flags().BoolVar(&draft, "draft", false, "open the PR as a draft")
	return c
}

func (a *App) newFinishReconcileCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "reconcile <epic>",
		Short: "Reconcile local jj state after the epic's PR merges",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return exit.Hardf("finish reconcile not yet implemented") // replaced in Tasks 5–6
		},
	}
	return c
}
```

Register in `internal/cli/root.go` after line 55 (`app.newConflictCmd()`):

```go
	root.AddCommand(app.newFinishCmd())
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/cli/ -run TestFinishOpenRefuses -v`
Expected: PASS (all three preflight refusals).

- [ ] **Step 5: Commit**

`jj describe -m "feat(weft-hjx.9): finish command tree + finish open preflight"` then `jj new`.

---

## Task 4: `finish open` happy path (push, PR body, gh pr create, idempotency, --dry-run)

**Files:**
- Modify: `internal/cli/finish.go` (`newFinishOpenCmd` RunE)
- Test: `internal/cli/finish_test.go`

- [ ] **Step 1: Write the failing tests**

Add to `internal/cli/finish_test.go`:

```go
import "encoding/json" // add to the import block

// These tests reuse finishPreflightRunner (defined in Task 3): every preflight
// check passes, the epic has one closed pick (weft-e.1 / cha), and `over`
// overlays the command(s) specific to each test. Do NOT redefine a second
// all-pass runner here.

func TestFinishOpenDryRunMutatesNothing(t *testing.T) {
	r := finishPreflightRunner(nil)
	out, err := newTestCmd(r, "finish", "open", "weft-e", "--dry-run", "--json")
	if err != nil {
		t.Fatalf("dry-run: %v", err)
	}
	for _, c := range r.calls {
		j := strings.Join(c, " ")
		if strings.Contains(j, "git push") || strings.Contains(j, "pr create") || strings.Contains(j, "bookmark set") {
			t.Errorf("dry-run must not mutate; saw %v", c)
		}
	}
	if !strings.Contains(out.String(), `"dry_run": true`) {
		t.Errorf("dry-run envelope missing dry_run:true: %q", out.String())
	}
}

func TestFinishOpenPushesAndCreatesPR(t *testing.T) {
	r := finishPreflightRunner(func(j string) (run.Result, bool) {
		if strings.Contains(j, "pr view weft-e") {
			return run.Result{Code: 1, Stderr: "no pull requests found"}, true // no existing PR
		}
		if strings.Contains(j, "pr create") {
			return run.Result{Stdout: "https://github.com/o/r/pull/42\n", Code: 0}, true
		}
		return run.Result{}, false
	})
	out, err := newTestCmd(r, "finish", "open", "weft-e", "--json")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	var sawSet, sawPush, sawCreate bool
	for _, c := range r.calls {
		j := strings.Join(c, " ")
		sawSet = sawSet || strings.Contains(j, "bookmark set weft-e")
		sawPush = sawPush || strings.Contains(j, "git push -b weft-e")
		sawCreate = sawCreate || strings.Contains(j, "pr create")
	}
	if !sawSet || !sawPush || !sawCreate {
		t.Fatalf("expected bookmark set + push + pr create; set=%v push=%v create=%v calls=%v", sawSet, sawPush, sawCreate, r.calls)
	}
	var env struct {
		Data struct {
			PRURL string       `json:"pr_url"`
			Picks []finishPick `json:"picks"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(out.String()), &env); err != nil {
		t.Fatalf("decode envelope: %v; out=%q", err, out.String())
	}
	if env.Data.PRURL != "https://github.com/o/r/pull/42" {
		t.Errorf("pr_url = %q", env.Data.PRURL)
	}
	if len(env.Data.Picks) != 1 || env.Data.Picks[0].Change != "cha" {
		t.Errorf("picks = %+v", env.Data.Picks)
	}
}

func TestFinishOpenIdempotentWhenPRExists(t *testing.T) {
	r := finishPreflightRunner(func(j string) (run.Result, bool) {
		if strings.Contains(j, "pr view weft-e") {
			return run.Result{Stdout: `{"url":"https://github.com/o/r/pull/7","state":"OPEN"}`, Code: 0}, true
		}
		return run.Result{}, false
	})
	out, err := newTestCmd(r, "finish", "open", "weft-e", "--json")
	if err != nil {
		t.Fatalf("open (existing PR): %v", err)
	}
	for _, c := range r.calls {
		if strings.Contains(strings.Join(c, " "), "pr create") {
			t.Errorf("must NOT create a second PR when one exists: %v", c)
		}
	}
	if !strings.Contains(out.String(), `"pr_exists": true`) || !strings.Contains(out.String(), "pull/7") {
		t.Errorf("expected pr_exists:true + existing url: %q", out.String())
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/cli/ -run TestFinishOpen -v`
Expected: the new happy-path tests FAIL with "finish open happy path not yet implemented" (exit 2); the Task-3 refusal tests still PASS.

- [ ] **Step 3: Implement the happy path**

Replace the `newFinishOpenCmd` RunE body (everything after `picks, err := a.finishOpenPreflight(epic)` + error check) with:

```go
			title := fmt.Sprintf("%s (%s)", epic, epic) // see NOTE below on title source
			body := assemblePRBody(epic, epic, picks)

			if dryRun {
				data := map[string]any{
					"epic": epic, "bookmark": epic, "pushed": false,
					"pr_url": "", "pr_exists": false, "picks": picks, "dry_run": true,
				}
				return Emit(cmd, "finish.open", data,
					fmt.Sprintf("[dry-run] would push %s and open PR:\n%s", epic, body))
			}

			// Set the bookmark at the working-copy tip and push.
			if res, err := run.JJ(a.Runner, "bookmark", "set", epic, "-r", "@"); err != nil {
				return exit.Hardf("jj bookmark set could not run: %v", err)
			} else if res.Code != 0 {
				return exit.Hardf("jj bookmark set %s failed: %s", epic, strings.TrimSpace(res.Stderr))
			}
			if res, err := run.JJ(a.Runner, "git", "push", "-b", epic); err != nil {
				return exit.Hardf("jj git push could not run: %v", err)
			} else if res.Code != 0 {
				return exit.Hardf("jj git push -b %s failed: %s", epic, strings.TrimSpace(res.Stderr))
			}

			// Idempotency (§4.3): if a PR already exists for the branch, re-push is
			// done above; report the existing PR instead of opening a second.
			if res, err := run.GH(a.Runner, "pr", "view", epic, "--json", "url"); err == nil && res.Code == 0 {
				var existing struct {
					URL string `json:"url"`
				}
				if json.Unmarshal([]byte(res.Stdout), &existing) == nil && existing.URL != "" {
					data := map[string]any{
						"epic": epic, "bookmark": epic, "pushed": true,
						"pr_url": existing.URL, "pr_exists": true, "picks": picks, "dry_run": false,
					}
					return Emit(cmd, "finish.open", data,
						fmt.Sprintf("re-pushed %s; PR already open: %s", epic, existing.URL))
				}
			}

			// Assemble the body to a temp file (shell-arg limits; same idiom as
			// plan emit's bd create --graph payload).
			path, cleanup, err := writeTempPayload("weft-pr-body-*.md", []byte(body))
			if err != nil {
				return err
			}
			defer cleanup()

			ghArgs := []string{"pr", "create", "--title", title, "--body-file", path, "--base", "main"}
			if draft {
				ghArgs = append(ghArgs, "--draft")
			}
			res, err := run.GH(a.Runner, ghArgs...)
			if err != nil {
				return exit.Hardf("gh pr create could not run: %v", err)
			}
			if res.Code != 0 {
				return exit.Hardf("gh pr create failed: %s", strings.TrimSpace(res.Stderr))
			}
			prURL := strings.TrimSpace(res.Stdout)
			data := map[string]any{
				"epic": epic, "bookmark": epic, "pushed": true,
				"pr_url": prURL, "pr_exists": false, "picks": picks, "dry_run": false,
			}
			return Emit(cmd, "finish.open", data, fmt.Sprintf("opened PR for %s: %s", epic, prURL))
```

NOTE on title: spec §4.2 derives the title as `"<epic-title> (<epic-id>)"`. The epic *title* requires a `bd show <epic> --json` read (the `epic` arg is the id). For this task use `epic` for both halves (`"<epic-id> (<epic-id>)"`) to keep the task self-contained, and file a follow-up note to enrich the title from `bd show` — OR, if you prefer, read the epic title here with a small `bd show <epic> --json` call extracting `.[0].title`. Pick one; do not leave a placeholder. (Recommended: read the title — it is one `run.BD(a.Runner, "show", epic, "--json")` call parsed for `title`, mirroring `showBead` in `bead.go`.)

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/cli/ -run TestFinishOpen -v`
Expected: PASS (preflight + dry-run + push/create + idempotent).

- [ ] **Step 5: Empty-picks envelope test (the []T{} contract)**

Add and run this test (the nil-slice→null guard, a weft engine output-contract memory):

```go
func TestFinishOpenDryRunPicksSerializeAsArray(t *testing.T) {
	// One closed pick → picks is a populated []; assert it's a JSON array, not null.
	r := finishPreflightRunner(nil)
	out, _ := newTestCmd(r, "finish", "open", "weft-e", "--dry-run", "--json")
	if !strings.Contains(out.String(), `"picks": [`) {
		t.Errorf("picks must serialize as a JSON array: %q", out.String())
	}
}
```

Run: `go test ./internal/cli/ -run TestFinishOpenDryRunPicksSerializeAsArray -v`
Expected: PASS (`closedPicks` already uses `make([]finishPick, 0, n)`).

- [ ] **Step 6: Commit**

`jj describe -m "feat(weft-hjx.9): finish open — push, PR body from closed beads, gh pr create, idempotency, dry-run"` then `jj new`.

---

## Task 5: `finish reconcile` merged-state gate + merge-style detection

**Files:**
- Modify: `internal/cli/finish.go`
- Test: `internal/cli/finish_test.go`

- [ ] **Step 1: Write the failing tests**

Add to `internal/cli/finish_test.go`:

```go
func TestFinishReconcileRefusesUnmergedPR(t *testing.T) {
	r := &routeRunner{fn: func(name string, args []string) run.Result {
		j := strings.Join(append([]string{name}, args...), " ")
		if strings.Contains(j, "pr view weft-e") {
			return run.Result{Stdout: `{"state":"OPEN","mergeCommit":null}`, Code: 0}
		}
		return run.Result{Code: 0}
	}}
	_, err := newTestCmd(r, "finish", "reconcile", "weft-e")
	if got := exit.Code(err); got != 1 {
		t.Fatalf("unmerged PR must be exit 1, got %d (err=%v)", got, err)
	}
	for _, c := range r.calls {
		if strings.Contains(strings.Join(c, " "), "abandon") || strings.Contains(strings.Join(c, " "), "rebase") {
			t.Errorf("must NOT touch jj topology when the PR is not merged: %v", c)
		}
	}
}

func TestMergeStyleDetectsTrueMergeVsSquash(t *testing.T) {
	// Ancestor present → merge_commit.
	rMerge := &routeRunner{fn: func(name string, args []string) run.Result {
		j := strings.Join(append([]string{name}, args...), " ")
		if strings.Contains(j, "weft-e@origin & ::main@origin") {
			return run.Result{Stdout: "deadbeef\n", Code: 0}
		}
		return run.Result{Code: 0}
	}}
	if got, err := mergeStyle(rMerge, "weft-e"); err != nil || got != "merge_commit" {
		t.Errorf("ancestor present → merge_commit; got %q err=%v", got, err)
	}
	// Ancestor absent → squash_or_rebase.
	rSquash := &routeRunner{fn: func(name string, args []string) run.Result {
		j := strings.Join(append([]string{name}, args...), " ")
		if strings.Contains(j, "weft-e@origin & ::main@origin") {
			return run.Result{Stdout: "", Code: 0}
		}
		return run.Result{Code: 0}
	}}
	if got, err := mergeStyle(rSquash, "weft-e"); err != nil || got != "squash_or_rebase" {
		t.Errorf("ancestor absent → squash_or_rebase; got %q err=%v", got, err)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/cli/ -run 'TestFinishReconcileRefusesUnmergedPR|TestMergeStyleDetects' -v`
Expected: FAIL — `undefined: mergeStyle`; reconcile stub returns exit 2 not 1.

- [ ] **Step 3: Implement the gate + detection**

In `internal/cli/finish.go` add:

```go
// prMerged reports whether the epic's PR is in the MERGED state (spec §6.1
// safety gate — never abandon unmerged work; jj alone cannot distinguish a
// squash-merge from a never-merged branch).
func prMerged(r run.Runner, epic string) (bool, error) {
	res, err := run.GH(r, "pr", "view", epic, "--json", "state,mergeCommit")
	if err != nil {
		return false, exit.Hardf("gh pr view could not run: %v", err)
	}
	if res.Code != 0 {
		return false, exit.Hardf("gh pr view %s failed: %s", epic, strings.TrimSpace(res.Stderr))
	}
	var v struct {
		State string `json:"state"`
	}
	if err := json.Unmarshal([]byte(res.Stdout), &v); err != nil {
		return false, exit.Hardf("parse gh pr view json: %v", err)
	}
	return v.State == "MERGED", nil
}

// mergeStyle returns "merge_commit" if the epic's pushed tip (<epic>@origin) is
// an ancestor of main@origin (a true-merge, reconcilable via rebase
// --skip-emptied), or "squash_or_rebase" otherwise (content landed under a new
// commit id — needs jj new main + jj abandon). Spec §6.1.3.
func mergeStyle(r run.Runner, epic string) (string, error) {
	revset := epic + "@origin & ::main@origin"
	res, err := run.JJ(r, "log", "-r", revset, "--no-graph", "-T", "commit_id")
	if err != nil {
		return "", exit.Hardf("jj log (merge-style detect) could not run: %v", err)
	}
	if res.Code != 0 {
		return "", exit.Hardf("jj log (merge-style detect) failed: %s", strings.TrimSpace(res.Stderr))
	}
	if strings.TrimSpace(res.Stdout) != "" {
		return "merge_commit", nil
	}
	return "squash_or_rebase", nil
}
```

Update `newFinishReconcileCmd` RunE to wire the gate (full execution lands in Task 6):

```go
		RunE: func(cmd *cobra.Command, args []string) error {
			epic := args[0]
			merged, err := prMerged(a.Runner, epic)
			if err != nil {
				return err
			}
			if !merged {
				return exit.Invocationf("PR for %s is not merged — refusing to reconcile", epic)
			}
			return exit.Hardf("finish reconcile execution not yet implemented") // Task 6
		},
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/cli/ -run 'TestFinishReconcileRefusesUnmergedPR|TestMergeStyleDetects' -v`
Expected: PASS.

- [ ] **Step 5: Commit**

`jj describe -m "feat(weft-hjx.9): finish reconcile merged-state gate + merge-style detection"` then `jj new`.

---

## Task 6: `finish reconcile` execution (rebase vs new+abandon, bookmark + remote cleanup, --dry-run)

**Files:**
- Modify: `internal/cli/finish.go` (`newFinishReconcileCmd` RunE + a `--dry-run` flag)
- Test: `internal/cli/finish_test.go`

- [ ] **Step 1: Write the failing tests**

Add to `internal/cli/finish_test.go`:

```go
// mergedReconcileRunner: PR merged; merge-style is controlled by `ancestor`.
func mergedReconcileRunner(ancestor bool, extra func(j string) (run.Result, bool)) *routeRunner {
	return &routeRunner{fn: func(name string, args []string) run.Result {
		j := strings.Join(append([]string{name}, args...), " ")
		if extra != nil {
			if res, ok := extra(j); ok {
				return res
			}
		}
		switch {
		case strings.Contains(j, "pr view weft-e"):
			return run.Result{Stdout: `{"state":"MERGED","mergeCommit":{"oid":"abc"}}`, Code: 0}
		case strings.Contains(j, "weft-e@origin & ::main@origin"):
			if ancestor {
				return run.Result{Stdout: "deadbeef\n", Code: 0}
			}
			return run.Result{Stdout: "", Code: 0}
		case strings.Contains(j, "roots(trunk()..@)"):
			return run.Result{Stdout: "rootchg\n", Code: 0}
		}
		return run.Result{Code: 0}
	}}
}

func TestFinishReconcileSquashUsesNewAndAbandon(t *testing.T) {
	r := mergedReconcileRunner(false, nil)
	out, err := newTestCmd(r, "finish", "reconcile", "weft-e", "--json")
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	var sawNew, sawAbandon, sawRebase bool
	for _, c := range r.calls {
		j := strings.Join(c, " ")
		sawNew = sawNew || strings.Contains(j, "new main")
		sawAbandon = sawAbandon || strings.Contains(j, "abandon")
		sawRebase = sawRebase || strings.Contains(j, "rebase")
	}
	if !sawNew || !sawAbandon {
		t.Errorf("squash path must use jj new main + abandon; new=%v abandon=%v", sawNew, sawAbandon)
	}
	if sawRebase {
		t.Errorf("squash path must NOT rebase: %v", r.calls)
	}
	if !strings.Contains(out.String(), `"merge_style": "squash_or_rebase"`) {
		t.Errorf("envelope merge_style wrong: %q", out.String())
	}
}

func TestFinishReconcileTrueMergeUsesRebase(t *testing.T) {
	r := mergedReconcileRunner(true, nil)
	out, err := newTestCmd(r, "finish", "reconcile", "weft-e", "--json")
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	var sawRebase, sawAbandon bool
	for _, c := range r.calls {
		j := strings.Join(c, " ")
		sawRebase = sawRebase || strings.Contains(j, "rebase -b @ -o main")
		sawAbandon = sawAbandon || strings.Contains(j, "abandon")
	}
	if !sawRebase {
		t.Errorf("true-merge path must rebase --skip-emptied: %v", r.calls)
	}
	if sawAbandon {
		t.Errorf("true-merge path must NOT abandon: %v", r.calls)
	}
	if !strings.Contains(out.String(), `"merge_style": "merge_commit"`) {
		t.Errorf("envelope merge_style wrong: %q", out.String())
	}
}

func TestFinishReconcileDryRunMutatesNothing(t *testing.T) {
	r := mergedReconcileRunner(false, nil)
	out, err := newTestCmd(r, "finish", "reconcile", "weft-e", "--dry-run", "--json")
	if err != nil {
		t.Fatalf("dry-run: %v", err)
	}
	for _, c := range r.calls {
		j := strings.Join(c, " ")
		if strings.Contains(j, "new main") || strings.Contains(j, "abandon") || strings.Contains(j, "rebase") || strings.Contains(j, "bookmark delete") {
			t.Errorf("dry-run must not mutate; saw %v", c)
		}
	}
	if !strings.Contains(out.String(), `"dry_run": true`) {
		t.Errorf("dry-run envelope missing dry_run:true: %q", out.String())
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/cli/ -run TestFinishReconcile -v`
Expected: the three execution tests FAIL ("not yet implemented", exit 2); the unmerged-gate test still PASSes.

- [ ] **Step 3: Implement execution**

Add a `--dry-run` flag to `newFinishReconcileCmd` and replace its RunE tail (after the merged gate) with:

```go
func (a *App) newFinishReconcileCmd() *cobra.Command {
	var dryRun bool
	c := &cobra.Command{
		Use:   "reconcile <epic>",
		Short: "Reconcile local jj state after the epic's PR merges",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			epic := args[0]
			merged, err := prMerged(a.Runner, epic)
			if err != nil {
				return err
			}
			if !merged {
				return exit.Invocationf("PR for %s is not merged — refusing to reconcile", epic)
			}
			if res, err := run.JJ(a.Runner, "git", "fetch"); err != nil {
				return exit.Hardf("jj git fetch could not run: %v", err)
			} else if res.Code != 0 {
				return exit.Hardf("jj git fetch failed: %s", strings.TrimSpace(res.Stderr))
			}
			style, err := mergeStyle(a.Runner, epic)
			if err != nil {
				return err
			}
			abandoned := []string{}
			if dryRun {
				data := map[string]any{
					"epic": epic, "merged": true, "merge_style": style,
					"abandoned": abandoned, "bookmark_deleted": false,
					"remote_branch_deleted": false, "dry_run": true,
				}
				return Emit(cmd, "finish.reconcile", data,
					fmt.Sprintf("[dry-run] %s merged (%s) — would reconcile", epic, style))
			}
			switch style {
			case "merge_commit":
				if res, err := run.JJ(a.Runner, "rebase", "-b", "@", "-o", "main", "--skip-emptied"); err != nil {
					return exit.Hardf("jj rebase could not run: %v", err)
				} else if res.Code != 0 {
					return exit.Hardf("jj rebase failed: %s", strings.TrimSpace(res.Stderr))
				}
			default: // squash_or_rebase
				if res, err := run.JJ(a.Runner, "new", "main"); err != nil {
					return exit.Hardf("jj new main could not run: %v", err)
				} else if res.Code != 0 {
					return exit.Hardf("jj new main failed: %s", strings.TrimSpace(res.Stderr))
				}
				rootRes, err := run.JJ(a.Runner, "log", "-r", "roots(trunk()..@)", "--no-graph", "-T", `change_id.short(12) ++ "\n"`)
				if err != nil {
					return exit.Hardf("jj log roots could not run: %v", err)
				}
				if rootRes.Code != 0 {
					return exit.Hardf("jj log roots failed: %s", strings.TrimSpace(rootRes.Stderr))
				}
				for _, root := range splitTrimLines(rootRes.Stdout) {
					if res, err := run.JJ(a.Runner, "abandon", root+"::"); err != nil {
						return exit.Hardf("jj abandon could not run: %v", err)
					} else if res.Code != 0 {
						return exit.Hardf("jj abandon %s:: failed: %s", root, strings.TrimSpace(res.Stderr))
					}
					abandoned = append(abandoned, root)
				}
			}
			// Drop the local bookmark (idempotent backstop; the squash abandon may
			// already have removed it — tolerate "no such bookmark").
			run.JJ(a.Runner, "bookmark", "delete", epic) //nolint:errcheck // best-effort cleanup
			data := map[string]any{
				"epic": epic, "merged": true, "merge_style": style,
				"abandoned": abandoned, "bookmark_deleted": true,
				"remote_branch_deleted": false, "dry_run": false,
			}
			return Emit(cmd, "finish.reconcile", data,
				fmt.Sprintf("reconciled %s (%s): %d abandoned", epic, style, len(abandoned)))
		},
	}
	c.Flags().BoolVar(&dryRun, "dry-run", false, "detect the merge style and emit the plan without mutating")
	return c
}
```

NOTE on the best-effort `bookmark delete`: the `//nolint:errcheck` comment suppresses the lint for the intentionally-ignored cleanup error. If the project's linter is not configured for `errcheck`, capture and discard explicitly: `_, _ = run.JJ(...)`. Do NOT swallow it silently without one of these two forms (silent-failure-hunter will flag it). The remote-branch deletion (spec §6.1.4, `gh api -X DELETE` when `--delete-branch` was unreliable) is intentionally deferred to a follow-up: it requires resolving the `{owner}/{repo}` slug; file a bead rather than stubbing it. Reflect that by leaving `remote_branch_deleted: false` and noting it in the commit message.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/cli/ -run TestFinishReconcile -v`
Expected: PASS (squash → new+abandon, true-merge → rebase, dry-run, unmerged-gate).

- [ ] **Step 5: Full-suite + gates**

Run:
```
gofmt -l internal/cli/finish.go internal/cli/finish_test.go internal/run/run.go
go vet ./...
go build -o /dev/null ./cmd/weft
go test ./...
```
Expected: gofmt prints nothing; vet clean; build OK; all packages pass.

- [ ] **Step 6: Commit**

`jj describe -m "feat(weft-hjx.9): finish reconcile execution — merge-style-aware cleanup, dry-run"` then `jj new`.

---

## Coverage check (spec → task)

- §3 verb surface (open/reconcile, --dry-run/--json/--pick) → Tasks 3,4,6 (envelope via `Emit` inherits --json/--pick).
- §4.1 preflight → Task 3. §4.2 title → Task 4 (NOTE). §4.3 idempotency → Task 4. §4 push/PR + --dry-run → Task 4.
- §5 PR body from closed beads → Task 2 (`closedPicks`, `assemblePRBody`); empty-epic guard → Task 3.
- §6.1 merged gate → Task 5; detect merge style → Task 5 (`mergeStyle`); rebase vs new+abandon, bookmark delete, --dry-run → Task 6. §6.1.4 remote-branch delete → deferred (follow-up bead, Task 6 NOTE).
- §7 exit codes (1 = Invocationf, 2 = Hardf) → all tasks. §8 envelopes + `[]T{}` → Tasks 4,6 (incl. the array-serialization test).
- §9 ADRs → captured by the `capture-adrs` step (auto-fired after plan-reviewer READY), not a code task.

## Deferred follow-ups to file as beads

1. **Title enrichment** — read the epic *title* via `bd show <epic> --json` so the PR title is `"<epic-title> (<epic-id>)"` per §4.2 (Task 4 uses the id for both halves if not done inline).
2. **Remote-branch deletion** — `gh api -X DELETE repos/{owner}/{repo}/git/refs/heads/<epic>` when the merge left the branch (spec §6.1.4); needs the repo slug resolved (`gh repo view --json nameWithOwner`).
<!-- adr-capture: sha256=69f1053311f033e2; session=cli; ts=2026-06-05T13:44:06Z; adrs=weft-6rt,weft-yuj -->
