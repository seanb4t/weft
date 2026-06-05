// internal/cli/shed_test.go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Weft Contributors

package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
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
//
// Optional errFn: when non-nil, it is called first; if it returns a non-nil
// error the Run method records the call and returns (run.Result{}, err)
// without invoking fn. This allows targeted error-injection in tests without
// affecting existing tests (errFn is nil by default).
type routeRunner struct {
	fn    func(name string, args []string) run.Result
	errFn func(name string, args []string) error
	calls [][]string
}

func (r *routeRunner) Run(name string, args ...string) (run.Result, error) {
	r.calls = append(r.calls, append([]string{name}, args...))
	if r.errFn != nil {
		if err := r.errFn(name, args); err != nil {
			return run.Result{}, err
		}
	}
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

func TestShedCleanupRunnerErrorIsHardFailure(t *testing.T) {
	_, err := newTestCmd(errRunner{}, "shed", "cleanup", "weft-hjx.1.2")
	if got := exit.Code(err); got != 2 {
		t.Fatalf("subprocess that cannot start should be a hard failure (exit 2), got %d (err=%v)", got, err)
	}
}

func TestShedCleanupForgetFailureIsHardFailure(t *testing.T) {
	fake := &routeRunner{fn: func(name string, args []string) run.Result {
		if name == "jj" && len(args) >= 2 && args[1] == "root" {
			return run.Result{Stdout: "/repo/weft", Code: 0}
		}
		if name == "jj" && len(args) >= 2 && args[1] == "workspace" {
			return run.Result{Code: 1, Stderr: "jj: no such workspace"}
		}
		return run.Result{Code: 0}
	}}
	_, err := newTestCmd(fake, "shed", "cleanup", "weft-hjx.1.2")
	if got := exit.Code(err); got != 2 {
		t.Fatalf("jj workspace forget failure should be a hard failure (exit 2), got %d (err=%v)", got, err)
	}
}

func TestShedIntegrateBuildsLinearStack(t *testing.T) {
	// Two sealed picks; integrate orders them lexicographically (weft-hjx.1.1
	// before weft-hjx.1.2) and rebases each onto the previous tip, then
	// reports stack as {bead,change} pairs and no conflicts.
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
	// Stack entries must be {bead,change} pairs — both bead-ids and change-ids present.
	s := out.String()
	if !strings.Contains(s, `"bead": "weft-hjx.1.1"`) || !strings.Contains(s, `"change": "chA"`) {
		t.Errorf("stack missing weft-hjx.1.1/chA pair: %q", s)
	}
	if !strings.Contains(s, `"bead": "weft-hjx.1.2"`) || !strings.Contains(s, `"change": "chB"`) {
		t.Errorf("stack missing weft-hjx.1.2/chB pair: %q", s)
	}
	// Conflicts revset must be stack-scoped, not bare conflicts().
	var sawScopedConflicts bool
	for _, c := range r.calls {
		j := strings.Join(c, " ")
		if strings.Contains(j, "conflicts() & (") {
			sawScopedConflicts = true
		}
	}
	if !sawScopedConflicts {
		t.Errorf("conflicts revset must be scoped (conflicts() & (...)), saw calls: %v", r.calls)
	}
}

func TestShedIntegrateSurfacesConflicts(t *testing.T) {
	// chB comes back as conflicted; integrate still exits 0 with conflicts in data.
	r := &routeRunner{fn: func(name string, args []string) run.Result {
		j := strings.Join(append([]string{name}, args...), " ")
		switch {
		case strings.Contains(j, "bd show weft-hjx.1.2"):
			return run.Result{Stdout: `[{"title":"b","status":"in_progress","labels":["jj-change:chB"]}]`, Code: 0}
		case strings.Contains(j, "bd show weft-hjx.1.1"):
			return run.Result{Stdout: `[{"title":"a","status":"in_progress","labels":["jj-change:chA"]}]`, Code: 0}
		case strings.Contains(j, "log -r conflicts()"):
			return run.Result{Stdout: "chB\n", Code: 0} // chB conflicted
		default:
			return run.Result{Code: 0}
		}
	}}
	out, err := newTestCmd(r, "shed", "integrate", "weft-hjx.1.2", "weft-hjx.1.1", "--json")
	if err != nil {
		t.Fatalf("conflicts must not cause a non-zero exit (verdict is data): %v", err)
	}
	s := out.String()
	// Stack still has both {bead,change} pairs.
	if !strings.Contains(s, `"bead": "weft-hjx.1.1"`) || !strings.Contains(s, `"change": "chA"`) {
		t.Errorf("stack missing weft-hjx.1.1/chA: %q", s)
	}
	if !strings.Contains(s, `"bead": "weft-hjx.1.2"`) || !strings.Contains(s, `"change": "chB"`) {
		t.Errorf("stack missing weft-hjx.1.2/chB: %q", s)
	}
	if !strings.Contains(s, "chB") {
		t.Errorf("conflicted chB must appear in output: %q", s)
	}
	// Conflicts revset must be scoped.
	var sawScopedConflicts bool
	for _, c := range r.calls {
		if strings.Contains(strings.Join(c, " "), "conflicts() & (") {
			sawScopedConflicts = true
		}
	}
	if !sawScopedConflicts {
		t.Errorf("conflicts revset must be scoped; calls: %v", r.calls)
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

// When jj workspace add fails after bd update already set the bead in_progress,
// shed isolate hard-fails (the bead is deliberately left in_progress for resume,
// per the status-first invariant — spec §4). Verifies the status-first ordering
// held even on the failure path: bd update ran before the failing add.
func TestShedIsolateWorkspaceAddFailureLeavesBeadInProgress(t *testing.T) {
	var updatedBeforeAdd bool
	fake := &routeRunner{fn: func(name string, args []string) run.Result {
		if name == "jj" && len(args) >= 2 && args[1] == "root" {
			return run.Result{Stdout: "/repo/weft", Code: 0}
		}
		if name == "jj" && len(args) >= 3 && args[1] == "workspace" && args[2] == "add" {
			return run.Result{Code: 1, Stderr: "jj: revision trunk() not found"}
		}
		return run.Result{Code: 0}
	}}
	_, err := newTestCmd(fake, "shed", "isolate", "weft-hjx.1.1")
	if got := exit.Code(err); got != 2 {
		t.Fatalf("jj workspace add failure should be a hard failure (exit 2), got %d (err=%v)", got, err)
	}
	// The bd update (status-first) must have happened before the failing add.
	upd, add := -1, -1
	for i, c := range fake.calls {
		j := strings.Join(c, " ")
		if strings.Contains(j, "bd update weft-hjx.1.1 --status in_progress") {
			upd = i
		}
		if strings.Contains(j, "workspace add") {
			add = i
		}
	}
	if upd < 0 {
		t.Fatalf("bd update must run (status-first) even when add later fails: %v", fake.calls)
	}
	if add < 0 || upd > add {
		t.Errorf("status-first violated on failure path: upd=%d add=%d", upd, add)
	}
	updatedBeforeAdd = upd < add
	if !updatedBeforeAdd {
		t.Errorf("bead must be set in_progress before the workspace add attempt")
	}
}

func TestShedIntegrateConflictsCarryBead(t *testing.T) {
	r := &routeRunner{fn: func(name string, args []string) run.Result {
		j := strings.Join(append([]string{name}, args...), " ")
		switch {
		case strings.Contains(j, "bd show weft-hjx.4.1"):
			return run.Result{Stdout: `[{"title":"a","status":"in_progress","labels":["jj-change:chA"]}]`, Code: 0}
		case strings.Contains(j, "bd show weft-hjx.4.2"):
			return run.Result{Stdout: `[{"title":"b","status":"in_progress","labels":["jj-change:chB"]}]`, Code: 0}
		case strings.Contains(j, "log -r conflicts()"):
			return run.Result{Stdout: "chB\n", Code: 0} // chB is conflicted
		default: // jj rebase
			return run.Result{Code: 0}
		}
	}}
	out, err := newTestCmd(r, "shed", "integrate", "weft-hjx.4.1", "weft-hjx.4.2", "--json")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	// Decode the envelope and assert on the conflicts[] field SPECIFICALLY. A
	// substring check for {bead,change} is insufficient: stack[] also carries
	// {bead,change} pairs, so it passes even when conflicts[] is still bare
	// change-id strings. Unmarshalling conflicts[] into a {bead,change} struct
	// fails outright on the pre-enrichment shape (string into struct), so this
	// genuinely guards the enrichment.
	var env struct {
		Data struct {
			Conflicts []struct {
				Bead   string `json:"bead"`
				Change string `json:"change"`
			} `json:"conflicts"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(out.String()), &env); err != nil {
		t.Fatalf("decode envelope (conflicts[] not [{bead,change}]?): %v; out=%q", err, out.String())
	}
	if len(env.Data.Conflicts) != 1 {
		t.Fatalf("want 1 conflict, got %d: %q", len(env.Data.Conflicts), out.String())
	}
	if got := env.Data.Conflicts[0]; got.Bead != "weft-hjx.4.2" || got.Change != "chB" {
		t.Errorf("conflicts[0] = %+v; want {bead:weft-hjx.4.2 change:chB}", got)
	}
}

// TestShedFormMalformedBdReadyJSONIsHardFailure verifies that when bd ready
// returns output that cannot be unmarshalled as JSON, shed form exits Hard (2)
// (qeg.13).
func TestShedFormMalformedBdReadyJSONIsHardFailure(t *testing.T) {
	fake := &scriptedRunner{res: run.Result{Stdout: `not valid json`, Code: 0}}
	_, err := newTestCmd(fake, "shed", "form", "--epic", "weft-hjx")
	if got := exit.Code(err); got != 2 {
		t.Fatalf("malformed bd ready JSON should be a hard failure (exit 2), got %d (err=%v)", got, err)
	}
}

// TestShedIntegrateConflictUnknownChangeErrors verifies that if jj reports a
// conflicted change-id that is NOT in the integration stack (and therefore cannot
// be mapped to a bead), integrate returns a hard failure instead of silently
// emitting bead:"" in the conflicts[] array. (F4)
func TestShedIntegrateConflictUnknownChangeErrors(t *testing.T) {
	r := &routeRunner{fn: func(name string, args []string) run.Result {
		j := strings.Join(append([]string{name}, args...), " ")
		switch {
		case strings.Contains(j, "bd show weft-hjx.4.1"):
			return run.Result{Stdout: `[{"title":"a","status":"in_progress","labels":["jj-change:chA"]}]`, Code: 0}
		case strings.Contains(j, "log -r conflicts()"):
			// chZ is NOT in the integration stack (only chA is).
			return run.Result{Stdout: "chZ\n", Code: 0}
		default: // jj rebase
			return run.Result{Code: 0}
		}
	}}
	_, err := newTestCmd(r, "shed", "integrate", "weft-hjx.4.1", "--json")
	if got := exit.Code(err); got != 2 {
		t.Fatalf("conflicted change not in stack should be a hard failure (exit 2), got %d (err=%v)", got, err)
	}
	if err == nil || !strings.Contains(err.Error(), "chZ") {
		t.Errorf("error should name the unknown change-id, got %v", err)
	}
}
