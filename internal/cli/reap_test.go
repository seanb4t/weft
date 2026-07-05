// internal/cli/reap_test.go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Weft Contributors

package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/seanb4t/weft/internal/exit"
	"github.com/seanb4t/weft/internal/run"
)

// tsLayout mirrors liveness.tsLayout — the explicit strftime the liveness
// template renders, which LastActivity parses back.
const tsLayout = "2006-01-02T15:04:05-0700"

func TestReapCollectsNonInProgressWorkspaces(t *testing.T) {
	root := t.TempDir()
	wtRoot := root + "_worktrees"
	// Two executor workspaces: .1.1 (closed → orphan) and .1.2 (in_progress +
	// live → kept, now via the liveness gate).
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
		case name == "jj" && strings.Contains(j, "committer.timestamp"): // liveness probe
			// The kept in_progress workspace now flows through the liveness gate;
			// a FRESH timestamp keeps it (busy executor, not crashed).
			return run.Result{Stdout: time.Now().Format(tsLayout) + "\n", Code: 0}
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

// TestReapCollectsCrashedExecutor: an in_progress bead whose workspace has gone
// quiet past the liveness threshold is a CRASHED executor (invariant I3) —
// its bead is still in_progress but the seam-3 §5 liveness gate finds it dead,
// so it is reaped rather than over-retained.
func TestReapCollectsCrashedExecutor(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root+"_worktrees", "weft-hjx__1__1")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	tsStale := time.Now().Add(-72 * time.Hour).Format(tsLayout) // well past 45m default
	fake := &routeRunner{fn: func(name string, args []string) run.Result {
		j := strings.Join(append([]string{name}, args...), " ")
		switch {
		case name == "jj" && len(args) >= 2 && args[1] == "root":
			return run.Result{Stdout: root, Code: 0}
		case strings.Contains(j, "workspace list"):
			return run.Result{Stdout: "default\nweft-hjx__1__1\n", Code: 0}
		case strings.Contains(j, "bd show weft-hjx.1.1"):
			return run.Result{Stdout: `[{"status":"in_progress"}]`, Code: 0}
		case name == "jj" && strings.Contains(j, "committer.timestamp"): // liveness probe
			return run.Result{Stdout: tsStale + "\n", Code: 0}
		default:
			return run.Result{Code: 0}
		}
	}}
	out, err := newTestCmd(fake, "reap", "--json")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Errorf("crashed in_progress executor (stale past threshold) should be reaped, stat err = %v", err)
	}
	if !strings.Contains(out.String(), "weft-hjx.1.1") {
		t.Errorf("reaped set must carry the crashed bead-id: %q", out.String())
	}
}

// TestReapSkipsForeignWorkspace: a workspace resolving to NO bead whose
// directory is NOT under wtRoot is a foreign workspace (e.g. a Claude Code
// worktree-agent-* session). reap MUST skip it — forgetting it would break its
// owning session — and report it in foreign[], never reaped[].
func TestReapSkipsForeignWorkspace(t *testing.T) {
	root := t.TempDir()
	// No dir created under wtRoot for the foreign workspace; reapFake resolves it
	// to a missing bead (the not-found payload).
	fake := reapFake(root, "default\nworktree-agent-abc\n", map[string]run.Result{})
	foreignOut, err := newTestCmd(fake, "reap", "--pick", "data.foreign")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if got := strings.TrimSpace(foreignOut.String()); got != `["worktree-agent-abc"]` {
		t.Errorf("foreign workspace must be reported in foreign[], got %q", got)
	}
	reapedOut, err := newTestCmd(fake, "reap", "--pick", "data.reaped")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if got := strings.TrimSpace(reapedOut.String()); got != `[]` {
		t.Errorf("foreign workspace must NOT be reaped, reaped=%q", got)
	}
	for _, c := range fake.calls {
		if strings.Contains(strings.Join(c, " "), "workspace forget worktree-agent-abc") {
			t.Errorf("foreign workspace must never be forgotten: %v", c)
		}
	}
}

// TestReapStillReapsBeadlessDirUnderRoot: a workspace resolving to no bead but
// whose directory DOES exist under wtRoot is a genuine weft orphan (its bead was
// deleted) — the existing missing-bead semantic, now dir-gated. The foreign
// guard must not swallow it.
func TestReapStillReapsBeadlessDirUnderRoot(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root+"_worktrees", "weft-hjx__1__1")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	// bd reports the bead missing, but its dir is present under wtRoot.
	fake := reapFake(root, "default\nweft-hjx__1__1\n", map[string]run.Result{})
	if _, err := newTestCmd(fake, "reap"); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Errorf("beadless workspace whose dir is under wtRoot is a weft orphan and should be reaped, stat err = %v", err)
	}
}

// TestReapDryRunMutatesNothing: --dry-run reports would-reap and mutates
// nothing — no forget call, the directory stays on disk, reaped[] is empty and
// dry_run is true.
func TestReapDryRunMutatesNothing(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root+"_worktrees", "weft-hjx__1__1")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	fake := reapFake(root, "default\nweft-hjx__1__1\n", map[string]run.Result{
		"weft-hjx.1.1": {Stdout: `[{"status":"closed"}]`, Code: 0},
	})
	out, err := newTestCmd(fake, "reap", "--dry-run", "--json")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if _, statErr := os.Stat(dir); statErr != nil {
		t.Errorf("--dry-run must leave the workspace dir on disk, stat err = %v", statErr)
	}
	for _, c := range fake.calls {
		if strings.Contains(strings.Join(c, " "), "workspace forget") {
			t.Errorf("--dry-run must not forget any workspace: %v", c)
		}
	}
	s := out.String()
	if !strings.Contains(s, `"dry_run": true`) {
		t.Errorf("envelope must carry dry_run:true, got %q", s)
	}
	if !strings.Contains(s, "weft-hjx.1.1") {
		t.Errorf("would_reap must name the candidate bead-id, got %q", s)
	}
	if !strings.Contains(s, `"reaped": []`) {
		t.Errorf("reaped must be empty under --dry-run, got %q", s)
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
