// internal/cli/pick_test.go
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
