<!--
  ~ SPDX-License-Identifier: Apache-2.0
  ~ Copyright 2026 Weft Contributors
-->

# Plan Emit Field-Drop Guard (Seam 9) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use dev-flow:subagent-driven-development (recommended) or dev-flow:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make `weft plan emit` refuse to silently lose warp data by surfacing bd's existing unknown-field drop warnings and graph-shape counts, failing *before* mutation.

**Architecture:** Two pure helpers in `internal/plan` parse and categorize a `bd create --graph --dry-run --json` result (drop warnings from stderr; `node_count`/`edge_count`/`schema_version` from stdout). `internal/cli/plan.go` runs that dry-run as a **preflight** before the real create: a drop or count mismatch hard-fails (exit 2); `--allow-drop` downgrades a drop to a surfaced warning; a `schema_version` mismatch is a soft warning. `--dry-run` becomes bd-backed (runs the preflight, reports, no mutation). The replan/`bd import` path stops discarding stderr.

**Tech Stack:** Go 1.26, `bd` CLI (`bd 1.0.5`, graph schema_version 1), the engine's `run.Runner`/`run.BD` wrapper and `exit`/`Emit` envelope helpers.

**Spec:** `docs/seams/09-emit-field-drop-guard.md`. **Design bead:** `weft-hjx.15`.

---

## File Structure

| Path | Action | Responsibility |
| --- | --- | --- |
| `internal/plan/preflight.go` | create | `Preflight` type, `ParsePreflight` (parse bd dry-run stdout/stderr), `ExpectedGraphSchemaVersion`, `CheckPreflight` (categorize drops/count/schema) |
| `internal/plan/preflight_test.go` | create | pure unit tests for `ParsePreflight` + `CheckPreflight` |
| `internal/cli/plan.go` | modify | `planFirstEmit` runs the bd-backed preflight + `--allow-drop` flag; `--dry-run` bd-backed; real create stops discarding stderr; `planReplan` surfaces `bd import` stderr |
| `internal/cli/plan_test.go` | modify | update 2 existing emit tests for the bd-backed dry-run; add preflight-gate tests |
| `internal/plan/emit_test.go` | modify | regression test: `GraphJSON` emits only bd-known node/edge field keys |
| `internal/plan/integration_test.go` | create | build-tagged (`//go:build integration`) live-bd check: real `GraphJSON` → zero drop warnings + matching counts |

bd's grounded dry-run contract (from the spec §3, verified live against `bd 1.0.5`):

```text
# stdout (JSON)
{ "dry_run": true, "node_count": 2, "edge_count": 1, "parent_deps": 1, "schema_version": 1, "nodes": […], "validation_notes": […] }
# stderr (one line per dropped field; command still exits 0)
warning: graph plan node["@epic"] has unknown field(s): [acceptance] (silently dropped — see 'bd create --graph' schema)
```

---

## Task 1: `Preflight` type + `ParsePreflight` (pure)

**Files:**

- Create: `internal/plan/preflight.go`
- Test: `internal/plan/preflight_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/plan/preflight_test.go`:

```go
// internal/plan/preflight_test.go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Weft Contributors

package plan

import "testing"

const sampleDryRunStdout = `{
  "dry_run": true, "node_count": 2, "edge_count": 1,
  "parent_deps": 1, "schema_version": 1,
  "nodes": [{"key":"@epic"}], "validation_notes": []
}`

const sampleDryRunStderr = `warning: graph plan node["@epic"] has unknown field(s): [acceptance] (silently dropped — see 'bd create --graph' schema)
warning: graph plan edge[0] has unknown field(s): [bogus] (silently dropped — see 'bd create --graph' schema)`

func TestParsePreflightCountsAndDrops(t *testing.T) {
	pf, err := ParsePreflight([]byte(sampleDryRunStdout), []byte(sampleDryRunStderr))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if pf.NodeCount != 2 || pf.EdgeCount != 1 || pf.SchemaVersion != 1 {
		t.Errorf("counts = nodes %d edges %d schema %d; want 2/1/1", pf.NodeCount, pf.EdgeCount, pf.SchemaVersion)
	}
	if len(pf.Drops) != 2 {
		t.Fatalf("Drops = %v; want 2 lines", pf.Drops)
	}
}

func TestParsePreflightCleanHasEmptyDrops(t *testing.T) {
	pf, err := ParsePreflight([]byte(sampleDryRunStdout), []byte(""))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	// Must be a non-nil empty slice (JSON null breaks --json consumers).
	if pf.Drops == nil || len(pf.Drops) != 0 {
		t.Errorf("clean stderr must yield empty (non-nil) Drops, got %#v", pf.Drops)
	}
}

func TestParsePreflightBadStdoutErrors(t *testing.T) {
	if _, err := ParsePreflight([]byte("not json"), []byte("")); err == nil {
		t.Error("unparseable stdout must return an error")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/plan/ -run TestParsePreflight -v`
