<!--
  ~ SPDX-License-Identifier: Apache-2.0
  ~ Copyright 2026 Weft Contributors
-->

# Phased-Emission Engine Enablers Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement spec `docs/superpowers/specs/2026-06-10-phased-emission-engine-enablers-design.md` (bead `weft-ccy.5`): warp-plan `phases[]` roadmap emission, `ids` echo from `bd create --graph --json`, and post-import application of deferred re-plan edges.

**Architecture:** Pure transforms in `internal/plan` (model, validation, graph building) with thin CLI wiring in `internal/cli/plan.go`, exactly the existing seam-2 split. All bd interaction goes through the injectable `run.Runner`. TDD throughout; integration tests use the `internal/weave` scratch harness (`//go:build integration`).

**Tech Stack:** Go 1.26, cobra, `bd` CLI (graph/import/dep surfaces — all shapes empirically verified 2026-06-10, recorded as bd notes on `weft-ccy.5`).

**Verified ground truths the code relies on** (do not re-derive):

- `bd create --graph --json` (real run) prints `{"ids": {"<node-key>": "<bead-id>", ...}, "schema_version": 1}` — lowercase `ids`.
- `bd create --graph --dry-run --json` prints `node_count`/`edge_count`/`schema_version` (plus fields weft ignores) — `ParsePreflight` already handles it.
- bd accepts epic-type nodes with `parent_key` pointing at another epic node, and `blocks` edges between epic nodes.
- `bd import` cannot forward-reference ids within a batch; post-import `bd dep add <issue> <depends-on>` is the only way to wire edges touching new picks.
- `bd ready` excludes children of a blocked epic transitively; they release when the blocker closes.

---

## File Structure

```text
internal/plan/plan.go        # Phase model + conditionalized Validate (MODIFY)
internal/plan/plan_test.go   # validation matrix (MODIFY)
internal/plan/emit.go        # roadmap GraphJSON branch + RoadmapCounts (MODIFY)
internal/plan/emit_test.go   # roadmap golden + pick-path regression (MODIFY)
internal/plan/verify.go      # ReadbackBead gains ID (MODIFY)
internal/cli/plan.go         # preflight branch, envelopes, ids echo, applied edges (MODIFY)
internal/cli/plan_test.go    # CLI matrix (MODIFY)
internal/weave/fixture_seed_test.go      # migrate to ids (MODIFY)
internal/weave/roadmap_integration_test.go  # roadmap round-trip + gating pin (CREATE)
docs/seams/02-planning-emission.md       # §3/§5/§6/§7/§8 updates (MODIFY)
```

---

### Task 1: `Phase` model + conditionalized `Validate`

**Files:**

- Modify: `internal/plan/plan.go`
- Test: `internal/plan/plan_test.go`

- [ ] **Step 1: Write the failing tests** (append to `internal/plan/plan_test.go`)

```go
func phasesPlan(phases ...Phase) WarpPlan {
	return WarpPlan{Epic: Epic{Title: "E", Description: "d"}, Phases: phases}
}

func TestValidatePhasesAndPicksMutuallyExclusive(t *testing.T) {
	p := WarpPlan{
		Epic:   Epic{Title: "E"},
		Picks:  []Pick{{Ref: "a", Title: "A", Description: "a"}},
		Phases: []Phase{{Ref: "p1", Title: "P1", Description: "p"}},
	}
	issues := Validate(p)
	if len(issues) != 1 || !strings.Contains(issues[0].Message, "phases or picks, not both") {
		t.Fatalf("want single mutual-exclusion issue, got %v", issues)
	}
}

func TestValidateRoadmapHappyPath(t *testing.T) {
	p := phasesPlan(
		Phase{Ref: "p1", Title: "P1", Description: "first"},
		Phase{Ref: "p2", Title: "P2", Description: "second", Needs: []string{"p1"}},
	)
	if issues := Validate(p); len(issues) != 0 {
		t.Fatalf("valid roadmap rejected: %v", issues)
	}
}

func TestValidateRoadmapIssueMatrix(t *testing.T) {
	cases := []struct {
		name string
		p    WarpPlan
		want string // substring of the expected issue message
	}{
		{"missing ref", phasesPlan(Phase{Title: "P", Description: "d"}), "phase.ref is required"},
		{"reserved ref", phasesPlan(Phase{Ref: EpicKey, Title: "P", Description: "d"}), "reserved"},
		{"bad chars", phasesPlan(Phase{Ref: "p 1", Title: "P", Description: "d"}), "invalid characters"},
		{"dup ref", phasesPlan(
			Phase{Ref: "p1", Title: "A", Description: "d"},
			Phase{Ref: "p1", Title: "B", Description: "d"}), "duplicate phase.ref"},
		{"missing title", phasesPlan(Phase{Ref: "p1", Description: "d"}), "phase.title is required"},
		{"missing description", phasesPlan(Phase{Ref: "p1", Title: "P"}), "phase.description is required"},
		{"self need", phasesPlan(Phase{Ref: "p1", Title: "P", Description: "d", Needs: []string{"p1"}}), "references itself"},
		{"unknown need", phasesPlan(Phase{Ref: "p1", Title: "P", Description: "d", Needs: []string{"nope"}}), "unknown ref"},
		{"reserved need", phasesPlan(Phase{Ref: "p1", Title: "P", Description: "d", Needs: []string{EpicKey}}), "reserved"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			issues := Validate(tc.p)
			found := false
			for _, is := range issues {
				if strings.Contains(is.Message, tc.want) {
					found = true
				}
			}
			if !found {
				t.Fatalf("want issue containing %q, got %v", tc.want, issues)
			}
		})
	}
}

func TestValidateNoPhasesKeepsPickRules(t *testing.T) {
	// phases absent + picks absent => the existing "at least one pick" issue.
	issues := Validate(WarpPlan{Epic: Epic{Title: "E"}})
	found := false
	for _, is := range issues {
		if strings.Contains(is.Message, "at least one pick") {
			found = true
		}
	}
	if !found {
		t.Fatalf("pick-plan rules must be unchanged when phases absent: %v", issues)
	}
}
```

