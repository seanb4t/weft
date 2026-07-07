// internal/cli/status_test.go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Weft Contributors

package cli

import (
	"strings"
	"testing"

	"github.com/seanb4t/weft/internal/exit"
	"github.com/seanb4t/weft/internal/run"
)

// --- pure counting helper (TDD core) ---

func TestCountByStatusMixed(t *testing.T) {
	children := []warpChild{
		{ID: "a", Status: "closed"},
		{ID: "b", Status: "closed"},
		{ID: "c", Status: "in_progress"},
		{ID: "d", Status: "blocked"},
		{ID: "e", Status: "open"},
		{ID: "f", Status: "open"},
		{ID: "g", Status: "open"},
	}
	got := countByStatus(children)
	want := statusCounts{Closed: 2, InProgress: 1, Blocked: 1, Open: 3}
	if got != want {
		t.Errorf("countByStatus mixed = %+v, want %+v", got, want)
	}
	if got.done() != 2 {
		t.Errorf("done() = %d, want 2", got.done())
	}
	if got.remaining() != 5 {
		t.Errorf("remaining() = %d, want 5", got.remaining())
	}
}

func TestCountByStatusEmpty(t *testing.T) {
	got := countByStatus(nil)
	want := statusCounts{}
	if got != want {
		t.Errorf("countByStatus empty = %+v, want zero value", got)
	}
	if got.done() != 0 || got.remaining() != 0 {
		t.Errorf("done/remaining on empty set must be 0, got done=%d remaining=%d", got.done(), got.remaining())
	}
}

// bd's status vocabulary may grow; an unrecognised value must not crash or
// skew the counts it does understand.
func TestCountByStatusUnknownStatusIgnored(t *testing.T) {
	children := []warpChild{
		{ID: "a", Status: "closed"},
		{ID: "b", Status: "hooked"},
	}
	got := countByStatus(children)
	want := statusCounts{Closed: 1}
	if got != want {
		t.Errorf("countByStatus with unknown status = %+v, want %+v", got, want)
	}
}

// --- command-level table tests (fake Runner idiom, per resume_test.go) ---

func TestStatusOverviewJSONShape(t *testing.T) {
	r := &routeRunner{fn: func(name string, args []string) run.Result {
		j := strings.Join(append([]string{name}, args...), " ")
		switch {
		case strings.Contains(j, "list --type epic"):
			return run.Result{Stdout: `[{"id":"weft-aaa","title":"Epic A","status":"open"},{"id":"weft-bbb","title":"Epic B","status":"open"}]`, Code: 0}
		case strings.Contains(j, "list --parent weft-aaa"):
			return run.Result{Stdout: `[{"id":"weft-aaa.1","title":"pick 1","status":"closed"},{"id":"weft-aaa.2","title":"pick 2","status":"open"}]`, Code: 0}
		case strings.Contains(j, "list --parent weft-bbb"):
			return run.Result{Stdout: `[{"id":"weft-bbb.1","title":"pick 1","status":"in_progress"}]`, Code: 0}
		default:
			return run.Result{Code: 0}
		}
	}}
	out, err := newTestCmd(r, "status", "--json")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	s := out.String()
	for _, want := range []string{`"epics"`, `"aggregate"`, "weft-aaa", "weft-bbb"} {
		if !strings.Contains(s, want) {
			t.Errorf("status --json missing %q: %q", want, s)
		}
	}

	pickOut, err := newTestCmd(r, "status", "--pick", "data.epics[0].counts.closed")
	if err != nil {
		t.Fatalf("execute --pick: %v", err)
	}
	if got := strings.TrimSpace(pickOut.String()); got != "1" {
		t.Errorf("--pick data.epics[0].counts.closed = %q, want %q", got, "1")
	}
}

func TestStatusOverviewEmptyWarp(t *testing.T) {
	r := &routeRunner{fn: func(name string, args []string) run.Result {
		if strings.Contains(strings.Join(append([]string{name}, args...), " "), "list --type epic") {
			return run.Result{Stdout: `[]`, Code: 0}
		}
		return run.Result{Code: 0}
	}}
	out, err := newTestCmd(r, "status")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(out.String(), "no epics") {
		t.Errorf("empty warp text = %q, want it to mention no epics", out.String())
	}

	jsonOut, err := newTestCmd(r, "status", "--json")
	if err != nil {
		t.Fatalf("execute --json: %v", err)
	}
	if s := jsonOut.String(); !strings.Contains(s, `"epics": []`) {
		t.Errorf("empty warp --json missing epics: [], got %q", s)
	}
}

func TestStatusEpicDrillDown(t *testing.T) {
	r := &routeRunner{fn: func(name string, args []string) run.Result {
		j := strings.Join(append([]string{name}, args...), " ")
		switch {
		case name == "bd" && len(args) >= 2 && args[0] == "show" && args[1] == "weft-ccc":
			return run.Result{Stdout: `[{"title":"Epic C","status":"open","labels":[]}]`, Code: 0}
		case strings.Contains(j, "list --parent weft-ccc"):
			return run.Result{Stdout: `[{"id":"weft-ccc.1","title":"pick one","status":"closed"},{"id":"weft-ccc.2","title":"pick two","status":"blocked"}]`, Code: 0}
		default:
			return run.Result{Code: 0}
		}
	}}
	out, err := newTestCmd(r, "status", "weft-ccc")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	s := out.String()
	for _, want := range []string{"Epic C", "closed:", "weft-ccc.1", "pick one", "blocked:", "weft-ccc.2", "pick two"} {
		if !strings.Contains(s, want) {
			t.Errorf("drill-down text missing %q: %q", want, s)
		}
	}
}

