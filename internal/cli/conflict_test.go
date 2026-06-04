// internal/cli/conflict_test.go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Weft Contributors

package cli

import (
	"strings"
	"testing"

	"github.com/seanb4t/weft/internal/exit"
	"github.com/seanb4t/weft/internal/run"
)

func TestConflictOpenCreatesResolveWorkspace(t *testing.T) {
	r := &routeRunner{fn: func(name string, args []string) run.Result {
		j := strings.Join(append([]string{name}, args...), " ")
		switch {
		case strings.Contains(j, "bd show weft-hjx.4.2"):
			return run.Result{Stdout: `[{"title":"b","status":"in_progress","labels":["jj-change:chB"]}]`, Code: 0}
		case strings.Contains(j, "jj") && strings.Contains(j, "root"):
			return run.Result{Stdout: "/repo/weft", Code: 0}
		case strings.Contains(j, "conflicts() & chB"):
			return run.Result{Stdout: "chB\n", Code: 0} // chB IS conflicted -> proceed
		default: // workspace add, config set
			return run.Result{Code: 0}
		}
	}}
	out, err := newTestCmd(r, "conflict", "open", "weft-hjx.4.2", "--json")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	var sawAdd, sawMarker bool
	for _, c := range r.calls {
		j := strings.Join(c, " ")
		if strings.Contains(j, "workspace add") && strings.Contains(j, "weft-hjx__4__2-resolve") && strings.Contains(j, "-r chB") {
			sawAdd = true
		}
		if strings.Contains(j, "config set --repo ui.conflict-marker-style diff") {
			sawMarker = true
		}
	}
	if !sawAdd {
		t.Errorf("expected workspace add of weft-hjx__4__2-resolve at -r chB; calls=%v", r.calls)
	}
	if !sawMarker {
		t.Errorf("expected ui.conflict-marker-style=diff; calls=%v", r.calls)
	}
	if !strings.Contains(out.String(), `"change": "chB"`) {
		t.Errorf("brief missing change: %q", out.String())
	}
}

func TestConflictOpenRefusesUnconflictedChange(t *testing.T) {
	r := &routeRunner{fn: func(name string, args []string) run.Result {
		j := strings.Join(append([]string{name}, args...), " ")
		switch {
		case strings.Contains(j, "bd show"):
			return run.Result{Stdout: `[{"title":"b","status":"in_progress","labels":["jj-change:chB"]}]`, Code: 0}
		case strings.Contains(j, "jj") && strings.Contains(j, "root"):
			return run.Result{Stdout: "/repo/weft", Code: 0}
		case strings.Contains(j, "conflicts() & chB"):
			return run.Result{Stdout: "", Code: 0} // NOT conflicted
		default:
			return run.Result{Code: 0}
		}
	}}
	if got := exit.Code(runRoot(r, "conflict", "open", "weft-hjx.4.2")); got != 1 {
		t.Fatalf("opening a non-conflicted change must be exit 1, got %d", got)
	}
	for _, c := range r.calls {
		if strings.Contains(strings.Join(c, " "), "workspace add") {
			t.Fatalf("must NOT create a workspace for a non-conflicted change: %v", r.calls)
		}
	}
}

func TestConflictFinalizeSquashesAndReaps(t *testing.T) {
	r := &routeRunner{fn: func(name string, args []string) run.Result {
		j := strings.Join(append([]string{name}, args...), " ")
		switch {
		case strings.Contains(j, "bd show weft-hjx.4.2"):
			return run.Result{Stdout: `[{"title":"b","status":"in_progress","labels":["jj-change:chB"]}]`, Code: 0}
		case strings.Contains(j, "jj") && strings.Contains(j, "root"):
			return run.Result{Stdout: "/repo/weft", Code: 0}
		case strings.Contains(j, "conflicts() & weft-hjx__4__2-resolve@"):
			return run.Result{Stdout: "", Code: 0} // resolver cleared the markers
		case strings.Contains(j, "diff --git -r weft-hjx__4__2-resolve@"):
			return run.Result{Stdout: "diff --git a/x b/x\n+fixed\n", Code: 0} // non-empty resolution
		case strings.Contains(j, "log -r conflicts()"):
			return run.Result{Stdout: "", Code: 0} // post-squash: nothing conflicted -> healed
		default: // squash, workspace forget
			return run.Result{Code: 0}
		}
	}}
	out, err := newTestCmd(r, "conflict", "finalize", "weft-hjx.4.2", "--json")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	var sawSquash, sawForget bool
	for _, c := range r.calls {
		j := strings.Join(c, " ")
		if strings.Contains(j, "squash --from weft-hjx__4__2-resolve@ --into chB") {
			sawSquash = true
		}
		if strings.Contains(j, "workspace forget weft-hjx__4__2-resolve") {
			sawForget = true
		}
	}
	if !sawSquash {
		t.Errorf("expected squash --from <resolve>@ --into chB; calls=%v", r.calls)
	}
	if !sawForget {
		t.Errorf("expected reap (workspace forget) of the resolution workspace; calls=%v", r.calls)
	}
	if !strings.Contains(out.String(), `"healed"`) || !strings.Contains(out.String(), "chB") {
		t.Errorf("expected healed:[chB] in output: %q", out.String())
	}
}

func TestConflictFinalizeEscalatesWhenStillConflicted(t *testing.T) {
	r := &routeRunner{fn: func(name string, args []string) run.Result {
		j := strings.Join(append([]string{name}, args...), " ")
		switch {
		case strings.Contains(j, "bd show"):
			return run.Result{Stdout: `[{"title":"b","status":"in_progress","labels":["jj-change:chB"]}]`, Code: 0}
		case strings.Contains(j, "jj") && strings.Contains(j, "root"):
			return run.Result{Stdout: "/repo/weft", Code: 0}
		case strings.Contains(j, "conflicts() & weft-hjx__4__2-resolve@"):
			return run.Result{Stdout: "chB\n", Code: 0} // STILL conflicted -> escalate
		default:
			return run.Result{Code: 0}
		}
	}}
	out, err := newTestCmd(r, "conflict", "finalize", "weft-hjx.4.2", "--json")
	if err != nil {
		t.Fatalf("finalize must exit 0 even when escalating (verdict is data): %v", err)
	}
	var sawHuman, sawSquash bool
	for _, c := range r.calls {
		j := strings.Join(c, " ")
		if strings.Contains(j, "bd update weft-hjx.4.2 --add-label human") {
			sawHuman = true
		}
		if strings.Contains(j, "squash") {
			sawSquash = true
		}
	}
	if !sawHuman {
		t.Errorf("expected bd update --add-label human escalation; calls=%v", r.calls)
	}
	if sawSquash {
		t.Fatalf("must NOT squash a still-conflicted resolution: %v", r.calls)
	}
	if !strings.Contains(out.String(), `"escalated": true`) {
		t.Errorf("expected escalated:true: %q", out.String())
	}
}
