// internal/cli/ws_test.go
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

// scriptedRunner and errRunner are defined in version_test.go — shared across verb tests.

func TestWsListParsesWorkspaceNames(t *testing.T) {
	fake := &scriptedRunner{res: run.Result{Stdout: "default\nweft-a1\nweft-a2\n", Code: 0}}
	out, err := newTestCmd(fake, "ws", "list", "--json")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	s := out.String()
	if !strings.Contains(s, `"weft-a1"`) || !strings.Contains(s, `"default"`) {
		t.Errorf("expected workspace names in output: %q", s)
	}
	// Verify it shelled out to jj with --no-pager and a name template.
	if fake.gotName != "jj" || fake.gotArgs[0] != "--no-pager" {
		t.Errorf("ran %s %v, want jj --no-pager ...", fake.gotName, fake.gotArgs)
	}
}

func TestWsListEmptyEmitsJSONArrayNotNull(t *testing.T) {
	fake := &scriptedRunner{res: run.Result{Stdout: "", Code: 0}}
	out, err := newTestCmd(fake, "ws", "list", "--json")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if s := out.String(); !strings.Contains(s, `"workspaces": []`) {
		t.Errorf("empty workspace list must serialize as [], not null: %q", s)
	}
}

func TestWsListRunnerErrorIsHardFailure(t *testing.T) {
	_, err := newTestCmd(errRunner{}, "ws", "list")
	if got := exit.Code(err); got != 2 {
		t.Fatalf("jj that cannot start should be a hard failure (exit 2), got %d (err=%v)", got, err)
	}
}

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

func TestWsForgetEmitsRemovedPath(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root+"_worktrees", "weft-hjx__1"), 0o755); err != nil {
		t.Fatal(err)
	}
	fake := &scriptedRunner{res: run.Result{Stdout: root, Code: 0}}
	out, err := newTestCmd(fake, "ws", "forget", "weft-hjx.1", "--json")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	// The envelope must carry the removed dir's path (symmetry with ws.add).
	if s := out.String(); !strings.Contains(s, `"path"`) || !strings.Contains(s, "weft-hjx__1") {
		t.Errorf("forget envelope missing path field: %q", s)
	}
}

func TestWsAddRunnerErrorIsHardFailure(t *testing.T) {
	_, err := newTestCmd(errRunner{}, "ws", "add", "weft-hjx.1")
	if got := exit.Code(err); got != 2 {
		t.Fatalf("jj that cannot start should be a hard failure (exit 2), got %d (err=%v)", got, err)
	}
}

func TestWsForgetRunnerErrorIsHardFailure(t *testing.T) {
	_, err := newTestCmd(errRunner{}, "ws", "forget", "weft-hjx.1")
	if got := exit.Code(err); got != 2 {
		t.Fatalf("jj that cannot start should be a hard failure (exit 2), got %d (err=%v)", got, err)
	}
}

// Non-zero exit (vs cannot-start) on the workspace op must also hard-fail and
// surface stderr. Uses routeRunner so jj root succeeds and only the workspace
// add/forget call returns Code != 0.
func TestWsAddNonZeroExitIsHardFailure(t *testing.T) {
	fake := &routeRunner{fn: func(name string, args []string) run.Result {
		if name == "jj" && len(args) >= 2 && args[1] == "root" {
			return run.Result{Stdout: "/repo/weft", Code: 0}
		}
		return run.Result{Code: 1, Stderr: "jj: workspace already exists"}
	}}
	_, err := newTestCmd(fake, "ws", "add", "weft-hjx.1")
	if got := exit.Code(err); got != 2 {
		t.Fatalf("jj workspace add non-zero exit should hard-fail (exit 2), got %d (err=%v)", got, err)
	}
}

func TestWsForgetNonZeroExitIsHardFailure(t *testing.T) {
	fake := &routeRunner{fn: func(name string, args []string) run.Result {
		if name == "jj" && len(args) >= 2 && args[1] == "root" {
			return run.Result{Stdout: "/repo/weft", Code: 0}
		}
		return run.Result{Code: 1, Stderr: "jj: no such workspace"}
	}}
	_, err := newTestCmd(fake, "ws", "forget", "weft-hjx.1")
	if got := exit.Code(err); got != 2 {
		t.Fatalf("jj workspace forget non-zero exit should hard-fail (exit 2), got %d (err=%v)", got, err)
	}
}
