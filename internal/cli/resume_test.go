// internal/cli/resume_test.go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Weft Contributors

package cli

import (
	"fmt"
	"strings"
	"testing"

	"github.com/seanb4t/weft/internal/exit"
	"github.com/seanb4t/weft/internal/run"
)

// fp0.15: resume without --epic must be exit 1 (Invocation error).
func TestResumeRequiresEpic(t *testing.T) {
	r := &routeRunner{fn: func(string, []string) run.Result { return run.Result{Code: 0} }}
	_, err := newTestCmd(r, "resume")
	if got := exit.Code(err); got != 1 {
		t.Fatalf("missing --epic must be exit 1, got %d (err=%v)", got, err)
	}
}

// fp0.13: beadIDsByStatus Hardf branch when bd list fails.
func TestBeadIDsByStatusHardfOnBdError(t *testing.T) {
	r := &routeRunner{fn: func(name string, args []string) run.Result {
		if name == "bd" {
			return run.Result{Code: 1, Stderr: "bd: list failed"}
		}
		return run.Result{Code: 0}
	}}
	_, err := beadIDsByStatus(r, "weft-hjx.1", "closed")
	if got := exit.Code(err); got != 2 {
		t.Fatalf("bd list failure must be exit 2 (Hardf), got %d (err=%v)", got, err)
	}
}

// fp0.13: beadIDsByStatus Hardf branch when runner itself fails to start.
func TestBeadIDsByStatusHardfOnRunnerError(t *testing.T) {
	r := &routeRunner{errFn: func(string, []string) error { return fmt.Errorf("bd not found") }}
	_, err := beadIDsByStatus(r, "weft-hjx.1", "closed")
	if got := exit.Code(err); got != 2 {
		t.Fatalf("runner error on bd list must be exit 2 (Hardf), got %d (err=%v)", got, err)
	}
}

// fp0.13: readyIDs Hardf branch when bd ready fails.
func TestReadyIDsHardfOnBdError(t *testing.T) {
	r := &routeRunner{fn: func(name string, args []string) run.Result {
		if name == "bd" {
			return run.Result{Code: 1, Stderr: "bd: ready failed"}
		}
		return run.Result{Code: 0}
	}}
	_, err := readyIDs(r, "weft-hjx.1")
	if got := exit.Code(err); got != 2 {
		t.Fatalf("bd ready failure must be exit 2 (Hardf), got %d (err=%v)", got, err)
	}
}

// fp0.13: readyIDs Hardf branch when runner fails to start.
func TestReadyIDsHardfOnRunnerError(t *testing.T) {
	r := &routeRunner{errFn: func(string, []string) error { return fmt.Errorf("bd not found") }}
	_, err := readyIDs(r, "weft-hjx.1")
	if got := exit.Code(err); got != 2 {
		t.Fatalf("runner error on bd ready must be exit 2 (Hardf), got %d (err=%v)", got, err)
	}
}

// fp0.13: conflictChanges Hardf branch when jj log fails.
// The mock lets epicChanges succeed (bd list returns a bead with jj-change:cha)
// so flow reaches the scoped jj log call, which then fails with Code:1.
func TestConflictChangesHardfOnJJError(t *testing.T) {
	r := &routeRunner{fn: func(name string, args []string) run.Result {
		j := strings.Join(append([]string{name}, args...), " ")
		if name == "bd" && strings.Contains(j, "list --parent") {
			return run.Result{Stdout: `[{"id":"weft-hjx.1.1","labels":["jj-change:cha"]}]`, Code: 0}
		}
		if name == "jj" {
			return run.Result{Code: 1, Stderr: "jj: revset error"}
		}
		return run.Result{Code: 0}
	}}
	_, err := conflictChanges(r, "weft-hjx.1")
	if got := exit.Code(err); got != 2 {
		t.Fatalf("jj log failure must be exit 2 (Hardf), got %d (err=%v)", got, err)
	}
}

// fp0.13: conflictChanges Hardf branch when runner fails to start.
// errFn fires on the first call (bd list for epicChanges) → still exit 2.
func TestConflictChangesHardfOnRunnerError(t *testing.T) {
	r := &routeRunner{errFn: func(string, []string) error { return fmt.Errorf("bd list or jj not found") }}
	_, err := conflictChanges(r, "weft-hjx.1")
	if got := exit.Code(err); got != 2 {
		t.Fatalf("runner error on bd list or jj log must be exit 2 (Hardf), got %d (err=%v)", got, err)
	}
}