Expected: FAIL — `undefined: ParsePreflight` / `Preflight`.

- [ ] **Step 3: Write minimal implementation**

Create `internal/plan/preflight.go`:

```go
// internal/plan/preflight.go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Weft Contributors

package plan

import (
	"encoding/json"
	"fmt"
	"strings"
)

// dropMarker is bd's stable phrase for a silently-dropped unknown field in a
// graph plan (grounded against bd 1.0.5). Classify on this marker, not loose
// English — mirrors the gh-api error-classification convention.
const dropMarker = "unknown field(s)"

// Preflight is the parsed result of `bd create --graph --dry-run --json`.
type Preflight struct {
	NodeCount     int
	EdgeCount     int
	SchemaVersion int
	Drops         []string // verbatim bd warning lines naming dropped fields
}

// dryRunEnvelope is the subset of bd's dry-run JSON weft reads.
type dryRunEnvelope struct {
	NodeCount     int `json:"node_count"`
	EdgeCount     int `json:"edge_count"`
	SchemaVersion int `json:"schema_version"`
}

// ParsePreflight parses a bd graph dry-run: counts + schema_version from the
// JSON stdout, dropped-field warnings from stderr (one verbatim line each).
func ParsePreflight(stdout, stderr []byte) (Preflight, error) {
	var env dryRunEnvelope
	if err := json.Unmarshal(stdout, &env); err != nil {
		return Preflight{}, fmt.Errorf("parse bd graph dry-run json: %w", err)
	}
	drops := []string{}
	for _, line := range strings.Split(string(stderr), "\n") {
		line = strings.TrimSpace(line)
		if line != "" && strings.Contains(line, dropMarker) {
			drops = append(drops, line)
		}
	}
	return Preflight{
		NodeCount:     env.NodeCount,
		EdgeCount:     env.EdgeCount,
		SchemaVersion: env.SchemaVersion,
		Drops:         drops,
	}, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/plan/ -run TestParsePreflight -v`
Expected: PASS (3 subtests).

- [ ] **Step 5: Commit**

Commit using VCS-appropriate commands per `references/vcs-preamble.md` (jj): `jj commit -m "feat(weft-hjx.15): add ParsePreflight for bd graph dry-run output"`.

---

## Task 2: `ExpectedGraphSchemaVersion` + `CheckPreflight` (pure)

**Files:**

- Modify: `internal/plan/preflight.go`
- Test: `internal/plan/preflight_test.go`

- [ ] **Step 1: Write the failing test**

Append to `internal/plan/preflight_test.go`:

