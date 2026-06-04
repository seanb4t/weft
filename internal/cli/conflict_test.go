// internal/cli/conflict_test.go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Weft Contributors

package cli

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/seanb4t/weft/internal/exit"
	"github.com/seanb4t/weft/internal/run"
	"github.com/seanb4t/weft/internal/workspace"
)

func TestConflictOpenCreatesResolveWorkspace(t *testing.T) {
	r := &routeRunner{fn: func(name string, args []string) run.Result {
		j := strings.Join(append([]string{name}, args...), " ")
		switch {
		case strings.Contains(j, "bd show weft-hjx.4.2"):
			return run.Result{Stdout: `[{"title":"b","status":"in_progress","labels":["jj-change:chb"]}]`, Code: 0}
		case strings.Contains(j, "jj") && strings.Contains(j, "root"):
			return run.Result{Stdout: "/repo/weft", Code: 0}
		case strings.Contains(j, "conflicts() & chb"):
			return run.Result{Stdout: "chb\n", Code: 0} // chb IS conflicted -> proceed
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
		if strings.Contains(j, "workspace add") && strings.Contains(j, "weft-hjx__4__2-resolve") && strings.Contains(j, "-r chb") {
			sawAdd = true
		}
		if strings.Contains(j, "config set --repo ui.conflict-marker-style diff") {
			sawMarker = true
		}
	}
	if !sawAdd {
		t.Errorf("expected workspace add of weft-hjx__4__2-resolve at -r chb; calls=%v", r.calls)
	}
	if !sawMarker {
		t.Errorf("expected ui.conflict-marker-style=diff; calls=%v", r.calls)
	}
	if !strings.Contains(out.String(), `"change": "chb"`) {
		t.Errorf("brief missing change: %q", out.String())
	}
}

func TestConflictOpenRefusesUnconflictedChange(t *testing.T) {
	r := &routeRunner{fn: func(name string, args []string) run.Result {
		j := strings.Join(append([]string{name}, args...), " ")
		switch {
		case strings.Contains(j, "bd show"):
			return run.Result{Stdout: `[{"title":"b","status":"in_progress","labels":["jj-change:chb"]}]`, Code: 0}
		case strings.Contains(j, "jj") && strings.Contains(j, "root"):
			return run.Result{Stdout: "/repo/weft", Code: 0}
		case strings.Contains(j, "conflicts() & chb"):
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

// TestConflictFinalizeSquashesAndReaps verifies the happy path: resolver cleared
// markers, squash folds the resolution in, workspace is reaped, healed=[chb].
// F3: uses t.TempDir() as jj root so os.Stat(resolve path) succeeds.
// F8: structurally decodes the envelope to assert healed/remaining_conflicts shape.
func TestConflictFinalizeSquashesAndReaps(t *testing.T) {
	root := t.TempDir()
	// Compute the resolve path and create it so the existence check passes (F3).
	resolvePath := workspace.ResolvePath(root, "", "weft-hjx.4.2")
	if err := os.MkdirAll(resolvePath, 0o755); err != nil {
		t.Fatalf("mkdir resolve path: %v", err)
	}
	r := &routeRunner{fn: func(name string, args []string) run.Result {
		j := strings.Join(append([]string{name}, args...), " ")
		switch {
		case strings.Contains(j, "bd show weft-hjx.4.2"):
			return run.Result{Stdout: `[{"title":"b","status":"in_progress","labels":["jj-change:chb"]}]`, Code: 0}
		case strings.Contains(j, "jj") && strings.Contains(j, "root"):
			return run.Result{Stdout: root, Code: 0}
		case strings.Contains(j, "conflicts() & weft-hjx__4__2-resolve@"):
			return run.Result{Stdout: "", Code: 0} // resolver cleared the markers
		case strings.Contains(j, "diff --git -r weft-hjx__4__2-resolve@"):
			return run.Result{Stdout: "diff --git a/x b/x\n+fixed\n", Code: 0} // non-empty resolution
		case strings.Contains(j, "conflicts()") && strings.Contains(j, "descendants(chb)"):
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
		if strings.Contains(j, "squash --from weft-hjx__4__2-resolve@ --into chb") {
			sawSquash = true
		}
		if strings.Contains(j, "workspace forget weft-hjx__4__2-resolve") {
			sawForget = true
		}
	}
	if !sawSquash {
		t.Errorf("expected squash --from <resolve>@ --into chb; calls=%v", r.calls)
	}
	if !sawForget {
		t.Errorf("expected reap (workspace forget) of the resolution workspace; calls=%v", r.calls)
	}
	// F8: structural decode — assert healed=[chb] and remaining_conflicts=[] specifically.
	// A bare substring check on {bead,change} is insufficient because stack[] also carries
	// those pairs. Decoding into a struct fails outright on a wrong shape.
	var env struct {
		Data struct {
			Healed             []string `json:"healed"`
			RemainingConflicts []string `json:"remaining_conflicts"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(out.String()), &env); err != nil {
		t.Fatalf("decode envelope: %v; out=%q", err, out.String())
	}
	if len(env.Data.Healed) != 1 || env.Data.Healed[0] != "chb" {
		t.Errorf("healed = %v; want [chb]", env.Data.Healed)
	}
	if len(env.Data.RemainingConflicts) != 0 {
		t.Errorf("remaining_conflicts = %v; want []", env.Data.RemainingConflicts)
	}
}

func TestConflictFinalizeEscalatesWhenStillConflicted(t *testing.T) {
	root := t.TempDir()
	// Compute the resolve path and create it so the existence check passes (F3).
	resolvePath := workspace.ResolvePath(root, "", "weft-hjx.4.2")
	if err := os.MkdirAll(resolvePath, 0o755); err != nil {
		t.Fatalf("mkdir resolve path: %v", err)
	}
	r := &routeRunner{fn: func(name string, args []string) run.Result {
		j := strings.Join(append([]string{name}, args...), " ")
		switch {
		case strings.Contains(j, "bd show"):
			return run.Result{Stdout: `[{"title":"b","status":"in_progress","labels":["jj-change:chb"]}]`, Code: 0}
		case strings.Contains(j, "jj") && strings.Contains(j, "root"):
			return run.Result{Stdout: root, Code: 0}
		case strings.Contains(j, "conflicts() & weft-hjx__4__2-resolve@"):
			return run.Result{Stdout: "chb\n", Code: 0} // STILL conflicted -> escalate
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

// TestConflictFinalizeRefusesEmptyResolution verifies the empty-diff gate (F3/§6):
// the workspace exists and markers are cleared, but the resolver made no edits, so
// `jj diff -r <resolve>@` is empty. finalize must exit 1 (Invocation) WITHOUT
// squashing or reaping — a no-op resolution must not destroy the workspace.
func TestConflictFinalizeRefusesEmptyResolution(t *testing.T) {
	root := t.TempDir()
	resolvePath := workspace.ResolvePath(root, "", "weft-hjx.4.2")
	if err := os.MkdirAll(resolvePath, 0o755); err != nil {
		t.Fatalf("mkdir resolve path: %v", err)
	}
	r := &routeRunner{fn: func(name string, args []string) run.Result {
		j := strings.Join(append([]string{name}, args...), " ")
		switch {
		case strings.Contains(j, "bd show weft-hjx.4.2"):
			return run.Result{Stdout: `[{"title":"b","status":"in_progress","labels":["jj-change:chb"]}]`, Code: 0}
		case strings.Contains(j, "jj") && strings.Contains(j, "root"):
			return run.Result{Stdout: root, Code: 0}
		case strings.Contains(j, "conflicts() & weft-hjx__4__2-resolve@"):
			return run.Result{Stdout: "", Code: 0} // markers cleared -> not conflicted
		case strings.Contains(j, "diff --git -r weft-hjx__4__2-resolve@"):
			return run.Result{Stdout: "", Code: 0} // EMPTY -> resolver made no edits
		default:
			return run.Result{Code: 0}
		}
	}}
	if got := exit.Code(runRoot(r, "conflict", "finalize", "weft-hjx.4.2")); got != 1 {
		t.Fatalf("empty resolution must be exit 1 (Invocation), got %d", got)
	}
	for _, c := range r.calls {
		j := strings.Join(c, " ")
		if strings.Contains(j, "squash") || strings.Contains(j, "workspace forget") {
			t.Errorf("must NOT squash/reap an empty resolution: %v", r.calls)
		}
	}
}

// TestChangeConflictedRejectsUnsafeRev verifies that changeConflicted short-circuits
// before any jj invocation when the revision contains revset metacharacters.
func TestChangeConflictedRejectsUnsafeRev(t *testing.T) {
	badRevs := []string{"all()", "x & y", "a::b", "a|b", "..", "a b", "@", ""}
	for _, bad := range badRevs {
		t.Run(bad, func(t *testing.T) {
			r := &routeRunner{fn: func(name string, args []string) run.Result {
				return run.Result{Code: 0}
			}}
			_, err := changeConflicted(r, bad)
			if err == nil {
				t.Fatalf("changeConflicted(%q) returned nil error; want exit-2", bad)
			}
			if code := exit.Code(err); code != 2 {
				t.Errorf("changeConflicted(%q) exit code = %d; want 2", bad, code)
			}
			if len(r.calls) != 0 {
				t.Errorf("changeConflicted(%q) must not invoke jj; got calls=%v", bad, r.calls)
			}
		})
	}
}

// TestChangeConflictedAcceptsValidRevShapes verifies that a bare change-id and a
// workspace working-copy ref both pass the allowlist and reach jj.
func TestChangeConflictedAcceptsValidRevShapes(t *testing.T) {
	validRevs := []string{"kxqpmsqz", "weft-hjx__4__2-resolve@"}
	for _, rev := range validRevs {
		t.Run(rev, func(t *testing.T) {
			r := &routeRunner{fn: func(name string, args []string) run.Result {
				return run.Result{Stdout: "", Code: 0}
			}}
			got, err := changeConflicted(r, rev)
			if err != nil {
				t.Fatalf("changeConflicted(%q) returned error: %v", rev, err)
			}
			if got != false {
				t.Errorf("changeConflicted(%q) = %v; want false (empty stdout)", rev, got)
			}
			if len(r.calls) == 0 {
				t.Errorf("changeConflicted(%q) must invoke jj; got no calls", rev)
			}
		})
	}
}

// TestScopedConflictChangesRejectsUnsafeRev verifies that scopedConflictChanges
// short-circuits before any jj invocation when rootChange contains metacharacters.
func TestScopedConflictChangesRejectsUnsafeRev(t *testing.T) {
	badRevs := []string{"descendants(all())", "a & b", "all()", ""}
	for _, bad := range badRevs {
		t.Run(bad, func(t *testing.T) {
			r := &routeRunner{fn: func(name string, args []string) run.Result {
				return run.Result{Code: 0}
			}}
			_, err := scopedConflictChanges(r, bad)
			if err == nil {
				t.Fatalf("scopedConflictChanges(%q) returned nil error; want exit-2", bad)
			}
			if code := exit.Code(err); code != 2 {
				t.Errorf("scopedConflictChanges(%q) exit code = %d; want 2", bad, code)
			}
			if len(r.calls) != 0 {
				t.Errorf("scopedConflictChanges(%q) must not invoke jj; got calls=%v", bad, r.calls)
			}
		})
	}
}

// TestConflictFinalizeRequiresOpenWorkspace verifies that calling finalize without
// a prior `conflict open` (no resolve workspace on disk) returns exit 1 (Invocation)
// and does NOT attempt squash or workspace forget. (F3)
func TestConflictFinalizeRequiresOpenWorkspace(t *testing.T) {
	root := t.TempDir()
	// The resolve path is deliberately NOT created — simulates a missing workspace.
	r := &routeRunner{fn: func(name string, args []string) run.Result {
		j := strings.Join(append([]string{name}, args...), " ")
		switch {
		case strings.Contains(j, "bd show weft-hjx.4.2"):
			return run.Result{Stdout: `[{"title":"b","status":"in_progress","labels":["jj-change:chb"]}]`, Code: 0}
		case strings.Contains(j, "jj") && strings.Contains(j, "root"):
			return run.Result{Stdout: root, Code: 0}
		default:
			return run.Result{Code: 0}
		}
	}}
	if got := exit.Code(runRoot(r, "conflict", "finalize", "weft-hjx.4.2")); got != 1 {
		t.Fatalf("finalize without open workspace must be exit 1 (Invocation), got %d", got)
	}
	for _, c := range r.calls {
		j := strings.Join(c, " ")
		if strings.Contains(j, "workspace forget") || strings.Contains(j, "squash") {
			t.Errorf("must NOT call squash/workspace forget when resolve path missing: %v", r.calls)
		}
	}
}
