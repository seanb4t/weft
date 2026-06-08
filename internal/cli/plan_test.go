// internal/cli/plan_test.go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Weft Contributors

package cli

import (
	"fmt"
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
	r := &routeRunner{fn: func(_ string, _ []string) run.Result { return run.Result{} }}
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
	r := &routeRunner{fn: func(_ string, _ []string) run.Result { return run.Result{} }}
	out, err := newTestCmd(r, "plan", "check", file, "--json")
	if err != nil {
		t.Fatalf("check must exit 0 even when invalid: %v", err)
	}
	if !strings.Contains(out.String(), `"valid": false`) {
		t.Errorf("expected valid:false, got %q", out.String())
	}
}

// dryRunOK scripts bd's dry-run JSON envelope with the given node/edge counts
// and a clean stderr (no drops, no notes), so the preflight passes cleanly.
func dryRunOK(nodes, edges int) run.Result {
	return run.Result{
		Stdout: fmt.Sprintf(`{"dry_run":true,"node_count":%d,"edge_count":%d,"schema_version":1}`, nodes, edges),
		Code:   0,
	}
}

// isMutatingCreate reports whether a recorded call is the real (non-dry-run)
// bd create --graph.
func isMutatingCreate(call []string) bool {
	j := strings.Join(call, " ")
	return strings.Contains(j, "create --graph") && !strings.Contains(j, "--dry-run")
}

func TestPlanEmitDryRunRunsPreflightNoMutation(t *testing.T) {
	// 2 picks share x.go,y.go > overlap_max(1) => 1 edge; nodes = epic+2 = 3.
	file := writePlanFile(t, `{"epic":{"title":"E"},"picks":[{"ref":"a","title":"A","description":"a","files":["x.go","y.go"]},{"ref":"b","title":"B","description":"b","files":["x.go","y.go"]}]}`)
	r := &routeRunner{fn: func(_ string, args []string) run.Result {
		if strings.Contains(strings.Join(args, " "), "--dry-run") {
			return dryRunOK(3, 1)
		}
		return run.Result{}
	}}
	out, err := newTestCmd(r, "plan", "emit", file, "--dry-run", "--json")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(out.String(), `"dry_run": true`) {
		t.Errorf("expected dry_run:true: %q", out.String())
	}
	sawDryRun := false
	for _, c := range r.calls {
		if isMutatingCreate(c) {
			t.Fatalf("dry-run must not mutate: %v", r.calls)
		}
		if strings.Contains(strings.Join(c, " "), "--dry-run") {
			sawDryRun = true
		}
	}
	if !sawDryRun {
		t.Errorf("dry-run must run the bd preflight; calls=%v", r.calls)
	}
}

func TestPlanEmitFirstCreatesGraph(t *testing.T) {
	file := writePlanFile(t, `{"epic":{"title":"E"},"picks":[{"ref":"a","title":"A","description":"a"}]}`)
	// 1 pick => nodes=epic+1=2, edges=0.
	r := &routeRunner{fn: func(_ string, args []string) run.Result {
		if strings.Contains(strings.Join(args, " "), "--dry-run") {
			return dryRunOK(2, 0)
		}
		return run.Result{Stdout: "created weft-zzz", Code: 0}
	}}
	out, err := newTestCmd(r, "plan", "emit", file, "--json")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(out.String(), `"mode": "create"`) {
		t.Errorf("expected mode:create, got %q", out.String())
	}
	saw := false
	for _, c := range r.calls {
		if isMutatingCreate(c) {
			saw = true
		}
	}
	if !saw {
		t.Errorf("first emit must run the real bd create --graph: %v", r.calls)
	}
}

func TestPlanEmitPreflightExecErrorIsHard(t *testing.T) {
	file := writePlanFile(t, `{"epic":{"title":"E"},"picks":[{"ref":"a","title":"A","description":"a"}]}`)
	r := &routeRunner{errFn: func(_ string, _ []string) error { return fmt.Errorf("exec: bd not found") }}
	if got := exit.Code(runRoot(r, "plan", "emit", file)); got != 2 {
		t.Fatalf("preflight exec failure must be a hard error (exit 2), got %d", got)
	}
	for _, c := range r.calls {
		if isMutatingCreate(c) {
			t.Fatalf("must not mutate when preflight cannot run: %v", r.calls)
		}
	}
}

func TestPlanEmitPreflightNonZeroExitIsHard(t *testing.T) {
	file := writePlanFile(t, `{"epic":{"title":"E"},"picks":[{"ref":"a","title":"A","description":"a"}]}`)
	r := &routeRunner{fn: func(_ string, args []string) run.Result {
		if strings.Contains(strings.Join(args, " "), "--dry-run") {
			return run.Result{Code: 1, Stderr: "preflight boom"}
		}
		return run.Result{Code: 0}
	}}
	if got := exit.Code(runRoot(r, "plan", "emit", file)); got != 2 {
		t.Fatalf("preflight non-zero exit must be a hard error (exit 2), got %d", got)
	}
	for _, c := range r.calls {
		if isMutatingCreate(c) {
			t.Fatalf("must not mutate after preflight failure: %v", r.calls)
		}
	}
}