(`strings` is already imported in `plan_test.go`; if not, add it.)

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/plan/ -run 'TestValidatePhases|TestValidateRoadmap|TestValidateNoPhases' -v`
Expected: compile error — `Phase` / `Phases` undefined.

- [ ] **Step 3: Implement** (in `internal/plan/plan.go`)

Add after the `Pick` type:

```go
// Phase is one roadmap phase (phased-emission spec §1): emitted as a phase
// sub-epic under the project epic, carrying its weft-ref:<ref> identity label.
type Phase struct {
	Ref         string   `json:"ref"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Acceptance  string   `json:"acceptance,omitempty"`
	Needs       []string `json:"needs,omitempty"`
}
```

Add `Phases []Phase \`json:"phases,omitempty"\`` to `WarpPlan` (after `Picks`).

Restructure `Validate` — keep the epic-title check first, then branch:

```go
func Validate(p WarpPlan) []Issue {
	issues := []Issue{}
	if p.Epic.Title == "" {
		issues = append(issues, Issue{Message: "epic.title is required"})
	}
	if len(p.Phases) > 0 && len(p.Picks) > 0 {
		return append(issues, Issue{Message: "a plan carries phases or picks, not both"})
	}
	if len(p.Phases) > 0 {
		return append(issues, validatePhases(p.Phases)...)
	}
	if len(p.Picks) == 0 {
		issues = append(issues, Issue{Message: "at least one pick is required"})
	}
	// ... the two existing pick loops, byte-identical ...
	return issues
}

// validatePhases mirrors the pick rules for the roadmap shape (spec §1).
func validatePhases(phases []Phase) []Issue {
	issues := []Issue{}
	seen := map[string]bool{}
	for _, ph := range phases {
		switch {
		case ph.Ref == "":
			issues = append(issues, Issue{Message: "phase.ref is required"})
			continue
		case ph.Ref == EpicKey:
			issues = append(issues, Issue{Ref: ph.Ref, Message: fmt.Sprintf("phase.ref %q is reserved", EpicKey)})
			continue
		case seen[ph.Ref]:
			issues = append(issues, Issue{Ref: ph.Ref, Message: "duplicate phase.ref"})
		}
		seen[ph.Ref] = true
		if !refPattern.MatchString(ph.Ref) {
			issues = append(issues, Issue{Ref: ph.Ref, Message: fmt.Sprintf("phase.ref %q contains invalid characters (allowed: a-z A-Z 0-9 . _ -)", ph.Ref)})
		}
		if ph.Title == "" {
			issues = append(issues, Issue{Ref: ph.Ref, Message: "phase.title is required"})
		}
		if ph.Description == "" {
			issues = append(issues, Issue{Ref: ph.Ref, Message: "phase.description is required"})
		}
	}
	for _, ph := range phases {
		for _, n := range ph.Needs {
			switch {
			case n == ph.Ref:
				issues = append(issues, Issue{Ref: ph.Ref, Message: "phase.needs references itself"})
			case n == EpicKey:
				issues = append(issues, Issue{Ref: ph.Ref, Message: fmt.Sprintf("phase.needs references reserved ref %q", EpicKey)})
			case !seen[n]:
				issues = append(issues, Issue{Ref: ph.Ref, Message: fmt.Sprintf("phase.needs references unknown ref %q", n)})
			}
		}
	}
	return issues
}
```

Note the early `return` on mutual exclusion: one clear issue, no misleading
secondary noise from both branches.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/plan/ -v`
Expected: all PASS (new tests and the entire existing suite — the pick path must be untouched).

- [ ] **Step 5: Commit**

Run: `jj --no-pager commit -m "feat(weft-ccy.5): warp-plan Phase model + conditionalized validation"`

---

### Task 2: Roadmap `GraphJSON` + `RoadmapCounts`

**Files:**

- Modify: `internal/plan/emit.go`
- Test: `internal/plan/emit_test.go`

- [ ] **Step 1: Write the failing tests** (append to `internal/plan/emit_test.go`)

```go
func TestGraphJSONRoadmap(t *testing.T) {
	p := WarpPlan{
		Epic: Epic{Title: "Proj", Description: "proj desc", Acceptance: "proj AC"},
		Phases: []Phase{
			{Ref: "p2", Title: "Phase 2", Description: "second", Needs: []string{"p1"}},
			{Ref: "p1", Title: "Phase 1", Description: "first", Acceptance: "phase AC"},
		},
	}
	b, err := GraphJSON(p, Derivation{})
	if err != nil {
		t.Fatal(err)
	}
	var g struct {
		Nodes []struct {
			Key       string   `json:"key"`
			Type      string   `json:"type"`
			ParentKey string   `json:"parent_key"`
			Labels    []string `json:"labels"`
			Desc      string   `json:"description"`
		} `json:"nodes"`
		Edges []struct {
			FromKey string `json:"from_key"`
			ToKey   string `json:"to_key"`
			Type    string `json:"type"`
		} `json:"edges"`
	}
	if err := json.Unmarshal(b, &g); err != nil {
		t.Fatal(err)
	}
	if len(g.Nodes) != 3 {
		t.Fatalf("want 3 nodes (epic + 2 phases), got %d", len(g.Nodes))
	}
	// Nodes: epic first, then phases sorted by ref (p1 before p2).
	if g.Nodes[0].Key != EpicKey || g.Nodes[0].Type != "epic" {
		t.Fatalf("node 0 must be the project epic: %+v", g.Nodes[0])
	}
	p1 := g.Nodes[1]
	if p1.Key != "p1" || p1.Type != "epic" || p1.ParentKey != EpicKey {
		t.Fatalf("phase node shape wrong: %+v", p1)
	}
	if len(p1.Labels) != 1 || p1.Labels[0] != "weft-ref:p1" {
		t.Fatalf("phase node must carry exactly the weft-ref label: %v", p1.Labels)
	}
	if !strings.Contains(p1.Desc, "## Acceptance\nphase AC") {
		t.Fatalf("phase acceptance must fold into description: %q", p1.Desc)
	}
	if len(g.Edges) != 1 || g.Edges[0].FromKey != "p2" || g.Edges[0].ToKey != "p1" || g.Edges[0].Type != "blocks" {
		t.Fatalf("want single p2->p1 blocks edge, got %v", g.Edges)
	}
}

func TestGraphJSONPickPathUnchangedByPhases(t *testing.T) {
	// A pick plan must produce byte-identical output before and after this
	// change set; pin it with a self-comparison through the public surface.
	p := WarpPlan{
		Epic:  Epic{Title: "E", Description: "d"},
		Picks: []Pick{{Ref: "a", Title: "A", Description: "a"}},
	}
	b, err := GraphJSON(p, Derivation{})
	if err != nil {
		t.Fatal(err)
	}
	var g struct {
		Nodes []struct {
			Key  string `json:"key"`
			Type string `json:"type"`
		} `json:"nodes"`
	}
	if err := json.Unmarshal(b, &g); err != nil {
		t.Fatal(err)
	}
	if len(g.Nodes) != 2 || g.Nodes[1].Type != "task" {
		t.Fatalf("pick path regressed: %+v", g.Nodes)
	}
}

func TestRoadmapCounts(t *testing.T) {
	p := WarpPlan{Phases: []Phase{
		{Ref: "p1"}, {Ref: "p2", Needs: []string{"p1"}}, {Ref: "p3", Needs: []string{"p1", "p2"}},
	}}
	nodes, edges := RoadmapCounts(p)
	if nodes != 4 || edges != 3 {
		t.Fatalf("want nodes=4 edges=3, got %d/%d", nodes, edges)
	}
}
```

(Add `"strings"` / `"encoding/json"` to the test file imports if missing.)

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/plan/ -run 'TestGraphJSONRoadmap|TestGraphJSONPickPath|TestRoadmapCounts' -v`
Expected: compile error — `RoadmapCounts` undefined; roadmap test fails (no phase nodes emitted).

- [ ] **Step 3: Implement** (in `internal/plan/emit.go`)

Extract the acceptance-folding that `GraphJSON` already does for the epic into
a helper, and branch on the plan shape:

```go
// foldAcceptance appends an "## Acceptance" section to a description when
// acceptance text is present (the graph node schema's acceptance field is
// unconfirmed — §8 posture — so it must never be a separate field).
func foldAcceptance(desc, acceptance string) string {
	if acceptance == "" {
		return desc
	}
	return strings.TrimRight(desc, "\n") + "\n\n## Acceptance\n" + acceptance
}
```

Replace the inline epic fold in `GraphJSON` with `foldAcceptance(p.Epic.Description, p.Epic.Acceptance)`, then add the phases branch after the epic node is built and before the pick loop:

```go
	if len(p.Phases) > 0 {
		for _, ph := range sortedPhases(p.Phases) {
			nodes = append(nodes, graphNode{
				Key:         ph.Ref,
				Title:       ph.Title,
				Description: foldAcceptance(ph.Description, ph.Acceptance),
				Type:        "epic",
				ParentKey:   EpicKey,
				Labels:      []string{RefLabelPrefix + ph.Ref},
				Priority:    DefaultPriority,
			})
		}
		edges := []graphEdge{}
		for _, ph := range sortedPhases(p.Phases) {
			for _, n := range ph.Needs {
				edges = append(edges, graphEdge{FromKey: ph.Ref, ToKey: n, Type: EdgeType})
			}
		}
		return json.MarshalIndent(graphPlan{Nodes: nodes, Edges: edges}, "", "  ")
	}
```

Add the helpers:

```go
func sortedPhases(phases []Phase) []Phase {
	out := append([]Phase{}, phases...)
	sort.Slice(out, func(i, j int) bool { return out[i].Ref < out[j].Ref })
	return out
}

// RoadmapCounts returns the node/edge counts a roadmap plan's graph payload
// carries, for the seam-9 preflight comparison. Phase edges come from authored
// needs inside GraphJSON — NOT from Derive — so callers must not use
// Derivation.Edges on the roadmap path (it is always empty there).
func RoadmapCounts(p WarpPlan) (nodes, edges int) {
	nodes = 1 + len(p.Phases)
	for _, ph := range p.Phases {
		edges += len(ph.Needs)
	}
	return nodes, edges
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/plan/ -v`
Expected: all PASS, including every pre-existing `GraphJSON` test (the pick path is shared and unchanged).

- [ ] **Step 5: Commit**

Run: `jj --no-pager commit -m "feat(weft-ccy.5): roadmap GraphJSON branch + RoadmapCounts"`

---

### Task 3: `planFirstEmit` roadmap branch — preflight counts + envelopes + `plan check` text

**Files:**

- Modify: `internal/cli/plan.go` (`planFirstEmit`, `planPreviewText`, `newPlanCheckCmd`)
- Test: `internal/cli/plan_test.go`

- [ ] **Step 1: Write the failing tests** (append to `internal/cli/plan_test.go`; reuse the existing `writePlanFile`, `routeRunner`, `dryRunOK`, `newTestCmd` helpers)

```go
const roadmapPlanJSON = `{"epic":{"title":"Proj","description":"d"},"phases":[` +
	`{"ref":"p1","title":"Phase 1","description":"first"},` +
	`{"ref":"p2","title":"Phase 2","description":"second","needs":["p1"]}]}`

func TestPlanCheckRoadmapText(t *testing.T) {
	file := writePlanFile(t, roadmapPlanJSON)
	r := &routeRunner{fn: func(_ string, _ []string) run.Result { return run.Result{} }}
	out, err := newTestCmd(r, "plan", "check", file)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(out.String(), "valid: 2 phase(s)") {
		t.Errorf("roadmap check text must count phases, got %q", out.String())
	}
	if strings.Contains(out.String(), "0 pick(s)") {
		t.Errorf("roadmap check text must not mention picks: %q", out.String())
	}
}

func TestPlanEmitRoadmapDryRunCountsAndEnvelope(t *testing.T) {
	// Roadmap: nodes = epic+2 phases = 3; edges = 1 (p2 needs p1).
	// d.Edges is empty on this path — the counts MUST come from RoadmapCounts.
	file := writePlanFile(t, roadmapPlanJSON)
	r := &routeRunner{fn: func(_ string, args []string) run.Result {
		if strings.Contains(strings.Join(args, " "), "--dry-run") {
			return dryRunOK(3, 1)
		}
		return run.Result{}
	}}
	out, err := newTestCmd(r, "plan", "emit", file, "--dry-run", "--json")
	if err != nil {
		t.Fatalf("roadmap dry-run must pass the preflight count check: %v", err)
	}
	s := out.String()
	if !strings.Contains(s, `"phases": 2`) {
		t.Errorf("roadmap envelope must carry phases count: %q", s)
	}
	if strings.Contains(s, `"picks"`) {
		t.Errorf("picks key must be ABSENT on the roadmap path (not zero): %q", s)
	}
}

func TestPlanEmitRoadmapCountMismatchIsHard(t *testing.T) {
	// bd reporting pick-plan-shaped counts (1 node, 0 edges) for a roadmap must
	// hard-fail — this is the exact bug the design review caught.
	file := writePlanFile(t, roadmapPlanJSON)
	r := &routeRunner{fn: func(_ string, args []string) run.Result {
		if strings.Contains(strings.Join(args, " "), "--dry-run") {
			return dryRunOK(1, 0)
		}
		return run.Result{}
	}}
	if got := exit.Code(runRoot(r, "plan", "emit", file, "--dry-run")); got != 2 {
		t.Fatalf("count mismatch must exit 2, got %d", got)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/cli/ -run 'TestPlanCheckRoadmap|TestPlanEmitRoadmap' -v`
Expected: FAIL. (If Task 2 already landed, `TestPlanEmitRoadmapDryRunCountsAndEnvelope` fails with the hard count-mismatch — preflight expects 1/0, bd reports 3/1 — proving the bug; the check-text test fails on "0 pick(s)". If run before Task 2, the failure is a compile error on `RoadmapCounts` — also a valid red state.)

- [ ] **Step 3: Implement** (in `internal/cli/plan.go`)

In `planFirstEmit`, replace the `CheckPreflight` call:

```go
	wantNodes, wantEdges := 1+len(wp.Picks), len(d.Edges)
	if len(wp.Phases) > 0 {
		// Roadmap path: phase edges come from authored needs inside GraphJSON,
		// not from Derive — d.Edges is always empty here and must not be used.
		wantNodes, wantEdges = plan.RoadmapCounts(wp)
	}
	issues := plan.CheckPreflight(pf, wantNodes, wantEdges)
```

Branch the envelope counts. In the dry-run block:

```go
	data := map[string]any{
		"dry_run": true, "mode": "create", "epic": wp.Epic.Title,
		"edges": d.Edges, "tolerated": d.Tolerated,
		"schema_version": pf.SchemaVersion, "warnings": warnings,
	}
	if len(wp.Phases) > 0 {
		data["phases"] = len(wp.Phases)
	} else {
		data["picks"] = len(wp.Picks)
	}
```

In the live-create block, same branch for `"created"`:

```go
	created := len(wp.Picks)
	if len(wp.Phases) > 0 {
		created = len(wp.Phases)
	}
	data := map[string]any{
		"mode": "create", "created": created, "edges": d.Edges,
		"tolerated": d.Tolerated, "schema_version": pf.SchemaVersion,
		"warnings": warnings, "bd_output": strings.TrimSpace(res.Stdout),
	}
	if len(wp.Phases) > 0 {
		data["phases"] = len(wp.Phases)
	}
```

In `planPreviewText`, branch the headline:

```go
	if len(wp.Phases) > 0 {
		fmt.Fprintf(&b, "DRY RUN (%s) — epic %q, %d phase(s) (roadmap)\n", mode, wp.Epic.Title, len(wp.Phases))
	} else {
		fmt.Fprintf(&b, "DRY RUN (%s) — epic %q, %d pick(s), %d edge(s)\n", mode, wp.Epic.Title, len(wp.Picks), len(d.Edges))
	}
```

In `newPlanCheckCmd`, branch the success text:

```go
	text := fmt.Sprintf("valid: %d pick(s), no issues", len(wp.Picks))
	if len(wp.Phases) > 0 {
		text = fmt.Sprintf("valid: %d phase(s), no issues", len(wp.Phases))
	}
```

Also update the live-emit summary text (`emitted warp: …`) with the same
phase/pick branch (`"emitted roadmap: %d phase(s)"` for the phases path).

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/cli/ -v`
Expected: all PASS — new tests and the entire existing pick-path suite untouched.

- [ ] **Step 5: Commit**

Run: `jj --no-pager commit -m "feat(weft-ccy.5): roadmap emit — preflight count branch, envelope counts, check text"`

---

### Task 4: `ids` echo from `bd create --graph --json`

**Files:**

- Modify: `internal/cli/plan.go` (`planFirstEmit` live-create block)
- Test: `internal/cli/plan_test.go` (also update one existing test stub)
- Modify: `internal/weave/fixture_seed_test.go` (consume `ids`)

- [ ] **Step 1: Write the failing tests**

```go
// graphCreateOK scripts bd's real (non-dry-run) create --graph --json output.
// Shape verified live (bd 1.0.x, 2026-06-10): {"ids":{...},"schema_version":1}.
func graphCreateOK(ids string) run.Result {
	return run.Result{Stdout: `{"ids":` + ids + `,"schema_version":1}`, Code: 0}
}

func TestPlanEmitEchoesIDs(t *testing.T) {
	file := writePlanFile(t, `{"epic":{"title":"E"},"picks":[{"ref":"a","title":"A","description":"a"}]}`)
	r := &routeRunner{fn: func(_ string, args []string) run.Result {
		j := strings.Join(args, " ")
		if strings.Contains(j, "--dry-run") {
			return dryRunOK(2, 0)
		}
		if strings.Contains(j, "create --graph") {
			if !strings.Contains(j, "--json") {
				t.Errorf("real create must pass --json: %v", args)
			}
			return graphCreateOK(`{"@epic":"w-1","a":"w-2"}`)
		}
		return run.Result{}
	}}
	out, err := newTestCmd(r, "plan", "emit", file, "--json")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	s := out.String()
	if !strings.Contains(s, `"@epic": "w-1"`) || !strings.Contains(s, `"a": "w-2"`) {
		t.Errorf("envelope must carry the ids map: %q", s)
	}
}

func TestPlanEmitUnparseableIDsIsHard(t *testing.T) {
	file := writePlanFile(t, `{"epic":{"title":"E"},"picks":[{"ref":"a","title":"A","description":"a"}]}`)
	r := &routeRunner{fn: func(_ string, args []string) run.Result {
		if strings.Contains(strings.Join(args, " "), "--dry-run") {
			return dryRunOK(2, 0)
		}
		return run.Result{Stdout: "created weft-zzz", Code: 0} // pre-ids legacy stdout
	}}
	if got := exit.Code(runRoot(r, "plan", "emit", file)); got != 2 {
		t.Fatalf("unparseable ids must exit 2 (loud, never degraded), got %d", got)
	}
}

func TestPlanEmitRoadmapEchoesPhaseIDs(t *testing.T) {
	file := writePlanFile(t, roadmapPlanJSON)
	r := &routeRunner{fn: func(_ string, args []string) run.Result {
		if strings.Contains(strings.Join(args, " "), "--dry-run") {
			return dryRunOK(3, 1)
		}
		return graphCreateOK(`{"@epic":"w-1","p1":"w-2","p2":"w-3"}`)
	}}
	out, err := newTestCmd(r, "plan", "emit", file, "--json")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(out.String(), `"p1": "w-2"`) {
		t.Errorf("roadmap envelope must carry phase ids: %q", out.String())
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/cli/ -run 'TestPlanEmitEchoesIDs|TestPlanEmitUnparseable|TestPlanEmitRoadmapEchoes' -v`
Expected: FAIL — no `--json` on the create call, no `ids` in the envelope, legacy stdout accepted.

- [ ] **Step 3: Implement** (in `planFirstEmit`'s live-create block)

Change the create invocation and parse the result:

```go
	res, err := run.BD(a.Runner, "create", "--graph", path, "--json")
	if err != nil {
		return exit.Hardf("bd create --graph could not run: %v", err)
	}
	if res.Code != 0 {
		return exit.Hardf("bd create --graph failed: %s", strings.TrimSpace(res.Stderr))
	}
	// Parse the ids map (node key -> created bead id). Shape verified live:
	// {"ids":{"@epic":"...","<ref>":"..."},"schema_version":1}. The warp was
	// already created at this point, so a parse failure is a hard error (loud,
	// never a silently degraded envelope) — the operator must investigate.
	var applied struct {
		IDs map[string]string `json:"ids"`
	}
	if err := json.Unmarshal([]byte(res.Stdout), &applied); err != nil || len(applied.IDs) == 0 {
		return exit.Hardf("warp created but bd create --graph --json output is unparseable (ids missing) — investigate before re-running (a re-run would duplicate the warp): %v\noutput: %s",
			err, strings.TrimSpace(res.Stdout))
	}
```

Add `"ids": applied.IDs` to the live-emit `data` map (both shapes), and append
the epic id to the human text: `fmt.Sprintf("emitted …\nepic: %s", applied.IDs["@epic"])`.

**Update the existing stubs** that script the real (non-dry-run) create with
legacy pre-ids stdout — after this change they hard-fail by design. Replace
each with `graphCreateOK(`{"@epic":"w-1","a":"w-2"}`)` (adjust refs to the
test's plan):

- `TestPlanEmitFirstCreatesGraph` (~line 107)
- `TestPlanEmitDropWithAllowSurfacesWarningAndProceeds` (~line 383)
- `TestPlanEmitSchemaMismatchIsSoft` (~line 423)
- `TestPlanEmitPreflightNoteAppearsInWarnings` (~line 472)

Then `rg -n '"created' internal/cli/plan_test.go` to confirm no other stub
remains.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/cli/ -v`
Expected: all PASS.

- [ ] **Step 5: Migrate the seam-10 fixture to consume `ids`**

In `internal/weave/fixture_seed_test.go`, `seedFixture` currently runs plan emit
for its side effect, then recovers the epic id via `r.onlyEpicID(t)` and the
ref→id map by scanning child labels. Replace both with the envelope contract:

```go
	env := r.runWeft(t, "", "plan", "emit", warp)
	var data struct {
		IDs map[string]string `json:"ids"`
	}
	if err := json.Unmarshal(env.Data, &data); err != nil || len(data.IDs) == 0 {
		t.Fatalf("seedFixture: plan emit envelope has no ids map: %v", err)
	}
	epicID := data.IDs["@epic"]
	byRef := map[string]string{}
	for ref, id := range data.IDs {
		if ref != "@epic" {
			byRef[ref] = id
		}
	}
```

Keep the existing per-ref presence assertions. Delete `onlyEpicID` (and the
label-scanning loop) if nothing else uses them — check with
`rg -n 'onlyEpicID|childBeads' internal/weave/` and remove only what becomes dead.
Update the now-stale comment ("The plan emit data field does NOT contain the
epic bead id") — it is the contract now.

- [ ] **Step 6: Run the integration suite**

Run: `go test -tags integration ./internal/weave/ -run TestWeave -v -timeout 600s`
Expected: PASS (the fixture seeds through the new contract).

- [ ] **Step 7: Commit**

Run: `jj --no-pager commit -m "feat(weft-ccy.5): plan emit echoes created ids; seam-10 fixture consumes the contract"`

---

### Task 5: §8 — apply deferred edges post-import (`applied_edges`)

**Files:**

- Modify: `internal/plan/verify.go` (`ReadbackBead` gains `ID`)
- Modify: `internal/cli/plan.go` (`warpReadback`, `planReplan`, `replanText`)
- Test: `internal/cli/plan_test.go`

- [ ] **Step 1: Write the failing tests**

The existing replan tests stub `bd list` responses — follow their pattern
(read `TestPlanEmitReplanUpsertsMatchedRef` at `internal/cli/plan_test.go:184`
first; it shows the `list --parent` stub shape). Append:

```go
func TestPlanEmitReplanAppliesDeferredEdges(t *testing.T) {
	// Plan: existing pick a (matched), new pick b with needs:[a] -> the a<-b
	// edge is deferred past import and must be wired via bd dep add using the
	// post-import readback ids.
	file := writePlanFile(t, `{"epic":{"title":"E"},"picks":[`+
		`{"ref":"a","title":"A","description":"a"},`+
		`{"ref":"b","title":"B","description":"b","needs":["a"]}]}`)
	preImport := `[{"id":"w-a","status":"open","title":"A","priority":2,"labels":["weft-ref:a"],"description":"a"}]`
	postImport := `[{"id":"w-a","status":"open","title":"A","priority":2,"labels":["weft-ref:a"],"description":"a"},` +
		`{"id":"w-b","status":"open","title":"B","priority":2,"labels":["weft-ref:b"],"description":"b"}]`
	listCalls := 0
	var depArgs []string
	r := &routeRunner{fn: func(_ string, args []string) run.Result {
		j := strings.Join(args, " ")
		switch {
		case strings.Contains(j, "list --parent"):
			listCalls++
			if listCalls == 1 {
				return run.Result{Stdout: preImport, Code: 0}
			}
			return run.Result{Stdout: postImport, Code: 0}
		case strings.Contains(j, "dep add"):
			depArgs = args
			return run.Result{Code: 0}
		}
		return run.Result{Code: 0}
	}}
	out, err := newTestCmd(r, "plan", "emit", file, "--epic", "w-epic", "--json")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	want := strings.Join([]string{"dep", "add", "w-b", "w-a", "--type", "blocks"}, " ")
	if strings.Join(depArgs, " ") != want {
		t.Errorf("dep add args = %v, want %q", depArgs, want)
	}
	if !strings.Contains(out.String(), `"applied_edges"`) || strings.Contains(out.String(), `"deferred_edges"`) {
		t.Errorf("envelope key must be applied_edges (renamed): %q", out.String())
	}
}

func TestPlanEmitReplanDepAddFailureIsHard(t *testing.T) {
	file := writePlanFile(t, `{"epic":{"title":"E"},"picks":[`+
		`{"ref":"a","title":"A","description":"a"},`+
		`{"ref":"b","title":"B","description":"b","needs":["a"]}]}`)
	preImport := `[{"id":"w-a","status":"open","title":"A","priority":2,"labels":["weft-ref:a"],"description":"a"}]`
	postImport := `[{"id":"w-a","status":"open","title":"A","priority":2,"labels":["weft-ref:a"],"description":"a"},` +
		`{"id":"w-b","status":"open","title":"B","priority":2,"labels":["weft-ref:b"],"description":"b"}]`
	listCalls := 0
	r := &routeRunner{fn: func(_ string, args []string) run.Result {
		j := strings.Join(args, " ")
		switch {
		case strings.Contains(j, "list --parent"):
			listCalls++
			if listCalls == 1 {
				return run.Result{Stdout: preImport, Code: 0}
			}
			return run.Result{Stdout: postImport, Code: 0}
		case strings.Contains(j, "dep add"):
			return run.Result{Code: 1, Stderr: "boom"}
		}
		return run.Result{Code: 0}
	}}
	if got := exit.Code(runRoot(r, "plan", "emit", file, "--epic", "w-epic")); got != 2 {
		t.Fatalf("dep add failure must exit 2, got %d", got)
	}
}

func TestPlanEmitReplanUnresolvableEdgeIsHard(t *testing.T) {
	// Post-import readback missing the new pick (bd silently didn't create it):
	// the deferred edge cannot resolve -> hard fail, warp incomplete.
	file := writePlanFile(t, `{"epic":{"title":"E"},"picks":[`+
		`{"ref":"a","title":"A","description":"a"},`+
		`{"ref":"b","title":"B","description":"b","needs":["a"]}]}`)
	preImport := `[{"id":"w-a","status":"open","title":"A","priority":2,"labels":["weft-ref:a"],"description":"a"}]`
	r := &routeRunner{fn: func(_ string, args []string) run.Result {
		if strings.Contains(strings.Join(args, " "), "list --parent") {
			return run.Result{Stdout: preImport, Code: 0} // same both times: b never appears
		}
		return run.Result{Code: 0}
	}}
	if got := exit.Code(runRoot(r, "plan", "emit", file, "--epic", "w-epic")); got != 2 {
		t.Fatalf("unresolvable deferred edge must exit 2, got %d", got)
	}
}
```

Note: the unresolvable case will first trip `VerifyReplan`'s read-back check
(pick b's fields never round-tripped) — that is fine; either hard-fail path
satisfies the contract. If `VerifyReplan` fires first, keep the test but assert
exit 2 only (as written).

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/cli/ -run 'TestPlanEmitReplanApplies|TestPlanEmitReplanDepAdd|TestPlanEmitReplanUnresolvable' -v`
Expected: FAIL — no `dep add` call is made; envelope still says `deferred_edges`.

- [ ] **Step 3: Implement**

In `internal/plan/verify.go`, add `ID string` to `ReadbackBead` (document: "bead
id, for post-import edge wiring; ignored by VerifyReplan"). In
`internal/cli/plan.go` `warpReadback`, populate `ID: it.ID`.

In `planReplan`, after the `VerifyReplan` check and before building the final
`data` map:

```go
	// §8: wire edges that touched a new pick — bd import cannot forward-
	// reference ids inside a batch, so they are applied post-import from the
	// readback map. Any failure leaves the warp structurally incomplete: hard.
	for _, e := range rp.DeferredEdges {
		from, fok := readback[e.From]
		to, tok := readback[e.To]
		if !fok || !tok || from.ID == "" || to.ID == "" {
			return exit.Hardf("re-plan applied but edge %s->%s could not be resolved post-import; the warp is incomplete — investigate", e.From, e.To)
		}
		dep, err := run.BD(a.Runner, "dep", "add", from.ID, to.ID, "--type", "blocks")
		if err != nil {
			return exit.Hardf("re-plan applied but bd dep add could not run for %s->%s: %v", e.From, e.To, err)
		}
		if dep.Code != 0 {
			return exit.Hardf("re-plan applied but edge %s->%s could not be wired: %s — the warp is incomplete; investigate", e.From, e.To, strings.TrimSpace(dep.Stderr))
		}
	}
```

Rename the envelope key in BOTH the dry-run and live `data` maps:
`"deferred_edges": rp.DeferredEdges` → `"applied_edges": rp.DeferredEdges`
(dry-run reports the edges that WILL be applied; `dry_run: true` contextualizes).

Update `replanText`: the deferred-edges line becomes

```go
		fmt.Fprintf(&b, "  %d edge(s) touch a new pick — wired post-import: %s\n",
			len(rp.DeferredEdges), strings.Join(parts, ", "))
```

with a `dry` variant `"— will wire post-import: "`. Update the two existing
replan tests that assert on `deferred_edges` / the old text if any do
(`rg -n 'deferred_edges|wire after creation' internal/cli/`).

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/cli/ ./internal/plan/ -v`
Expected: all PASS.

- [ ] **Step 5: Commit**

Run: `jj --no-pager commit -m "feat(weft-ccy.5): apply deferred re-plan edges post-import (applied_edges)"`

---

### Task 6: Integration — roadmap round-trip + transitive-gating pin

**Files:**

- Create: `internal/weave/roadmap_integration_test.go`

- [ ] **Step 1: Read the harness** — `internal/weave/harness_test.go` (`newScratchRepo`, `runWeft`, `mustBD`, the `envelope` type) and the `//go:build integration` tag convention in `fixture_seed_test.go`. The new file lives in the same package and reuses all of it.

- [ ] **Step 2: Write the test**

```go
// internal/weave/roadmap_integration_test.go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Weft Contributors

//go:build integration

package weave

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// bdJSON runs bd against the scratch repo and returns stdout (for assertions
// the harness's mustBD, which discards output, cannot make).
func bdJSON(t *testing.T, r *scratchRepo, args ...string) []byte {
	t.Helper()
	cmd := exec.Command("bd", args...)
	cmd.Dir = r.root
	cmd.Env = append(os.Environ(), "BEADS_DIR="+r.beadsDir)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("bd %s: %v", strings.Join(args, " "), err)
	}
	return out
}

func readyIDs(t *testing.T, r *scratchRepo) map[string]bool {
	t.Helper()
	var arr []struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(bdJSON(t, r, "ready", "--json"), &arr); err != nil {
		t.Fatalf("parse bd ready: %v", err)
	}
	m := map[string]bool{}
	for _, it := range arr {
		m[it.ID] = true
	}
	return m
}

func TestRoadmapEmitAndTransitiveGating(t *testing.T) {
	r := newScratchRepo(t)

	// 1. Roadmap emit: project epic -> two phase sub-epics, p2 blocked by p1.
	warp := filepath.Join(r.root, "roadmap.json")
	if err := os.WriteFile(warp, []byte(`{"epic":{"title":"Proj","description":"d"},"phases":[`+
		`{"ref":"p1","title":"Phase 1","description":"first"},`+
		`{"ref":"p2","title":"Phase 2","description":"second","needs":["p1"]}]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	env := r.runWeft(t, "", "plan", "emit", warp)
	var data struct {
		IDs map[string]string `json:"ids"`
	}
	if err := json.Unmarshal(env.Data, &data); err != nil {
		t.Fatalf("parse emit data: %v", err)
	}
	for _, k := range []string{"@epic", "p1", "p2"} {
		if data.IDs[k] == "" {
			t.Fatalf("ids map missing %q: %v", k, data.IDs)
		}
	}

	// 2. Gating pin: a pick created under the BLOCKED p2 epic must not be
	// ready (bd's transitive epic gating — the behavior the phased model
	// depends on; if bd regresses, this test fails, not a live weave).
	r.mustBD(t, "create", "--title", "early pick", "--type", "task", "--parent", data.IDs["p2"])
	var children []struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(bdJSON(t, r, "list", "--parent", data.IDs["p2"], "--json"), &children); err != nil || len(children) != 1 {
		t.Fatalf("expected exactly the early pick under p2: %v err=%v", children, err)
	}
	early := children[0].ID
	if readyIDs(t, r)[early] {
		t.Fatalf("pick %s under blocked phase epic must NOT be ready", early)
	}

	// 3. Close phase 1 -> the early pick (and p2) become ready.
	r.mustBD(t, "close", data.IDs["p1"], "--reason=phase 1 shipped")
	if !readyIDs(t, r)[early] {
		t.Fatalf("pick %s must be ready once the blocking phase closes", early)
	}
}

func TestPerPhaseReplanAppliesNewPickEdges(t *testing.T) {
	r := newScratchRepo(t)

	// Roadmap with one phase, then JIT-plan two picks (b needs a) into it.
	warp := filepath.Join(r.root, "roadmap.json")
	if err := os.WriteFile(warp, []byte(`{"epic":{"title":"Proj","description":"d"},"phases":[`+
		`{"ref":"p1","title":"Phase 1","description":"first"}]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	env := r.runWeft(t, "", "plan", "emit", warp)
	var data struct {
		IDs map[string]string `json:"ids"`
	}
	if err := json.Unmarshal(env.Data, &data); err != nil {
		t.Fatalf("parse emit data: %v", err)
	}
	phase := data.IDs["p1"]

	picks := filepath.Join(r.root, "phase1-picks.json")
	if err := os.WriteFile(picks, []byte(`{"epic":{"title":"Phase 1","description":"d"},"picks":[`+
		`{"ref":"a","title":"A","description":"a"},`+
		`{"ref":"b","title":"B","description":"b","needs":["a"]}]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	env = r.runWeft(t, "", "plan", "emit", picks, "--epic", phase)
	var rep struct {
		Applied []any `json:"applied_edges"`
	}
	if err := json.Unmarshal(env.Data, &rep); err != nil {
		t.Fatalf("parse replan data: %v", err)
	}
	if len(rep.Applied) != 1 {
		t.Fatalf("want 1 applied edge, got %v", rep.Applied)
	}

	// The b->a edge is live iff bd ready scoped to the phase shows ONLY a.
	ready := readyIDs(t, r)
	var children []struct {
		ID     string   `json:"id"`
		Labels []string `json:"labels"`
	}
	if err := json.Unmarshal(bdJSON(t, r, "list", "--parent", phase, "--json"), &children); err != nil {
		t.Fatal(err)
	}
	byRef := map[string]string{}
	for _, c := range children {
		for _, l := range c.Labels {
			if strings.HasPrefix(l, "weft-ref:") {
				byRef[strings.TrimPrefix(l, "weft-ref:")] = c.ID
			}
		}
	}
	if !ready[byRef["a"]] {
		t.Fatalf("pick a must be ready")
	}
	if ready[byRef["b"]] {
		t.Fatalf("pick b must be blocked by the applied b->a edge")
	}
}
```

- [ ] **Step 3: Run the integration tests**

Run: `go test -tags integration ./internal/weave/ -run 'TestRoadmap|TestPerPhase' -v -timeout 600s`
Expected: PASS. (If `bd ready --json`'s output shape differs from `[{"id":...}]`, fix `readyIDs` against the real output — verify with a manual `bd ready --json` in any scratch repo first.)

- [ ] **Step 4: Commit**

Run: `jj --no-pager commit -m "test(weft-ccy.5): integration — roadmap round-trip + transitive-gating pin + per-phase replan edges"`

---

### Task 7: Seam-2 doc updates + full gates

**Files:**

- Modify: `docs/seams/02-planning-emission.md`

- [ ] **Step 1: Update the seam doc** (targeted edits, not a rewrite):

- §3 (warp-plan shape): add the `phases[]` block with the Task 1 field list and the mutual-exclusion rule; note `ref` reuses the pick ref contract and lands as `weft-ref:<ref>` on the phase sub-epic.
- §5 (verbs table): `plan emit` row gains "roadmap plans (phases[]) create project epic → phase sub-epics → inter-phase blocks edges; envelope carries `ids` (node key → bead id) on every create".
- §6 (warp structure): note the deliberate extension — one warp-plan → one epic *or* one roadmap (project epic + phase sub-epics); phase = ship unit (one PR per phase via `finish`).
- §7 (re-plan): new-pick edges are now applied post-import via `bd dep add` (envelope key `applied_edges`); the §8 open list shrinks to removed-pick supersede.
- §8: strike the new-pick-edges sub-seam (done; cite bead `weft-ccy.5`), keep supersede.

- [ ] **Step 2: Full gates**

Run:

```bash
go test ./... && go vet ./... && go build -o /dev/null ./cmd/weft
go test -tags integration ./internal/weave/ -timeout 900s
```

Expected: all green.

- [ ] **Step 3: Commit**

Run: `jj --no-pager commit -m "docs(weft-ccy.5): seam-2 — phases[] roadmap emission, ids contract, applied_edges"`

---

## Done criteria

- All gates green (`go test ./...`, `go vet`, build, integration suite).
- A roadmap plan round-trips `plan check` + `plan emit` into project epic → phase sub-epics → blocks edges, with `ids` echoed.
- Re-plan applies new-pick edges; `applied_edges` in the envelope; hard-fail on any unwired edge.
- Pick plans behave identically to today (regression suite + golden tests prove it).
- `bd close weft-ccy.5` (after review); `bd dolt push`.

## Out of scope (later phases / open sub-seams)

- Phased planner prompts + `new-project` changes (weft-ccy.3, phase C).
- Removed-pick supersede (§8 remainder).
- Hybrid plans (roadmap + picks in one file).
