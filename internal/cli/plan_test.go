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

func TestPlanEmitReplanUpsertsMatchedRef(t *testing.T) {
	file := writePlanFile(t, `{"epic":{"title":"E"},"picks":[{"ref":"a","title":"A2","description":"updated"}]}`)
	r := &routeRunner{fn: func(name string, args []string) run.Result {
		j := strings.Join(append([]string{name}, args...), " ")
		if strings.Contains(j, "bd list --parent weft-hjx.9") {
			return run.Result{Stdout: `[{"id":"weft-hjx.9.1","status":"open","labels":["weft-ref:a"]}]`, Code: 0}
		}
		return run.Result{Code: 0}
	}}
	out, err := newTestCmd(r, "plan", "emit", file, "--epic", "weft-hjx.9", "--json")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	sawImport := false
	for _, c := range r.calls {
		if strings.Contains(strings.Join(c, " "), "bd import") {
			sawImport = true
		}
	}
	if !sawImport {
		t.Errorf("expected bd import: %v", r.calls)
	}
	if !strings.Contains(out.String(), `"mode": "upsert"`) {
		t.Errorf("output: %q", out.String())
	}
}

func TestPlanEmitReplanDryRunReportsDeltas(t *testing.T) {
	// ref "a" matches an existing bead; "new" is created; existing "gone" is removed.
	file := writePlanFile(t, `{"epic":{"title":"E"},"picks":[{"ref":"a","title":"A","description":"a"},{"ref":"new","title":"N","description":"n"}]}`)
	r := &routeRunner{fn: func(name string, args []string) run.Result {
		if strings.Contains(strings.Join(append([]string{name}, args...), " "), "bd list") {
			return run.Result{Stdout: `[{"id":"e.1","status":"open","labels":["weft-ref:a"]},{"id":"e.2","status":"closed","labels":["weft-ref:gone"]}]`, Code: 0}
		}
		return run.Result{Code: 0}
	}}
	out, err := newTestCmd(r, "plan", "emit", file, "--epic", "e", "--dry-run", "--json")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	s := out.String()
	for _, want := range []string{`"updated"`, `"created"`, `"removed"`, "gone", "new"} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %q in %q", want, s)
		}
	}
	for _, c := range r.calls {
		if strings.Contains(strings.Join(c, " "), "bd import") {
			t.Fatalf("dry-run must not import: %v", r.calls)
		}
	}
}

func TestPlanEmitCreateGraphNonZeroExitIsHard(t *testing.T) {
	file := writePlanFile(t, `{"epic":{"title":"E"},"picks":[{"ref":"a","title":"A","description":"a"}]}`)
	r := &routeRunner{fn: func(name string, args []string) run.Result {
		if strings.Contains(strings.Join(append([]string{name}, args...), " "), "bd create --graph") {
			return run.Result{Code: 1, Stderr: "create boom"}
		}
		return run.Result{Code: 0}
	}}
	if got := exit.Code(runRoot(r, "plan", "emit", file)); got != 2 {
		t.Fatalf("bd create --graph failure must be a hard error (exit 2), got %d", got)
	}
}

func TestPlanEmitImportNonZeroExitIsHard(t *testing.T) {
	file := writePlanFile(t, `{"epic":{"title":"E"},"picks":[{"ref":"a","title":"A","description":"a"}]}`)
	r := &routeRunner{fn: func(name string, args []string) run.Result {
		j := strings.Join(append([]string{name}, args...), " ")
		if strings.Contains(j, "bd list") {
			return run.Result{Stdout: `[{"id":"e.1","status":"open","labels":["weft-ref:a"]}]`, Code: 0}
		}
		if strings.Contains(j, "bd import") {
			return run.Result{Code: 1, Stderr: "import boom"}
		}
		return run.Result{Code: 0}
	}}
	if got := exit.Code(runRoot(r, "plan", "emit", file, "--epic", "e")); got != 2 {
		t.Fatalf("bd import failure must be a hard error (exit 2), got %d", got)
	}
}

func TestPlanEmitReplanListNonZeroExitIsHard(t *testing.T) {
	file := writePlanFile(t, `{"epic":{"title":"E"},"picks":[{"ref":"a","title":"A","description":"a"}]}`)
	r := &routeRunner{fn: func(name string, args []string) run.Result {
		if strings.Contains(strings.Join(append([]string{name}, args...), " "), "bd list") {
			return run.Result{Code: 1, Stderr: "list boom"}
		}
		return run.Result{Code: 0}
	}}
	if got := exit.Code(runRoot(r, "plan", "emit", file, "--epic", "e")); got != 2 {
		t.Fatalf("bd list failure must be a hard error (exit 2), got %d", got)
	}
}

func TestPlanEmitReplanMalformedListJSONIsHard(t *testing.T) {
	file := writePlanFile(t, `{"epic":{"title":"E"},"picks":[{"ref":"a","title":"A","description":"a"}]}`)
	r := &routeRunner{fn: func(name string, args []string) run.Result {
		if strings.Contains(strings.Join(append([]string{name}, args...), " "), "bd list") {
			return run.Result{Stdout: "not json at all", Code: 0}
		}
		return run.Result{Code: 0}
	}}
	if got := exit.Code(runRoot(r, "plan", "emit", file, "--epic", "e")); got != 2 {
		t.Fatalf("malformed bd list JSON must be a hard error (exit 2), got %d", got)
	}
}

func TestPlanEmitReplanEmptyListCreatesAll(t *testing.T) {
	// An epic with no existing children: every pick is a create, import runs cleanly.
	file := writePlanFile(t, `{"epic":{"title":"E"},"picks":[{"ref":"a","title":"A","description":"a"}]}`)
	r := &routeRunner{fn: func(name string, args []string) run.Result {
		if strings.Contains(strings.Join(append([]string{name}, args...), " "), "bd list") {
			return run.Result{Stdout: `[]`, Code: 0}
		}
		return run.Result{Code: 0}
	}}
	out, err := newTestCmd(r, "plan", "emit", file, "--epic", "e", "--json")
	if err != nil {
		t.Fatalf("empty warp should upsert-create cleanly: %v", err)
	}
	if !strings.Contains(out.String(), `"created"`) {
		t.Errorf("expected created list in output: %q", out.String())
	}
}

func TestPlanEmitRejectsDashEpic(t *testing.T) {
	file := writePlanFile(t, `{"epic":{"title":"E"},"picks":[{"ref":"a","title":"A","description":"a"}]}`)
	r := &routeRunner{fn: func(string, []string) run.Result { return run.Result{} }}
	if got := exit.Code(runRoot(r, "plan", "emit", file, "--epic=-x")); got != 1 {
		t.Fatalf("flag-like --epic value must be an invocation error (exit 1), got %d", got)
	}
}
