// internal/cli/reap_test.go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Weft Contributors

package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/seanb4t/weft/internal/exit"
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
	// reaped[] carries bead-ids (matching shed.isolate/cleanup), not sanitized
	// workspace names: the orphan's bead-id present, the kept bead's absent.
	if !strings.Contains(out.String(), "weft-hjx.1.1") || strings.Contains(out.String(), "weft-hjx.1.2") {
		t.Errorf("reaped set wrong (want bead-id weft-hjx.1.1, not weft-hjx.1.2): %q", out.String())
	}
	// `default` must never be queried or reaped.
	for _, c := range fake.calls {
		if strings.Contains(strings.Join(c, " "), "forget default") {
			t.Errorf("default workspace must never be reaped")
		}
	}
}

func TestReapRunnerErrorIsHardFailure(t *testing.T) {
	_, err := newTestCmd(errRunner{}, "reap")
	if got := exit.Code(err); got != 2 {
		t.Fatalf("subprocess that cannot start should be a hard failure (exit 2), got %d (err=%v)", got, err)
	}
}

// A zero-exit bd show whose JSON does not parse is an anomaly, not a missing
// bead. Because reap is destructive, that must hard-fail and the workspace must
// survive — never silently reaped on an indeterminate status.
func TestReapMalformedBeadJSONIsHardFailureAndPreservesWorkspace(t *testing.T) {
	root := t.TempDir()
	wsDir := filepath.Join(root+"_worktrees", "weft-hjx__1__1")
	if err := os.MkdirAll(wsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	fake := &routeRunner{fn: func(name string, args []string) run.Result {
		j := strings.Join(append([]string{name}, args...), " ")
		switch {
		case name == "jj" && len(args) >= 2 && args[1] == "root":
			return run.Result{Stdout: root, Code: 0}
		case strings.Contains(j, "workspace list"):
			return run.Result{Stdout: "default\nweft-hjx__1__1\n", Code: 0}
		case strings.Contains(j, "bd show weft-hjx.1.1"):
			return run.Result{Stdout: "not-json", Code: 0} // exit 0 but garbage
		default:
			return run.Result{Code: 0}
		}
	}}
	_, err := newTestCmd(fake, "reap")
	if got := exit.Code(err); got != 2 {
		t.Fatalf("malformed bd JSON (exit 0) should be a hard failure (exit 2), got %d (err=%v)", got, err)
	}
	if _, statErr := os.Stat(wsDir); statErr != nil {
		t.Errorf("workspace must be preserved on indeterminate status, stat err = %v", statErr)
	}
}

// reapFake builds a routeRunner: jj root → root, workspace list → the given
// lines, and bd show dispatched by beadResults[bead-id] (a run.Result). Any
// unmatched call returns Code 0 (forget, etc.).
func reapFake(root, listOut string, beadResults map[string]run.Result) *routeRunner {
	return &routeRunner{fn: func(name string, args []string) run.Result {
		j := strings.Join(append([]string{name}, args...), " ")
		switch {
		case name == "jj" && len(args) >= 2 && args[1] == "root":
			return run.Result{Stdout: root, Code: 0}
		case strings.Contains(j, "workspace list"):
			return run.Result{Stdout: listOut, Code: 0}
		case name == "bd" && len(args) >= 2 && args[0] == "show":
			if r, ok := beadResults[args[1]]; ok {
				return r
			}
			return run.Result{Code: 1, Stdout: `{"error":"no issues found matching the provided IDs"}`}
		default:
			return run.Result{Code: 0}
		}
	}}
}

func TestReapEpicScopeFilter(t *testing.T) {
	root := t.TempDir()
	wtRoot := root + "_worktrees"
	inScope := filepath.Join(wtRoot, "weft-hjx__1__1")  // under epic weft-hjx.1
	outScope := filepath.Join(wtRoot, "weft-zzz__9__9") // different epic
	for _, d := range []string{inScope, outScope} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	// Both beads are closed (orphans), but --epic weft-hjx.1 must scope to the
	// first only — the out-of-scope workspace is never even queried or removed.
	fake := reapFake(root, "default\nweft-hjx__1__1\nweft-zzz__9__9\n", map[string]run.Result{
		"weft-hjx.1.1": {Stdout: `[{"status":"closed"}]`, Code: 0},
		"weft-zzz.9.9": {Stdout: `[{"status":"closed"}]`, Code: 0},
	})
	if _, err := newTestCmd(fake, "reap", "--epic", "weft-hjx.1"); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if _, err := os.Stat(inScope); !os.IsNotExist(err) {
		t.Errorf("in-scope orphan should be reaped, stat err = %v", err)
	}
	if _, err := os.Stat(outScope); err != nil {
		t.Errorf("out-of-scope workspace must be untouched, stat err = %v", err)
	}
	for _, c := range fake.calls {
		if strings.Contains(strings.Join(c, " "), "bd show weft-zzz.9.9") {
			t.Errorf("out-of-scope bead must not even be queried: %v", c)
		}
	}
}

func TestReapBeadNotFoundIsReaped(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root+"_worktrees", "weft-hjx__1__1")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	// bd show exits non-zero with the recognized not-found payload → orphan.
	fake := reapFake(root, "default\nweft-hjx__1__1\n", map[string]run.Result{
		"weft-hjx.1.1": {Stdout: `{"error":"no issues found matching the provided IDs"}`, Code: 1},
	})
	if _, err := newTestCmd(fake, "reap"); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Errorf("workspace of a genuinely-missing bead should be reaped, stat err = %v", err)
	}
}