```go
func TestCheckPreflightCleanIsAllZero(t *testing.T) {
	pf := Preflight{NodeCount: 3, EdgeCount: 2, SchemaVersion: ExpectedGraphSchemaVersion, Drops: []string{}}
	got := CheckPreflight(pf, 3, 2)
	if len(got.Drops) != 0 || got.CountMismatch != "" || got.SchemaNote != "" {
		t.Errorf("clean preflight must yield no issues, got %#v", got)
	}
}

func TestCheckPreflightCountMismatch(t *testing.T) {
	pf := Preflight{NodeCount: 2, EdgeCount: 2, SchemaVersion: ExpectedGraphSchemaVersion, Drops: []string{}}
	got := CheckPreflight(pf, 3, 2) // want 3 nodes, bd saw 2
	if got.CountMismatch == "" {
		t.Error("node-count mismatch must be reported")
	}
}

func TestCheckPreflightSchemaMismatchIsSoft(t *testing.T) {
	pf := Preflight{NodeCount: 3, EdgeCount: 2, SchemaVersion: ExpectedGraphSchemaVersion + 99, Drops: []string{}}
	got := CheckPreflight(pf, 3, 2)
	if got.SchemaNote == "" {
		t.Error("schema_version mismatch must produce a soft note")
	}
	if got.CountMismatch != "" {
		t.Error("schema mismatch must NOT be a count error")
	}
}

func TestCheckPreflightDropsPassThrough(t *testing.T) {
	pf := Preflight{NodeCount: 3, EdgeCount: 2, SchemaVersion: ExpectedGraphSchemaVersion, Drops: []string{"warning: … unknown field(s): [x]"}}
	got := CheckPreflight(pf, 3, 2)
	if len(got.Drops) != 1 {
		t.Errorf("drops must pass through for the caller's --allow-drop policy, got %#v", got.Drops)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/plan/ -run TestCheckPreflight -v`
Expected: FAIL — `undefined: ExpectedGraphSchemaVersion` / `CheckPreflight`.

- [ ] **Step 3: Write minimal implementation**

Append to `internal/plan/preflight.go`:

```go
// ExpectedGraphSchemaVersion is the bd graph schema version this build was
// grounded against. A mismatch is a soft signal to re-ground weft, not a stop.
const ExpectedGraphSchemaVersion = 1

// PreflightIssues categorizes a preflight against weft's expectations. Drops and
// CountMismatch are hard (CountMismatch always; Drops unless the caller passes
// --allow-drop); SchemaNote is always soft.
type PreflightIssues struct {
	Drops         []string // unknown-field drop warnings (verbatim)
	CountMismatch string   // "" when node/edge counts match; else a description
	SchemaNote    string   // "" when schema_version matches; else a soft note
}

// CheckPreflight compares a parsed preflight to the node/edge counts weft built.
func CheckPreflight(pf Preflight, wantNodes, wantEdges int) PreflightIssues {
	issues := PreflightIssues{Drops: pf.Drops}
	if pf.NodeCount != wantNodes || pf.EdgeCount != wantEdges {
		issues.CountMismatch = fmt.Sprintf(
			"bd parsed %d node(s)/%d edge(s); weft built %d/%d (graph shape drift)",
			pf.NodeCount, pf.EdgeCount, wantNodes, wantEdges)
	}
	if pf.SchemaVersion != ExpectedGraphSchemaVersion {
		issues.SchemaNote = fmt.Sprintf(
			"bd graph schema_version %d != expected %d — re-ground weft (proceeding)",
			pf.SchemaVersion, ExpectedGraphSchemaVersion)
	}
	return issues
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/plan/ -run 'TestCheckPreflight|TestParsePreflight' -v`
Expected: PASS (all subtests).

- [ ] **Step 5: Commit**

`jj commit -m "feat(weft-hjx.15): add CheckPreflight + ExpectedGraphSchemaVersion"`.

---

## Task 3: Wire the preflight into `planFirstEmit` (+ `--allow-drop`, bd-backed dry-run)

**Files:**

- Modify: `internal/cli/plan.go` (`newPlanEmitCmd`, `planFirstEmit`)
- Modify: `internal/cli/plan_test.go` (update 2 existing tests; add gate tests)

This is the behavior change. `planFirstEmit` gains an `allowDrop` parameter; it always stages the payload, runs `bd create --graph <path> --dry-run --json` as a preflight, parses+checks it, applies the strictness policy, and only then (non-dry-run) runs the real create — now capturing stderr on success too.

- [ ] **Step 1: Update the two existing tests that assumed dry-run never calls bd**

In `internal/cli/plan_test.go`, the dry-run is now bd-backed. Replace `TestPlanEmitDryRunNoMutation` and `TestPlanEmitFirstCreatesGraph` with versions whose `routeRunner` scripts the dry-run JSON, and whose "no mutation" assertion targets the *mutating* create (no `--dry-run`) only:

```go
// dryRunOK returns a scripted bd dry-run result with matching counts and no drops.
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
}
```

