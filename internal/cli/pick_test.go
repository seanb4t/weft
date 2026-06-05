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

// fp0.14: pick seal with a non-default --type uses that type in the commit subject.
func TestPickSealNonDefaultType(t *testing.T) {
	r := &routeRunner{fn: func(name string, args []string) run.Result {
		j := strings.Join(append([]string{name}, args...), " ")
		switch {
		case strings.Contains(j, "bd show"):
			return run.Result{Stdout: `[{"title":"Fix bug","status":"in_progress","labels":[]}]`, Code: 0}
		case strings.Contains(j, "log -r @-"):
			return run.Result{Stdout: "deadbeef1234\n", Code: 0}
		default:
			return run.Result{Code: 0}
		}
	}}
	if err := runRoot(r, "pick", "seal", "weft-hjx.2.1", "--type", "fix"); err != nil {
		t.Fatalf("execute: %v", err)
	}
	// The jj commit message must use the supplied type, not the default "feat".
	wantSubject := "fix(weft-hjx.2.1): Fix bug"
	var sawCommit bool
	for _, c := range r.calls {
		j := strings.Join(c, " ")
		if strings.Contains(j, "jj") && strings.Contains(j, "commit") && strings.Contains(j, wantSubject) {
			sawCommit = true
		}
	}
	if !sawCommit {
		t.Errorf("expected commit subject %q; calls=%v", wantSubject, r.calls)
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
			return run.Result{Stdout: `[{"title":"t","status":"in_progress","labels":["jj-change:chx"]}]`, Code: 0}
		case strings.Contains(j, "conflicts()"):
			return run.Result{Stdout: "chx\n", Code: 0} // chx IS conflicted
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
			return run.Result{Stdout: `[{"title":"t","status":"in_progress","labels":["jj-change:chx"]}]`, Code: 0}
		case strings.Contains(j, "conflicts()"):
			return run.Result{Stdout: "", Code: 0} // clean
		default:
			return run.Result{Code: 0}
		}
	}}
	if err := runRoot(r, "pick", "land", "weft-hjx.1.1"); err != nil {
		t.Fatalf("clean land error: %v", err)
	}
	if !calledWith(r.calls, "bd close weft-hjx.1.1 --suggest-next") {
		t.Errorf("expected bd close --suggest-next: %v", r.calls)
	}
}

// runRoot executes a command with a fresh root over the given runner.
func runRoot(r run.Runner, args ...string) error {
	root := NewRootCmd(&App{Runner: r})
	root.SetArgs(args)
	return root.Execute()
}

func calledWith(calls [][]string, want string) bool {
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
			return run.Result{Stdout: `[{"title":"t","status":"in_progress","labels":["jj-change:chx"]}]`, Code: 0}
		}
		return run.Result{Code: 0}
	}}
	if err := runRoot(r, "pick", "redo", "weft-hjx.1.1"); err != nil {
		t.Fatalf("redo error: %v", err)
	}
	if !calledWith(r.calls, "jj --no-pager abandon chx") {
		t.Errorf("expected jj abandon chx: %v", r.calls)
	}
	if !calledWith(r.calls, "bd update weft-hjx.1.1 --status open") {
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
	if !calledWith(r.calls, "bd update weft-hjx.1.1 --status open") {
		t.Errorf("expected reopen even when unsealed: %v", r.calls)
	}
}

func TestPickLandRefusesUnsealedBead(t *testing.T) {
	r := &routeRunner{fn: func(name string, args []string) run.Result {
		j := strings.Join(append([]string{name}, args...), " ")
		if strings.Contains(j, "bd show") {
			// No jj-change label — bead is not sealed
			return run.Result{Stdout: `[{"title":"t","status":"in_progress","labels":[]}]`, Code: 0}
		}
		return run.Result{Code: 0}
	}}
	if got := exit.Code(runRoot(r, "pick", "land", "weft-hjx.1.1")); got != 1 {
		t.Fatalf("landing an unsealed bead must be exit 1, got %d", got)
	}
	for _, c := range r.calls {
		if strings.Contains(strings.Join(c, " "), "bd close") {
			t.Fatalf("must NOT bd close an unsealed bead: %v", r.calls)
		}
	}
}