func TestReapInfraErrorIsHardFailureAndPreserves(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root+"_worktrees", "weft-hjx__1__1")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	// bd show exits non-zero with an infrastructure error (NOT not-found) → must
	// hard-fail and preserve the workspace, never reap on a transient glitch.
	fake := reapFake(root, "default\nweft-hjx__1__1\n", map[string]run.Result{
		"weft-hjx.1.1": {Stderr: "bd: dial tcp 127.0.0.1:3306: connect: connection refused", Code: 1},
	})
	_, err := newTestCmd(fake, "reap")
	if got := exit.Code(err); got != 2 {
		t.Fatalf("infrastructure error should hard-fail (exit 2), got %d (err=%v)", got, err)
	}
	if _, statErr := os.Stat(dir); statErr != nil {
		t.Errorf("workspace must be preserved on infrastructure error, stat err = %v", statErr)
	}
}

func TestReapEmptyArrayIsReaped(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root+"_worktrees", "weft-hjx__1__1")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Zero-exit empty array → bd found nothing → orphan.
	fake := reapFake(root, "default\nweft-hjx__1__1\n", map[string]run.Result{
		"weft-hjx.1.1": {Stdout: `[]`, Code: 0},
	})
	if _, err := newTestCmd(fake, "reap"); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Errorf("empty-result bead's workspace should be reaped, stat err = %v", err)
	}
}

func TestReapRefusesPathTraversal(t *testing.T) {
	root := t.TempDir()
	// The worktrees root must exist so ContainsResolved resolves the parent and
	// actually exercises the containment-escape (!safe) branch — not the
	// parent-missing error branch, which would pass for the wrong reason.
	if err := os.MkdirAll(root+"_worktrees", 0o755); err != nil {
		t.Fatalf("mkdir wtRoot: %v", err)
	}
	// A workspace name that escapes the worktrees root via "..": the guard must
	// hard-fail BEFORE any forget/RemoveAll, never deleting outside wtRoot.
	fake := reapFake(root, "default\n../escape\n", map[string]run.Result{
		"../escape": {Stdout: `[{"status":"closed"}]`, Code: 0},
	})
	_, err := newTestCmd(fake, "reap")
	if got := exit.Code(err); got != 2 {
		t.Fatalf("path-traversal workspace name should hard-fail (exit 2), got %d (err=%v)", got, err)
	}
	for _, c := range fake.calls {
		if strings.Contains(strings.Join(c, " "), "workspace forget ../escape") {
			t.Errorf("must refuse before forgetting an escaping workspace: %v", c)
		}
	}
}

func TestReapWorkspaceListNonZeroIsHardFailure(t *testing.T) {
	fake := &routeRunner{fn: func(name string, args []string) run.Result {
		if name == "jj" && len(args) >= 2 && args[1] == "root" {
			return run.Result{Stdout: "/repo/weft", Code: 0}
		}
		if name == "jj" && len(args) >= 2 && args[1] == "workspace" {
			return run.Result{Code: 1, Stderr: "jj: not a workspace"}
		}
		return run.Result{Code: 0}
	}}
	_, err := newTestCmd(fake, "reap")
	if got := exit.Code(err); got != 2 {
		t.Fatalf("jj workspace list non-zero exit should hard-fail (exit 2), got %d (err=%v)", got, err)
	}
}