Also update `TestPlanEmitCreateGraphNonZeroExitIsHard` (currently matches **all**
`bd create --graph` calls, so the preflight would now intercept it and the test
would silently assert the *preflight*-failure path instead of the real-create
path). Route the dry-run to success and fail only the real create, so it keeps
testing what its name claims:

```go
func TestPlanEmitCreateGraphNonZeroExitIsHard(t *testing.T) {
	file := writePlanFile(t, `{"epic":{"title":"E"},"picks":[{"ref":"a","title":"A","description":"a"}]}`)
	r := &routeRunner{fn: func(_ string, args []string) run.Result {
		j := strings.Join(args, " ")
		if strings.Contains(j, "--dry-run") {
			return dryRunOK(2, 0) // preflight passes (nodes=2, edges=0)…
		}
		if strings.Contains(j, "create --graph") {
			return run.Result{Code: 1, Stderr: "create boom"} // …real create fails
		}
		return run.Result{Code: 0}
	}}
	if got := exit.Code(runRoot(r, "plan", "emit", file)); got != 2 {
		t.Fatalf("real bd create --graph failure must be a hard error (exit 2), got %d", got)
	}
}
```

(Add `"fmt"` to the `plan_test.go` import block if not already present.)

- [ ] **Step 2: Add the gate tests (these fail until Step 4)**

Append to `internal/cli/plan_test.go`:

```go
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
```

- [ ] **Step 3: Run the gate tests to verify they fail**

Run: `go test ./internal/cli/ -run 'TestPlanEmit' -v`
Expected: FAIL — `--allow-drop` flag unknown / preflight not wired (drops not detected, exit not 2).

- [ ] **Step 4: Implement the wiring**

In `internal/cli/plan.go`, add the flag in `newPlanEmitCmd`:

```go
	var allowDrop bool
	// … inside the cobra.Command, the RunE closure's final line becomes:
			return a.planFirstEmit(cmd, wp, d, dryRun, allowDrop)
	// … after the existing c.Flags() calls:
	c.Flags().BoolVar(&allowDrop, "allow-drop", false, "proceed despite bd dropping unknown graph fields (loud, opt-in)")
```

Replace `planFirstEmit` with the preflight-gated version:

```go
// planFirstEmit creates a brand-new warp via bd create --graph (spec §6),
// gated by a bd-backed dry-run preflight that refuses to silently drop fields
// (seam 9 / docs/seams/09-emit-field-drop-guard.md).
func (a *App) planFirstEmit(cmd *cobra.Command, wp plan.WarpPlan, d plan.Derivation, dryRun, allowDrop bool) error {
	graph, err := plan.GraphJSON(wp, d)
	if err != nil {
		return err
	}
	path, cleanup, err := writeTempPayload("weft-warp-*.json", graph)
	if err != nil {
		return err
	}
	defer cleanup()

	// Preflight: bd's own dry-run reports dropped fields (stderr) + the parsed
	// graph shape (stdout). It mutates nothing, so we can abort before any create.
	pre, err := run.BD(a.Runner, "create", "--graph", path, "--dry-run", "--json")
	if err != nil {
		return exit.Hardf("bd create --graph dry-run could not run: %v", err)
	}
	if pre.Code != 0 {
		return exit.Hardf("bd create --graph dry-run failed: %s", strings.TrimSpace(pre.Stderr))
	}
	pf, err := plan.ParsePreflight([]byte(pre.Stdout), []byte(pre.Stderr))
	if err != nil {
		return exit.Hardf("%v", err)
	}
	issues := plan.CheckPreflight(pf, 1+len(wp.Picks), len(d.Edges))

	warnings := []string{}
	if issues.CountMismatch != "" {
		return exit.Hardf("plan emit aborted: %s", issues.CountMismatch)
	}
	if len(issues.Drops) > 0 {
		if !allowDrop {
			return exit.Hardf("plan emit aborted — bd would drop fields (data loss); fix the payload or pass --allow-drop:\n%s",
				strings.Join(issues.Drops, "\n"))
		}
		warnings = append(warnings, issues.Drops...)
	}
	if issues.SchemaNote != "" {
		warnings = append(warnings, issues.SchemaNote)
	}

	if dryRun {
		data := map[string]any{
			"dry_run": true, "mode": "create", "epic": wp.Epic.Title,
			"picks": len(wp.Picks), "edges": d.Edges, "tolerated": d.Tolerated,
			"schema_version": pf.SchemaVersion, "warnings": warnings,
		}
		return Emit(cmd, "plan.emit", data, planPreviewText("create", wp, d))
	}

	res, err := run.BD(a.Runner, "create", "--graph", path)
	if err != nil {
		return exit.Hardf("bd create --graph could not run: %v", err)
	}
	if res.Code != 0 {
		return exit.Hardf("bd create --graph failed: %s", strings.TrimSpace(res.Stderr))
	}
	// Belt-and-suspenders: surface any warning the real create emits on success.
	if s := strings.TrimSpace(res.Stderr); s != "" {
		warnings = append(warnings, s)
	}
	data := map[string]any{
		"mode": "create", "created": len(wp.Picks), "edges": d.Edges,
		"tolerated": d.Tolerated, "schema_version": pf.SchemaVersion,
		"warnings": warnings, "bd_output": strings.TrimSpace(res.Stdout),
	}
	text := fmt.Sprintf("emitted warp: %d pick(s), %d edge(s), %d tolerated overlap(s)\n%s",
		len(wp.Picks), len(d.Edges), len(d.Tolerated), strings.TrimSpace(res.Stdout))
	return Emit(cmd, "plan.emit", data, text)
}
```

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./internal/cli/ -run 'TestPlanEmit' -v`
Expected: PASS (updated + new tests).

- [ ] **Step 6: Commit**

`jj commit -m "feat(weft-hjx.15): preflight-gate plan emit against bd field-drop (--allow-drop, bd-backed dry-run)"`.

---

## Task 4: Surface `bd import` stderr on the replan path

**Files:**

- Modify: `internal/cli/plan.go` (`planReplan`)
- Modify: `internal/cli/plan_test.go`

bd's import dry-run can't warn per-field (spec §7), so the minimum is: stop discarding `bd import` stderr on success and fold it into a `warnings` field.

- [ ] **Step 1: Write the failing test**

Append to `internal/cli/plan_test.go`:

```go
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
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/cli/ -run TestPlanReplanSurfacesImportStderr -v`
Expected: FAIL — stderr is discarded; "something bd noticed" absent from output.

- [ ] **Step 3: Implement**

In `planReplan` (`internal/cli/plan.go`), after the `bd import` success check, fold stderr into the envelope. Locate the success `data := map[string]any{...}` block in `planReplan` and add a `warnings` key:

```go
	// after: res, err := run.BD(a.Runner, "import", path) and the Code!=0 guard
	warnings := []string{}
	if s := strings.TrimSpace(res.Stderr); s != "" {
		warnings = append(warnings, s)
	}
	data := map[string]any{
		"mode": "upsert", "epic": epic, "warnings": warnings,
		"bd_output": strings.TrimSpace(res.Stdout),
		// … keep the existing replan data fields (created/updated/superseded counts) …
	}
```

(Preserve every existing key in that `data` map; only add `warnings`. Read the current `planReplan` body first and merge, do not drop fields.)

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/cli/ -run TestPlanReplan -v`
Expected: PASS.

- [ ] **Step 5: Commit**

`jj commit -m "feat(weft-hjx.15): surface bd import stderr on the replan path"`.

---

## Task 5: Regression guard — `GraphJSON` emits only bd-known fields

**Files:**

- Modify: `internal/plan/emit_test.go`
- Create: `internal/plan/integration_test.go`

A pure test pins that `GraphJSON` never adds a field bd would drop (the always-runs sentinel); a build-tagged test exercises the real bd (the opt-in drift sentinel).

- [ ] **Step 1: Write the failing pure test**

Append to `internal/plan/emit_test.go`:

```go
func TestGraphJSONEmitsOnlyKnownFields(t *testing.T) {
	wp := WarpPlan{
		Epic:  Epic{Title: "E", Description: "d", Acceptance: "AC"},
		Picks: []Pick{{Ref: "a", Title: "A", Description: "a", Labels: []string{"x"}}},
	}
	b, err := GraphJSON(wp, Derive(wp.Picks, nil, 1))
	if err != nil {
		t.Fatalf("graph: %v", err)
	}
	var raw struct {
		Nodes []map[string]json.RawMessage `json:"nodes"`
		Edges []map[string]json.RawMessage `json:"edges"`
	}
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	nodeOK := map[string]bool{"key": true, "title": true, "description": true, "type": true, "parent_key": true, "labels": true, "priority": true}
	edgeOK := map[string]bool{"from_key": true, "to_key": true, "type": true}
	for _, n := range raw.Nodes {
		for k := range n {
			if !nodeOK[k] {
				t.Errorf("node carries unknown-to-bd field %q (would be silently dropped)", k)
			}
		}
	}
	for _, e := range raw.Edges {
		for k := range e {
			if !edgeOK[k] {
				t.Errorf("edge carries unknown-to-bd field %q", k)
			}
		}
	}
}
```

(Confirm `emit_test.go` imports `encoding/json`; add it if absent.)

- [ ] **Step 2: Run to verify it passes immediately (characterization guard)**

Run: `go test ./internal/plan/ -run TestGraphJSONEmitsOnlyKnownFields -v`
Expected: PASS — `GraphJSON` already emits only known fields (this locks that in; it would fail if a future change added a raw field like `acceptance`).

- [ ] **Step 3: Add the build-tagged live-bd integration test**

Create `internal/plan/integration_test.go`:

```go
// internal/plan/integration_test.go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Weft Contributors

//go:build integration

package plan_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/seanb4t/weft/internal/plan"
)

// TestGraphJSONNoDropAgainstLiveBD asserts a representative GraphJSON produces
// zero unknown-field warnings from the installed bd (the drift sentinel for a bd
// graph-schema change). Run with: go test -tags integration ./internal/plan/.
func TestGraphJSONNoDropAgainstLiveBD(t *testing.T) {
	wp := plan.WarpPlan{
		Epic:  plan.Epic{Title: "E", Description: "d", Acceptance: "AC"},
		Picks: []plan.Pick{{Ref: "a", Title: "A", Description: "a", Labels: []string{"phase:impl"}}},
	}
	b, err := plan.GraphJSON(wp, plan.Derive(wp.Picks, nil, 1))
	if err != nil {
		t.Fatalf("graph: %v", err)
	}
	dir := t.TempDir()
	f := filepath.Join(dir, "g.json")
	if err := os.WriteFile(f, b, 0o644); err != nil {
		t.Fatal(err)
	}
	out, _ := exec.Command("bd", "create", "--graph", f, "--dry-run", "--json").CombinedOutput()
	if strings.Contains(string(out), "unknown field(s)") {
		t.Fatalf("live bd dropped fields from GraphJSON output:\n%s", out)
	}
}
```

- [ ] **Step 4: Verify the integration test builds and (optionally) runs**

Run: `go vet -tags integration ./internal/plan/` then, if `bd` is available: `go test -tags integration ./internal/plan/ -run TestGraphJSONNoDropAgainstLiveBD -v`
Expected: builds clean; with bd present, PASS.

- [ ] **Step 5: Run the full suite + commit**

Run: `go build ./... && go test ./...`
Expected: all packages PASS.
Commit: `jj commit -m "test(weft-hjx.15): regression-guard GraphJSON known-field set (+ live-bd integration)"`.

---

## Done criteria

- `weft plan emit` runs a bd-backed dry-run preflight; a drop or count mismatch hard-fails (exit 2) before any mutation; `--allow-drop` downgrades a drop to a surfaced warning; `schema_version` mismatch is a soft warning.
- `weft plan emit --dry-run` runs the preflight (no mutation) and reports warnings + `schema_version`.
- The real create and the replan `bd import` no longer discard stderr on success.
- `go test ./...` green; `go test -tags integration ./internal/plan/` green with bd present.
- Spec §5 facts (`acceptance` dropped; label namespaces accepted) hold under the new regression guard.
<!-- adr-capture: sha256=0537af6d24e2579e; session=cli; ts=2026-06-07T00:25:48Z; adrs=weft-108,weft-axe,weft-2y4 -->