func TestPickLandHardFailsOnConflictsCheckError(t *testing.T) {
	r := &routeRunner{fn: func(name string, args []string) run.Result {
		j := strings.Join(append([]string{name}, args...), " ")
		switch {
		case strings.Contains(j, "bd show"):
			return run.Result{Stdout: `[{"title":"t","status":"in_progress","labels":["jj-change:chx"]}]`, Code: 0}
		case strings.Contains(j, "conflicts()"):
			return run.Result{Code: 1, Stderr: "boom"}
		default:
			return run.Result{Code: 0}
		}
	}}
	if got := exit.Code(runRoot(r, "pick", "land", "weft-hjx.1.1")); got != 2 {
		t.Fatalf("conflicts check failure must be exit 2 (Hardf), got %d", got)
	}
	for _, c := range r.calls {
		if strings.Contains(strings.Join(c, " "), "bd close") {
			t.Fatalf("must NOT bd close when conflicts check errors: %v", r.calls)
		}
	}
}

// fp0.4: sanitizeSubject strips newlines and control chars from a bead title.
func TestSanitizeSubject(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"normal title", "normal title"},
		{"line\none", "line one"},
		{"line\r\ntwo", "line two"},
		{"tab\there", "tab here"},
		{"ctrl\x01char", "ctrl char"},
		// Multiple consecutive control chars collapse to a single space.
		{"a\n\nz", "a z"},
		// Leading/trailing control chars are trimmed.
		{"\nleading", "leading"},
		{"trailing\n", "trailing"},
	}
	for _, tc := range cases {
		if got := sanitizeSubject(tc.in); got != tc.want {
			t.Errorf("sanitizeSubject(%q) = %q; want %q", tc.in, got, tc.want)
		}
	}
}

// fp0.4: pick seal with a newline in the title produces a single-line commit subject.
func TestPickSealSanitizesMultilineTitleInSubject(t *testing.T) {
	r := &routeRunner{fn: func(name string, args []string) run.Result {
		j := strings.Join(append([]string{name}, args...), " ")
		switch {
		case strings.Contains(j, "bd show"):
			// Title contains a JSON-encoded newline (\n); sanitizeSubject
			// must collapse it so the commit subject is a single line.
			return run.Result{Stdout: `[{"title":"Add X\nmore detail","status":"in_progress","labels":[]}]`, Code: 0}
		case strings.Contains(j, "log -r @-"):
			return run.Result{Stdout: "abc123def456\n", Code: 0}
		default:
			return run.Result{Code: 0}
		}
	}}
	if err := runRoot(r, "pick", "seal", "weft-hjx.9.1"); err != nil {
		t.Fatalf("execute: %v", err)
	}
	// The commit message must be a single line — no embedded newlines. Guard
	// against a vacuous pass: assert the jj commit -m call was actually made and
	// its subject is the sanitized single-line form, not merely "no bad call seen".
	var sawCommit bool
	for _, c := range r.calls {
		j := strings.Join(c, " ")
		if strings.Contains(j, "jj") && strings.Contains(j, "commit") && strings.Contains(j, "-m") {
			// Find the -m argument value.
			for i, tok := range c {
				if tok == "-m" && i+1 < len(c) {
					sawCommit = true
					msg := c[i+1]
					if strings.ContainsAny(msg, "\n\r\x01") {
						t.Errorf("commit subject contains control chars: %q", msg)
					}
					if msg != "feat(weft-hjx.9.1): Add X more detail" {
						t.Errorf("commit subject = %q; want sanitized single line", msg)
					}
				}
			}
		}
	}
	if !sawCommit {
		t.Errorf("expected a jj commit -m call; calls=%v", r.calls)
	}
}