func TestPlanEmitPreflightBadJSONIsHard(t *testing.T) {
	file := writePlanFile(t, `{"epic":{"title":"E"},"picks":[{"ref":"a","title":"A","description":"a"}]}`)
	r := &routeRunner{fn: func(_ string, args []string) run.Result {
		if strings.Contains(strings.Join(args, " "), "--dry-run") {
			return run.Result{Stdout: "not json", Code: 0}
		}
		return run.Result{Code: 0}
	}}
	if got := exit.Code(runRoot(r, "plan", "emit", file)); got != 2 {
		t.Fatalf("preflight bad JSON must be a hard error (exit 2), got %d", got)
	}
	for _, c := range r.calls {
		if isMutatingCreate(c) {
			t.Fatalf("must not mutate after preflight parse failure: %v", r.calls)
		}
	}
}

func TestPlanEmitRefusesInvalidPlan(t *testing.T) {
	file := writePlanFile(t, `{"epic":{},"picks":[]}`)
	r := &routeRunner{fn: func(_ string, _ []string) run.Result { return run.Result{} }}
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
	r := &routeRunner{fn: func(_ string, args []string) run.Result {
		j := strings.Join(args, " ")
		if strings.Contains(j, "--dry-run") {
			return dryRunOK(2, 0) // preflight passes so the real create is reached
		}
		if strings.Contains(j, "create --graph") {
			return run.Result{Code: 1, Stderr: "create boom"}
		}
		return run.Result{Code: 0}
	}}
	if got := exit.Code(runRoot(r, "plan", "emit", file)); got != 2 {
		t.Fatalf("real bd create --graph failure must be a hard error (exit 2), got %d", got)
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
	r := &routeRunner{fn: func(_ string, _ []string) run.Result { return run.Result{} }}
	if got := exit.Code(runRoot(r, "plan", "emit", file, "--epic=-x")); got != 1 {
		t.Fatalf("flag-like --epic value must be an invocation error (exit 1), got %d", got)
	}
}

// dropPlanFile is a single-pick plan: dry-run expects nodes=2, edges=0.
func dropPlanFile(t *testing.T) string {
	return writePlanFile(t, `{"epic":{"title":"E"},"picks":[{"ref":"a","title":"A","description":"a"}]}`)
}

func dryRunWithDrop() run.Result {
	return run.Result{
		Stdout: `{"dry_run":true,"node_count":2,"edge_count":0,"schema_version":1}`,
		Stderr: `warning: graph plan node["@epic"] has unknown field(s): [acceptance] (silently dropped — see 'bd create --graph' schema)`,
		Code:   0,
	}
}

func TestPlanEmitDropWithoutAllowFailsBeforeMutation(t *testing.T) {
	file := dropPlanFile(t)
	r := &routeRunner{fn: func(_ string, args []string) run.Result {
		if strings.Contains(strings.Join(args, " "), "--dry-run") {
			return dryRunWithDrop()
		}
		return run.Result{Code: 0}
	}}
	_, err := newTestCmd(r, "plan", "emit", file, "--json")
	if exit.Code(err) != 2 {
		t.Fatalf("a drop must hard-fail (exit 2), got %v", err)
	}
	for _, c := range r.calls {
		if isMutatingCreate(c) {
			t.Fatalf("must not mutate after a drop: %v", r.calls)
		}
	}
}

func TestPlanEmitDropWithAllowSurfacesWarningAndProceeds(t *testing.T) {
	file := dropPlanFile(t)
	r := &routeRunner{fn: func(_ string, args []string) run.Result {
		if strings.Contains(strings.Join(args, " "), "--dry-run") {
			return dryRunWithDrop()
		}
		return run.Result{Stdout: "created", Code: 0}
	}}
	out, err := newTestCmd(r, "plan", "emit", file, "--allow-drop", "--json")
	if err != nil {
		t.Fatalf("--allow-drop must proceed: %v", err)
	}
	if !strings.Contains(out.String(), "unknown field(s)") {
		t.Errorf("drop must be surfaced in warnings: %q", out.String())
	}
	saw := false
	for _, c := range r.calls {
		if isMutatingCreate(c) {
			saw = true
		}
	}
	if !saw {
		t.Errorf("--allow-drop must run the real create: %v", r.calls)
	}
}

func TestPlanEmitCountMismatchHardEvenWithAllowDrop(t *testing.T) {
	file := dropPlanFile(t) // weft builds nodes=2, edges=0
	r := &routeRunner{fn: func(_ string, args []string) run.Result {
		if strings.Contains(strings.Join(args, " "), "--dry-run") {
			return run.Result{Stdout: `{"node_count":5,"edge_count":9,"schema_version":1}`, Code: 0}
		}
		return run.Result{Code: 0}
	}}
	_, err := newTestCmd(r, "plan", "emit", file, "--allow-drop", "--json")
	if exit.Code(err) != 2 {
		t.Fatalf("count mismatch must hard-fail even with --allow-drop, got %v", err)
	}
}

func TestPlanEmitSchemaMismatchIsSoft(t *testing.T) {
	file := dropPlanFile(t)
	r := &routeRunner{fn: func(_ string, args []string) run.Result {
		if strings.Contains(strings.Join(args, " "), "--dry-run") {
			return run.Result{Stdout: `{"node_count":2,"edge_count":0,"schema_version":99}`, Code: 0}
		}
		return run.Result{Stdout: "created", Code: 0}
	}}
	out, err := newTestCmd(r, "plan", "emit", file, "--json")
	if err != nil {
		t.Fatalf("schema mismatch must be soft (no error): %v", err)
	}
	if !strings.Contains(out.String(), "schema_version") {
		t.Errorf("schema note must be surfaced: %q", out.String())
	}
}

func TestPlanEmitDryRunDropWithAllowSurfacesWarning(t *testing.T) {
	file := dropPlanFile(t)
	r := &routeRunner{fn: func(_ string, args []string) run.Result {
		if strings.Contains(strings.Join(args, " "), "--dry-run") {
			return dryRunWithDrop()
		}
		return run.Result{Code: 0}
	}}
	out, err := newTestCmd(r, "plan", "emit", file, "--dry-run", "--allow-drop", "--json")
	if err != nil {
		t.Fatalf("--dry-run --allow-drop must succeed: %v", err)
	}
	s := out.String()
	if !strings.Contains(s, "unknown field(s)") {
		t.Errorf("drop warning must appear in dry-run output: %q", s)
	}
	if !strings.Contains(s, `"warnings"`) {
		t.Errorf("warnings key must be present in dry-run envelope: %q", s)
	}
	for _, c := range r.calls {
		if isMutatingCreate(c) {
			t.Fatalf("dry-run must not mutate even with --allow-drop: %v", r.calls)
		}
	}
}

func TestPlanEmitPreflightNoteAppearsInWarnings(t *testing.T) {
	// A non-drop stderr line (e.g. a deprecation notice) must be surfaced as a
	// soft warning; it must not block the emit.
	file := dropPlanFile(t)
	r := &routeRunner{fn: func(_ string, args []string) run.Result {
		if strings.Contains(strings.Join(args, " "), "--dry-run") {
			return run.Result{
				Stdout: `{"dry_run":true,"node_count":2,"edge_count":0,"schema_version":1}`,
				Stderr: "warning: bd deprecation notice",
				Code:   0,
			}
		}
		return run.Result{Stdout: "created", Code: 0}
	}}
	out, err := newTestCmd(r, "plan", "emit", file, "--json")
	if err != nil {
		t.Fatalf("non-drop preflight note must not block emit: %v", err)
	}
	if !strings.Contains(out.String(), "bd deprecation notice") {
		t.Errorf("preflight note must appear in warnings: %q", out.String())
	}
}

func TestPlanEmitAllowDropWithEpicIsInvocationError(t *testing.T) {
	file := writePlanFile(t, `{"epic":{"title":"E"},"picks":[{"ref":"a","title":"A","description":"a"}]}`)
	r := &routeRunner{fn: func(_ string, _ []string) run.Result { return run.Result{} }}
	err := runRoot(r, "plan", "emit", file, "--epic", "e", "--allow-drop")
	if got := exit.Code(err); got != 1 {
		t.Fatalf("--allow-drop with --epic must be an invocation error (exit 1), got %d (err=%v)", got, err)
	}
	// Guard must fire before any runner call — no bd list/import must be attempted.
	if len(r.calls) != 0 {
		t.Fatalf("guard must short-circuit before any runner call; got calls: %v", r.calls)
	}
}

func TestPlanReplanSurfacesImportStderr(t *testing.T) {
	file := writePlanFile(t, `{"epic":{"title":"E"},"picks":[{"ref":"a","title":"A","description":"a"}]}`)
	r := &routeRunner{fn: func(_ string, args []string) run.Result {
		j := strings.Join(args, " ")
		if strings.HasPrefix(j, "list --parent") {
			return run.Result{Stdout: "[]", Code: 0} // no existing children
		}
		if strings.HasPrefix(j, "import") {
			return run.Result{Stdout: "imported 1", Stderr: "warning: something bd noticed", Code: 0}
		}
		return run.Result{Code: 0}
	}}
	out, err := newTestCmd(r, "plan", "emit", file, "--epic", "weft-abc", "--json")
	if err != nil {
		t.Fatalf("replan: %v", err)
	}
	if !strings.Contains(out.String(), "something bd noticed") {
		t.Errorf("bd import stderr must be surfaced in warnings: %q", out.String())
	}
	if !strings.Contains(out.String(), `"warnings"`) {
		t.Errorf("warnings key must be present in envelope: %q", out.String())
	}
}