// weft-hjx.6: conflictChanges scopes conflicts() to the epic's sealed beads.
// Two sealed beads with jj-change:cha / jj-change:chb → scoped revset used.
func TestConflictChangesScopedToEpicStack(t *testing.T) {
	var jjConflictsCall []string
	r := &routeRunner{fn: func(name string, args []string) run.Result {
		j := strings.Join(append([]string{name}, args...), " ")
		// epicChanges: bd list --parent weft-hjx.1 --json
		if name == "bd" && strings.Contains(j, "list --parent weft-hjx.1") && !strings.Contains(j, "--status") {
			return run.Result{
				Stdout: `[{"id":"weft-hjx.1.1","labels":["jj-change:cha"]},{"id":"weft-hjx.1.2","labels":["jj-change:chb"]}]`,
				Code:   0,
			}
		}
		// scoped jj log
		if name == "jj" && strings.Contains(j, "conflicts()") {
			jjConflictsCall = append([]string{name}, args...)
			return run.Result{Stdout: "cha\n", Code: 0}
		}
		return run.Result{Code: 0}
	}}
	got, err := conflictChanges(r, "weft-hjx.1")
	if err != nil {
		t.Fatalf("conflictChanges: %v", err)
	}
	if len(got) != 1 || got[0] != "cha" {
		t.Errorf("conflicts: want [cha], got %v", got)
	}
	// Assert the scoped revset was used, not bare conflicts().
	scopedRevset := "conflicts() & (cha | chb)"
	jjCall := strings.Join(jjConflictsCall, " ")
	if !strings.Contains(jjCall, scopedRevset) {
		t.Errorf("expected scoped revset %q in jj call, got: %q", scopedRevset, jjCall)
	}
}

// weft-hjx.6: no sealed beads → conflicts: [] and jj conflicts() NOT called.
func TestConflictChangesEmptyWhenNoSealedBeads(t *testing.T) {
	r := &routeRunner{fn: func(name string, args []string) run.Result {
		j := strings.Join(append([]string{name}, args...), " ")
		// epicChanges: return beads with no jj-change labels
		if name == "bd" && strings.Contains(j, "list --parent weft-hjx.2") && !strings.Contains(j, "--status") {
			return run.Result{Stdout: `[{"id":"weft-hjx.2.1","labels":["status:open"]}]`, Code: 0}
		}
		return run.Result{Code: 0}
	}}
	got, err := conflictChanges(r, "weft-hjx.2")
	if err != nil {
		t.Fatalf("conflictChanges: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected no conflicts, got %v", got)
	}
	// Assert jj conflicts() was NOT called.
	for _, c := range r.calls {
		if c[0] == "jj" && strings.Contains(strings.Join(c, " "), "conflicts()") {
			t.Errorf("jj conflicts() must NOT be called when no sealed beads, but got: %v", c)
		}
	}
}

// fp0.15: resume halts and returns Hardf when a mid-pipeline subprocess fails.
func TestResumeMidPipelineErrorHalts(t *testing.T) {
	// Fail the bd list (beadIDsByStatus for "closed") — resume must exit 2 and
	// must not proceed to later subprocess calls (readyIDs, conflictChanges, etc.).
	r := &routeRunner{fn: func(name string, args []string) run.Result {
		j := strings.Join(append([]string{name}, args...), " ")
		if strings.Contains(j, "bd list") && strings.Contains(j, "closed") {
			return run.Result{Code: 1, Stderr: "bd: db unavailable"}
		}
		return run.Result{Code: 0}
	}}
	_, err := newTestCmd(r, "resume", "--epic", "weft-hjx.1")
	if got := exit.Code(err); got != 2 {
		t.Fatalf("mid-pipeline bd error must propagate as exit 2 (Hardf), got %d (err=%v)", got, err)
	}
	// Must not have called jj (conflicts check) — pipeline halted.
	for _, c := range r.calls {
		if c[0] == "jj" && strings.Contains(strings.Join(c, " "), "conflicts()") {
			t.Errorf("resume must halt on mid-pipeline error, but still called jj conflicts(): %v", c)
		}
	}
}

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
		// epicChanges: bd list --parent weft-hjx.1 --json (no --status)
		case name == "bd" && strings.Contains(j, "list --parent weft-hjx.1") && !strings.Contains(j, "--status"):
			return run.Result{
				Stdout: `[{"id":"weft-hjx.1.1","labels":["jj-change:cha"]},{"id":"weft-hjx.1.5","labels":["jj-change:chb"]}]`,
				Code:   0,
			}
		// scoped jj conflicts() returns empty — no conflicts in this epic
		case strings.Contains(j, "conflicts() & (cha | chb)"):
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
	// Hard invariant: resume must NOT mutate. Scan every token of every recorded
	// call against an exact mutating-verb set — exact-match (not substring) so
	// "--status closed" is not mistaken for "close", and per-token (not c[1]) so
	// jj's "--no-pager"-prefixed calls (verb at c[2]) are still checked.
	mutating := map[string]bool{"update": true, "close": true, "rebase": true, "abandon": true}
	for _, c := range r.calls {
		for _, tok := range c {
			if mutating[tok] {
				t.Fatalf("resume must be read-only, saw mutating call: %v", c)
			}
		}
	}
}
