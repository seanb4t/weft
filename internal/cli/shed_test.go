// internal/cli/shed_test.go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Weft Contributors

package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/seanb4t/weft/internal/exit"
	"github.com/seanb4t/weft/internal/run"
)

func TestShedFormBuildsWaveFromBdReady(t *testing.T) {
	fake := &scriptedRunner{res: run.Result{
		Stdout: `[{"id":"weft-a1","title":"x"},{"id":"weft-a2","title":"y"}]`,
		Code:   0,
	}}
	out, err := newTestCmd(fake, "shed", "form", "--epic", "weft-hjx", "--json")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	s := out.String()
	if !strings.Contains(s, `"weft-a1"`) || !strings.Contains(s, `"weft-a2"`) {
		t.Errorf("wave missing expected picks: %q", s)
	}
	// Verify it scoped bd ready to the epic.
	joined := strings.Join(fake.gotArgs, " ")
	if fake.gotName != "bd" || !strings.Contains(joined, "ready") || !strings.Contains(joined, "--parent weft-hjx") {
		t.Errorf("ran %s %v, want bd ready --parent weft-hjx ...", fake.gotName, fake.gotArgs)
	}
}

func TestShedFormRequiresEpic(t *testing.T) {
	_, err := newTestCmd(&scriptedRunner{}, "shed", "form")
	if got := exit.Code(err); got != 1 {
		t.Fatalf("missing --epic should be exit code 1, got %d (err=%v)", got, err)
	}
}

func TestShedFormEmptyWaveEmitsJSONArrayNotNull(t *testing.T) {
	fake := &scriptedRunner{res: run.Result{Stdout: `[]`, Code: 0}}
	out, err := newTestCmd(fake, "shed", "form", "--epic", "weft-hjx", "--json")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if s := out.String(); !strings.Contains(s, `"wave": []`) {
		t.Errorf("empty wave must serialize as [], not null: %q", s)
	}
}

func TestShedFormNonZeroBdExitIsHardFailure(t *testing.T) {
	fake := &scriptedRunner{res: run.Result{Code: 1, Stderr: "bd: unknown epic"}}
	_, err := newTestCmd(fake, "shed", "form", "--epic", "weft-hjx")
	if got := exit.Code(err); got != 2 {
		t.Fatalf("non-zero bd exit should be a hard failure (exit 2), got %d (err=%v)", got, err)
	}
	if err == nil || !strings.Contains(err.Error(), "bd: unknown epic") {
		t.Errorf("hard-failure error should surface bd stderr, got %v", err)
	}
}

func TestShedFormRunnerErrorIsHardFailure(t *testing.T) {
	_, err := newTestCmd(errRunner{}, "shed", "form", "--epic", "weft-hjx")
	if got := exit.Code(err); got != 2 {
		t.Fatalf("bd that cannot start should be a hard failure (exit 2), got %d (err=%v)", got, err)
	}
}

func TestShedFormMaxMustBePositive(t *testing.T) {
	_, err := newTestCmd(&scriptedRunner{}, "shed", "form", "--epic", "weft-hjx", "--max", "0")
	if got := exit.Code(err); got != 1 {
		t.Fatalf("--max 0 should be an invocation error (exit 1), got %d (err=%v)", got, err)
	}
}

func TestShedFormPassesMaxAsLimit(t *testing.T) {
	fake := &scriptedRunner{res: run.Result{Stdout: `[]`, Code: 0}}
	if _, err := newTestCmd(fake, "shed", "form", "--epic", "weft-hjx", "--max", "3"); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if joined := strings.Join(fake.gotArgs, " "); !strings.Contains(joined, "--limit 3") {
		t.Errorf("--max 3 should pass --limit 3 to bd, got args %v", fake.gotArgs)
	}
}

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

func TestShedIsolateRunnerErrorIsHardFailure(t *testing.T) {
	_, err := newTestCmd(errRunner{}, "shed", "isolate", "weft-hjx.1.1")
	if got := exit.Code(err); got != 2 {
		t.Fatalf("subprocess that cannot start should be a hard failure (exit 2), got %d (err=%v)", got, err)
	}
}

func TestShedIsolateFetchFailureIsHardFailure(t *testing.T) {
	fake := &routeRunner{fn: func(name string, args []string) run.Result {
		if name == "jj" && len(args) >= 3 && args[2] == "fetch" {
			return run.Result{Code: 1, Stderr: "jj: offline"}
		}
		return run.Result{Code: 0}
	}}
	_, err := newTestCmd(fake, "shed", "isolate", "weft-hjx.1.1")
	if got := exit.Code(err); got != 2 {
		t.Fatalf("jj git fetch failure should be a hard failure (exit 2), got %d (err=%v)", got, err)
	}
}

func TestShedIsolateBdUpdateFailureIsHardFailure(t *testing.T) {
	fake := &routeRunner{fn: func(name string, args []string) run.Result {
		if name == "jj" && len(args) >= 2 && args[1] == "root" {
			return run.Result{Stdout: "/repo/weft", Code: 0}
		}
		if name == "bd" && len(args) >= 1 && args[0] == "update" {
			return run.Result{Code: 1, Stderr: "bd: unknown bead"}
		}
		return run.Result{Code: 0}
	}}
	_, err := newTestCmd(fake, "shed", "isolate", "weft-hjx.1.1")
	if got := exit.Code(err); got != 2 {
		t.Fatalf("bd update failure should be a hard failure (exit 2), got %d (err=%v)", got, err)
	}
}
