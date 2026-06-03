// internal/cli/plan_test.go
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

// writePlanFile writes a warp-plan.json into a temp dir and returns its path.
func writePlanFile(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "warp-plan.json")
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestPlanCheckValid(t *testing.T) {
	file := writePlanFile(t, `{"epic":{"title":"E"},"picks":[{"ref":"p1","title":"A","description":"do a"}]}`)
	r := &routeRunner{fn: func(string, []string) run.Result { return run.Result{} }}
	out, err := newTestCmd(r, "plan", "check", file, "--json")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(out.String(), `"valid": true`) {
		t.Errorf("expected valid:true, got %q", out.String())
	}
}

func TestPlanCheckInvalidStillExitsZero(t *testing.T) {
	file := writePlanFile(t, `{"epic":{},"picks":[]}`)
	r := &routeRunner{fn: func(string, []string) run.Result { return run.Result{} }}
	out, err := newTestCmd(r, "plan", "check", file, "--json")
	if err != nil {
		t.Fatalf("check must exit 0 even when invalid: %v", err)
	}
	if !strings.Contains(out.String(), `"valid": false`) {
		t.Errorf("expected valid:false, got %q", out.String())
	}
}

func TestPlanEmitDryRunNoMutation(t *testing.T) {
	// 2 shared files (x.go,y.go) > default overlap_max(1) => serialized edge b->a.
	file := writePlanFile(t, `{"epic":{"title":"E"},"picks":[{"ref":"a","title":"A","description":"a","files":["x.go","y.go"]},{"ref":"b","title":"B","description":"b","files":["x.go","y.go"]}]}`)
	r := &routeRunner{fn: func(string, []string) run.Result { return run.Result{} }}
	out, err := newTestCmd(r, "plan", "emit", file, "--dry-run", "--json")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(out.String(), `"dry_run": true`) {
		t.Errorf("expected dry_run:true: %q", out.String())
	}
	for _, c := range r.calls {
		if strings.Contains(strings.Join(c, " "), "bd create") {
			t.Fatalf("dry-run must not mutate: %v", r.calls)
		}
	}
}

func TestPlanEmitFirstCreatesGraph(t *testing.T) {
	file := writePlanFile(t, `{"epic":{"title":"E"},"picks":[{"ref":"a","title":"A","description":"a"}]}`)
	r := &routeRunner{fn: func(string, []string) run.Result { return run.Result{Stdout: "created weft-zzz", Code: 0} }}
	out, err := newTestCmd(r, "plan", "emit", file, "--json")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	saw := false
	for _, c := range r.calls {
		if strings.Contains(strings.Join(c, " "), "bd create --graph") {
			saw = true
		}
	}
	if !saw {
		t.Errorf("expected bd create --graph: %v", r.calls)
	}
	if !strings.Contains(out.String(), `"mode": "create"`) {
		t.Errorf("output: %q", out.String())
	}
}

func TestPlanEmitRefusesInvalidPlan(t *testing.T) {
	file := writePlanFile(t, `{"epic":{},"picks":[]}`)
	r := &routeRunner{fn: func(string, []string) run.Result { return run.Result{} }}
	if got := exit.Code(runRoot(r, "plan", "emit", file)); got != 1 {
		t.Fatalf("emit must reject invalid plan with exit 1, got %d", got)
	}
}