func TestStatusEpicDrillDownEmptyEpic(t *testing.T) {
	r := &routeRunner{fn: func(name string, args []string) run.Result {
		j := strings.Join(append([]string{name}, args...), " ")
		switch {
		case name == "bd" && len(args) >= 2 && args[0] == "show":
			return run.Result{Stdout: `[{"title":"Epic Empty","status":"open","labels":[]}]`, Code: 0}
		case strings.Contains(j, "list --parent"):
			return run.Result{Stdout: `[]`, Code: 0}
		default:
			return run.Result{Code: 0}
		}
	}}
	out, err := newTestCmd(r, "status", "weft-empty")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(out.String(), "Epic Empty") {
		t.Errorf("empty-epic drill text = %q, want it to still show the epic header", out.String())
	}
}

func TestStatusInvalidEpicID(t *testing.T) {
	r := &routeRunner{fn: func(string, []string) run.Result { return run.Result{Code: 0} }}
	_, err := newTestCmd(r, "status", "..")
	if got := exit.Code(err); got != 1 {
		t.Fatalf("invalid epic id must be exit 1, got %d (err=%v)", got, err)
	}
}

// status must never call a mutating bd/jj verb (read-only invariant, spec).
func TestStatusReadOnly(t *testing.T) {
	r := &routeRunner{fn: func(name string, args []string) run.Result {
		j := strings.Join(append([]string{name}, args...), " ")
		switch {
		case strings.Contains(j, "list --type epic"):
			return run.Result{Stdout: `[{"id":"weft-ddd","title":"Epic D","status":"open"}]`, Code: 0}
		case name == "bd" && len(args) >= 1 && args[0] == "show":
			return run.Result{Stdout: `[{"title":"Epic D","status":"open","labels":[]}]`, Code: 0}
		case strings.Contains(j, "list --parent"):
			return run.Result{Stdout: `[{"id":"weft-ddd.1","title":"pick","status":"open"}]`, Code: 0}
		default:
			return run.Result{Code: 0}
		}
	}}
	if _, err := newTestCmd(r, "status"); err != nil {
		t.Fatalf("execute overview: %v", err)
	}
	if _, err := newTestCmd(r, "status", "weft-ddd"); err != nil {
		t.Fatalf("execute drill: %v", err)
	}
	mutating := map[string]bool{"update": true, "close": true, "rebase": true, "abandon": true}
	for _, c := range r.calls {
		for _, tok := range c {
			if mutating[tok] {
				t.Fatalf("status must be read-only, saw mutating call: %v", c)
			}
		}
	}
}

func TestStatusOverviewHardfOnEpicListError(t *testing.T) {
	r := &routeRunner{fn: func(name string, args []string) run.Result {
		if strings.Contains(strings.Join(append([]string{name}, args...), " "), "list --type epic") {
			return run.Result{Code: 1, Stderr: "bd: db unavailable"}
		}
		return run.Result{Code: 0}
	}}
	_, err := newTestCmd(r, "status")
	if got := exit.Code(err); got != 2 {
		t.Fatalf("bd list --type epic failure must be exit 2 (Hardf), got %d (err=%v)", got, err)
	}
}

func TestStatusOverviewHardfOnChildrenError(t *testing.T) {
	r := &routeRunner{fn: func(name string, args []string) run.Result {
		j := strings.Join(append([]string{name}, args...), " ")
		if strings.Contains(j, "list --type epic") {
			return run.Result{Stdout: `[{"id":"weft-eee","title":"Epic E","status":"open"}]`, Code: 0}
		}
		if strings.Contains(j, "list --parent weft-eee") {
			return run.Result{Code: 1, Stderr: "bd: list failed"}
		}
		return run.Result{Code: 0}
	}}
	_, err := newTestCmd(r, "status")
	if got := exit.Code(err); got != 2 {
		t.Fatalf("epic children failure must be exit 2 (Hardf), got %d (err=%v)", got, err)
	}
}

func TestStatusEpicHardfOnShowError(t *testing.T) {
	r := &routeRunner{fn: func(name string, args []string) run.Result {
		if name == "bd" && len(args) >= 1 && args[0] == "show" {
			return run.Result{Code: 1, Stdout: `{"error":"no issues found"}`}
		}
		return run.Result{Code: 0}
	}}
	_, err := newTestCmd(r, "status", "weft-fff")
	if got := exit.Code(err); got != 2 {
		t.Fatalf("bd show failure must be exit 2 (Hardf), got %d (err=%v)", got, err)
	}
}

func TestStatusEpicHardfOnChildrenError(t *testing.T) {
	r := &routeRunner{fn: func(name string, args []string) run.Result {
		j := strings.Join(append([]string{name}, args...), " ")
		if name == "bd" && len(args) >= 1 && args[0] == "show" {
			return run.Result{Stdout: `[{"title":"Epic G","status":"open","labels":[]}]`, Code: 0}
		}
		if strings.Contains(j, "list --parent weft-ggg") {
			return run.Result{Code: 1, Stderr: "bd: list failed"}
		}
		return run.Result{Code: 0}
	}}
	_, err := newTestCmd(r, "status", "weft-ggg")
	if got := exit.Code(err); got != 2 {
		t.Fatalf("epic children failure must be exit 2 (Hardf), got %d (err=%v)", got, err)
	}
}
