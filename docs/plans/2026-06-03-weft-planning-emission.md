<!--
  ~ SPDX-License-Identifier: Apache-2.0
  ~ Copyright 2026 Weft Contributors
-->

# Weft Planning → Warp Emission (Seam 2) — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement the seam-2 `weft plan` verbs — `plan check <file>` (validate `warp-plan.json`) and `plan emit <file> [--dry-run] [--epic <id>]` (derive file-overlap dependency edges, preview, then `bd create --graph` for a new warp or `bd import` upsert for a re-plan) — so an authored plan becomes the bead graph (the warp).

**Architecture:** A new pure `internal/plan` package models `warp-plan.json` and the deterministic transforms: validation (spec §5), file-overlap edge derivation (§4.2), and the `bd create --graph` / `bd import` payload builders (§6/§7). Thin cobra verbs in `internal/cli/plan.go` wrap that package, reaching `bd` through the existing injectable `run.Runner` (ADR `weft-re2`). The dry-run preview is the human approval gate; a warn+tolerate overlap is **data on exit 0**, never an error (seam-1 output contract).

**Tech Stack:** Go 1.26, `github.com/spf13/cobra` v1.10.x, stdlib `encoding/json`/`sort`/`strings`/`path/filepath`/`os`. Subprocess: `bd`.

**Spec:** `docs/seams/02-planning-emission.md` (READY round 3). `warp-plan.json` §3; dep derivation + advisory overlap policy §4; verbs §5; warp structure §6; re-planning §7.

**Grounded wire contracts** (probed live via `bd ... --help` / `--dry-run` this design session; recorded as notes on `weft-hjx.3`):

- `bd create --graph <file>` JSON: `{"nodes":[{"key","title","description","type","parent_key","labels","priority"}], "edges":[{"from_key","to_key","type"}]}`. Epic node `type:"epic"` (no `parent_key`); picks default `type:"task"`, `parent_key=<epic key>`. **Unknown fields are silently dropped (warning to stderr)** — so the spec's authoring names (`ref`/`needs`/`epic`/`picks`) MUST be mapped to the wire names (`key`/`edges`/`nodes`). Supports `--dry-run` and `--json`.
- Edge semantics: `{from_key, to_key, type:"blocks"}` means **`from_key` depends on `to_key`** (`from` is blocked by `to`), matching `bd dep add <issue> <depends-on>` (default type `blocks`).
- `bd import <file>` JSONL upserts **by `id`** (omitted `id` ⇒ create; present ⇒ update). Accepts the `bd export` schema; only `title` required. Fields used: `id`, `title`, `description`, `issue_type`, `priority`, `status`, `parent` (top-level), `labels`, `dependencies:[{issue_id, depends_on_id, type}]`.
- `bd list --parent <epic> --json` returns each child's `id`, `status`, and `labels` in one call ⇒ the `ref → bead-id` map is rebuilt by scanning `weft-ref:<ref>` labels (no N+1 `bd show`).

**Out of scope (explicit §8 sub-seams — surfaced as data, not applied):** wiring re-plan dependency edges that touch a *newly created* pick (no bead-id yet ⇒ reported as `deferred_edges`); superseding *removed* picks via `bd supersede` (reported as `removed`); a formal `warp-plan.json` JSON Schema; `**` glob support in `[plan].structural`; declared-vs-actual file-drift detection; `has_checkpoint`. Also out of scope: seam-4 conflict UX and the ship verbs.

---

## File Structure

| File | Responsibility |
|---|---|
| `internal/plan/plan.go` (create) | `WarpPlan`/`Epic`/`Pick`/`Issue` types, `Parse`/`Load`, `Validate` (§5), `EpicKey` const. |
| `internal/plan/overlap.go` (create) | `Edge`/`Overlap`/`Derivation`, `Derive` (§4.2 advisory policy), `isStructural` glob match. |
| `internal/plan/emit.go` (create) | `GraphJSON` (first-emit `bd create --graph` payload, §6), `BuildReplan`/`ExistingBead`/`Replan` (re-plan `bd import` payload + deltas, §7), `RefLabelPrefix`/`EdgeType`/`DefaultPriority`. |
| `internal/config/config.go` (modify) | Add `[plan]` block (`structural` globs, `overlap_max`) + `PlanStructural()`/`PlanOverlapMax()` accessors + defaults. |
| `internal/cli/plan.go` (create) | `weft plan` group: `check`, `emit` (`--dry-run`, `--epic`); first-emit + re-plan paths; `warpRefMap` helper. |
| `internal/cli/root.go` (modify) | Register `plan`. |

Tests live beside each file. The `internal/plan` package is pure and exhaustively unit-tested (Tasks 1–5); the CLI verbs (Tasks 6–8) are thin and tested with the existing `routeRunner` fake + `newTestCmd`/`runRoot` helpers (defined in `internal/cli/*_test.go`).

**Project output contract (enforced):** every list-emitting field MUST be initialized as `[]T{}` (never a nil slice — nil serializes to JSON `null` and breaks `--json` consumers). `Derive`, `Validate`, and `BuildReplan` already return initialized slices; keep it that way and assert it where natural.

---

### Task 1: `internal/plan/plan.go` — types, parse/load, validate

**Files:**
- Create: `internal/plan/plan.go`
- Test: `internal/plan/plan_test.go`

- [ ] **Step 1: Write the failing test**

```go
// internal/plan/plan_test.go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Weft Contributors

package plan

import "testing"

func TestValidateAcceptsWellFormedPlan(t *testing.T) {
	p := WarpPlan{
		Epic: Epic{Title: "E"},
		Picks: []Pick{
			{Ref: "p1", Title: "A", Description: "do A"},
			{Ref: "p2", Title: "B", Description: "do B", Needs: []string{"p1"}},
		},
	}
	if got := Validate(p); len(got) != 0 {
		t.Fatalf("want valid, got issues: %+v", got)
	}
}

func TestValidateFlagsMissingFieldsAndBadNeeds(t *testing.T) {
	p := WarpPlan{
		Epic: Epic{},
		Picks: []Pick{
			{Ref: "p1", Title: "A"},                                            // missing description
			{Ref: "p1", Description: "dup"},                                    // duplicate ref + missing title
			{Ref: "p3", Title: "C", Description: "c", Needs: []string{"nope", "p3"}}, // unknown + self need
		},
	}
	issues := Validate(p)
	want := map[string]bool{
		"epic.title is required": false,
		"pick.description is required (the bead description is the plan)": false,
		"duplicate pick.ref":             false,
		`pick.needs references unknown ref "nope"`: false,
		"pick.needs references itself":   false,
	}
	for _, is := range issues {
		if _, ok := want[is.Message]; ok {
			want[is.Message] = true
		}
	}
	for msg, seen := range want {
		if !seen {
			t.Errorf("expected issue %q; issues=%+v", msg, issues)
		}
	}
}

func TestParseReadsPickFields(t *testing.T) {
	src := []byte(`{"epic":{"title":"E","description":"d"},"picks":[{"ref":"p1","title":"A","description":"a","files":["x.go"],"priority":1}]}`)
	p, err := Parse(src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if p.Epic.Title != "E" || len(p.Picks) != 1 || p.Picks[0].Ref != "p1" {
		t.Fatalf("bad parse: %+v", p)
	}
	if p.Picks[0].Priority == nil || *p.Picks[0].Priority != 1 {
		t.Fatalf("priority not parsed: %+v", p.Picks[0])
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/plan/ -run 'TestValidate|TestParse'`
Expected: FAIL — `undefined: WarpPlan` / `undefined: Validate`.

- [ ] **Step 3: Write the implementation**

```go
// internal/plan/plan.go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Weft Contributors

// Package plan models warp-plan.json (spec §3) and the pure transforms that
// turn it into the bead graph: validation (§5 plan check), file-overlap
// dependency derivation (§4), and the bd create --graph / bd import payloads
// (§6/§7). The CLI verbs in internal/cli/plan.go are thin wrappers over this.
package plan

import (
	"encoding/json"
	"fmt"
	"os"
)

// Epic is the warp-plan.json epic block (spec §3): the ship unit (§6).
type Epic struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Acceptance  string `json:"acceptance,omitempty"`
}

// Pick is one authored pick (spec §3): one bead -> one jj change. Ref is the
// stable, plan-local identity key — the durable plan<->warp join (§3/§7).
type Pick struct {
	Ref         string   `json:"ref"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Needs       []string `json:"needs,omitempty"`
	Files       []string `json:"files,omitempty"`
	Priority    *int     `json:"priority,omitempty"` // pointer: distinguishes unset from 0 (P0)
	Labels      []string `json:"labels,omitempty"`
}

// WarpPlan is the whole authored artifact (spec §3).
type WarpPlan struct {
	Epic  Epic   `json:"epic"`
	Picks []Pick `json:"picks"`
}

// Issue is one validation problem (an element of plan check's output, spec §5).
type Issue struct {
	Ref     string `json:"ref,omitempty"`
	Message string `json:"message"`
}

// EpicKey is the internal bd create --graph node key for the epic. It is not a
// valid pick ref (Validate rejects a colliding ref), so it never clashes.
const EpicKey = "@epic"

// Parse decodes warp-plan.json bytes. Unknown fields are tolerated for
// forward-compatibility; Validate enforces the required shape.
func Parse(b []byte) (WarpPlan, error) {
	var p WarpPlan
	if err := json.Unmarshal(b, &p); err != nil {
		return WarpPlan{}, fmt.Errorf("parse warp-plan json: %w", err)
	}
	return p, nil
}

// Load reads and parses warp-plan.json from disk.
func Load(path string) (WarpPlan, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return WarpPlan{}, fmt.Errorf("read warp-plan %s: %w", path, err)
	}
	return Parse(b)
}

// Validate checks the plan against the spec §5 contract and returns the issues
// found (empty => valid). It is pure and never mutates. Cycle detection is left
// to bd create --graph's own validation at emit time.
func Validate(p WarpPlan) []Issue {
	issues := []Issue{}
	if p.Epic.Title == "" {
		issues = append(issues, Issue{Message: "epic.title is required"})
	}
	if len(p.Picks) == 0 {
		issues = append(issues, Issue{Message: "at least one pick is required"})
	}
	seen := map[string]bool{}
	for _, pk := range p.Picks {
		switch {
		case pk.Ref == "":
			issues = append(issues, Issue{Message: "pick.ref is required"})
			continue
		case pk.Ref == EpicKey:
			issues = append(issues, Issue{Ref: pk.Ref, Message: fmt.Sprintf("pick.ref %q is reserved", EpicKey)})
		case seen[pk.Ref]:
			issues = append(issues, Issue{Ref: pk.Ref, Message: "duplicate pick.ref"})
		}
		seen[pk.Ref] = true
		if pk.Title == "" {
			issues = append(issues, Issue{Ref: pk.Ref, Message: "pick.title is required"})
		}
		if pk.Description == "" {
			issues = append(issues, Issue{Ref: pk.Ref, Message: "pick.description is required (the bead description is the plan)"})
		}
	}
	// needs must reference a known ref and not the pick itself (seen is complete
	// after the first loop).
	for _, pk := range p.Picks {
		for _, n := range pk.Needs {
			if n == pk.Ref {
				issues = append(issues, Issue{Ref: pk.Ref, Message: "pick.needs references itself"})
				continue
			}
			if !seen[n] {
				issues = append(issues, Issue{Ref: pk.Ref, Message: fmt.Sprintf("pick.needs references unknown ref %q", n)})
			}
		}
	}
	return issues
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/plan/ -run 'TestValidate|TestParse'`
Expected: PASS.

- [ ] **Step 5: Commit**

Run: `jj commit -m "feat(plan): warp-plan.json types, parse/load, validate (seam 2 §5)"`

---

### Task 2: `internal/config/config.go` — the `[plan]` block

**Files:**
- Modify: `internal/config/config.go`
- Test: `internal/config/config_test.go`

- [ ] **Step 1: Write the failing test**

Append to `internal/config/config_test.go`:

```go
func TestPlanConfigDefaults(t *testing.T) {
	var c Config
	if c.PlanOverlapMax() != DefaultOverlapMax {
		t.Errorf("default overlap_max = %d, want %d", c.PlanOverlapMax(), DefaultOverlapMax)
	}
	if len(c.PlanStructural()) == 0 {
		t.Errorf("default structural must be non-empty")
	}
}

func TestLoadParsesPlanBlock(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte("[plan]\nstructural = [\"schema.sql\"]\noverlap_max = 0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if len(cfg.PlanStructural()) != 1 || cfg.PlanStructural()[0] != "schema.sql" {
		t.Errorf("structural = %v", cfg.PlanStructural())
	}
	if cfg.PlanOverlapMax() != 0 {
		t.Errorf("overlap_max = %d, want 0 (explicitly configured)", cfg.PlanOverlapMax())
	}
}
```

(`internal/config/config_test.go` already imports `os`, `path/filepath`, `testing` — confirm; if not, add them.)

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/config/ -run 'TestPlanConfig|TestLoadParsesPlan'`
Expected: FAIL — `cfg.PlanStructural` undefined.

- [ ] **Step 3: Add the `[plan]` block + accessors**

In `internal/config/config.go`, extend the `Config` struct with a `Plan` block (after `Verify`):

```go
	Verify struct {
		Command string `toml:"command"`
	} `toml:"verify"`
	Plan struct {
		Structural []string `toml:"structural"`
		OverlapMax *int     `toml:"overlap_max"` // pointer: distinguishes unset from an explicit 0
	} `toml:"plan"`
```

Add the defaults and accessors at the end of the file:

```go
// DefaultOverlapMax tolerates a single shared incidental (non-structural) file
// between same-shed picks; 2+ shared files serialize (spec §4.2).
const DefaultOverlapMax = 1

// DefaultStructural is the language-agnostic starter set of files whose
// concurrent edit is almost always a real conflict (spec §4.2). Globs match the
// path or its basename via filepath.Match; ** is unsupported (a §8 refinement).
func DefaultStructural() []string {
	return []string{"go.mod", "go.sum", "package.json", "package-lock.json", "Cargo.toml", "Cargo.lock", "*.lock"}
}

// PlanStructural returns the configured structural globs, or the defaults when
// none are set.
func (c Config) PlanStructural() []string {
	if len(c.Plan.Structural) == 0 {
		return DefaultStructural()
	}
	return c.Plan.Structural
}

// PlanOverlapMax returns the configured incidental-overlap tolerance, or the
// default when unset. A negative configured value clamps to 0 (serialize on any
// non-structural overlap).
func (c Config) PlanOverlapMax() int {
	if c.Plan.OverlapMax == nil {
		return DefaultOverlapMax
	}
	if *c.Plan.OverlapMax < 0 {
		return 0
	}
	return *c.Plan.OverlapMax
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/config/ -run 'TestPlanConfig|TestLoadParsesPlan'`
Expected: PASS.

- [ ] **Step 5: Commit**

Run: `jj commit -m "feat(config): [plan] structural globs + overlap_max (seam 2 §4.2)"`

---

### Task 3: `internal/plan/overlap.go` — dependency derivation

**Files:**
- Create: `internal/plan/overlap.go`
- Test: `internal/plan/overlap_test.go`

- [ ] **Step 1: Write the failing test**

```go
// internal/plan/overlap_test.go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Weft Contributors

package plan

import (
	"reflect"
	"testing"
)

func edgePairs(es []Edge) [][2]string {
	out := [][2]string{}
	for _, e := range es {
		out = append(out, [2]string{e.From, e.To})
	}
	return out
}

func TestDeriveExplicitNeedsBecomeEdges(t *testing.T) {
	picks := []Pick{{Ref: "p2", Needs: []string{"p1"}}, {Ref: "p1"}}
	d := Derive(picks, nil, 1)
	if !reflect.DeepEqual(edgePairs(d.Edges), [][2]string{{"p2", "p1"}}) {
		t.Fatalf("edges = %v", edgePairs(d.Edges))
	}
}

func TestDeriveStructuralOverlapSerializes(t *testing.T) {
	picks := []Pick{
		{Ref: "b", Files: []string{"go.mod", "b.go"}},
		{Ref: "a", Files: []string{"go.mod", "a.go"}},
	}
	d := Derive(picks, []string{"go.mod"}, 5) // overlapMax high; structural still serializes
	if len(d.Edges) != 1 || d.Edges[0].From != "b" || d.Edges[0].To != "a" {
		t.Fatalf("structural overlap should serialize b->a, got %v", edgePairs(d.Edges))
	}
	if len(d.Tolerated) != 0 {
		t.Fatalf("structural overlap must not be tolerated: %v", d.Tolerated)
	}
}

func TestDeriveIncidentalOverlapTolerated(t *testing.T) {
	picks := []Pick{
		{Ref: "a", Files: []string{"shared.go", "a.go"}},
		{Ref: "b", Files: []string{"shared.go", "b.go"}},
	}
	d := Derive(picks, []string{"go.mod"}, 1) // 1 shared, <= max => tolerate
	if len(d.Edges) != 0 {
		t.Fatalf("incidental overlap must not serialize: %v", edgePairs(d.Edges))
	}
	if len(d.Tolerated) != 1 || d.Tolerated[0].A != "a" || d.Tolerated[0].B != "b" {
		t.Fatalf("expected one tolerated overlap a/b, got %v", d.Tolerated)
	}
	if !reflect.DeepEqual(d.Tolerated[0].Shared, []string{"shared.go"}) {
		t.Fatalf("shared = %v", d.Tolerated[0].Shared)
	}
}

func TestDeriveOverlapBeyondMaxSerializes(t *testing.T) {
	picks := []Pick{
		{Ref: "a", Files: []string{"x.go", "y.go"}},
		{Ref: "b", Files: []string{"x.go", "y.go"}},
	}
	d := Derive(picks, nil, 1) // 2 shared > max(1) => serialize b->a
	if len(d.Edges) != 1 || d.Edges[0].From != "b" || d.Edges[0].To != "a" {
		t.Fatalf("expected b->a serialize, got %v", edgePairs(d.Edges))
	}
}

func TestDeriveExplicitEdgeSuppressesDerived(t *testing.T) {
	// a needs b explicitly AND they share 2 files: no duplicate/contradictory edge.
	picks := []Pick{
		{Ref: "a", Needs: []string{"b"}, Files: []string{"x.go", "y.go"}},
		{Ref: "b", Files: []string{"x.go", "y.go"}},
	}
	d := Derive(picks, nil, 1)
	if len(d.Edges) != 1 || d.Edges[0].From != "a" || d.Edges[0].To != "b" {
		t.Fatalf("explicit edge must win, got %v", edgePairs(d.Edges))
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/plan/ -run TestDerive`
Expected: FAIL — `undefined: Derive`.

- [ ] **Step 3: Write the implementation**

```go
// internal/plan/overlap.go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Weft Contributors

package plan

import (
	"path/filepath"
	"sort"
)

// Edge is a dependency edge: From depends on To (From is blocked by To). This
// matches bd's edge direction (bd dep add <issue> <depends-on>).
type Edge struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// Overlap is a warn+tolerate file overlap surfaced to the human in dry-run
// (spec §4.2): the two picks share files but stay in the same shed, and any
// resulting collision becomes a first-class jj conflict (resolved via seam 4).
type Overlap struct {
	A      string   `json:"a"`
	B      string   `json:"b"`
	Shared []string `json:"shared"`
}

// Derivation is the result of edge derivation (spec §4).
type Derivation struct {
	Edges     []Edge    `json:"edges"`
	Tolerated []Overlap `json:"tolerated"`
}

// Derive computes the warp's dependency edges from explicit needs plus the
// file-overlap policy (spec §4.2). It is pure and deterministic: picks are
// processed in ref-lexicographic order and the derived-edge tiebreaker keys on
// ref (bead-ids do not exist until emit runs bd create --graph).
func Derive(picks []Pick, structural []string, overlapMax int) Derivation {
	sorted := append([]Pick{}, picks...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Ref < sorted[j].Ref })

	edges := []Edge{}
	edgeSet := map[[2]string]bool{}     // directed dedup
	pairHasEdge := map[[2]string]bool{} // undirected (sorted) — author already ordered the pair
	addEdge := func(from, to string) {
		key := [2]string{from, to}
		if edgeSet[key] {
			return
		}
		edgeSet[key] = true
		lo, hi := from, to
		if lo > hi {
			lo, hi = hi, lo
		}
		pairHasEdge[[2]string{lo, hi}] = true
		edges = append(edges, Edge{From: from, To: to})
	}

	// 1. Explicit needs always become edges (sorted for determinism).
	for _, pk := range sorted {
		needs := append([]string{}, pk.Needs...)
		sort.Strings(needs)
		for _, n := range needs {
			addEdge(pk.Ref, n)
		}
	}

	// 2. File-overlap edges (spec §4.2 advisory threshold).
	tolerated := []Overlap{}
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			lo, hi := sorted[i].Ref, sorted[j].Ref // sorted: lo < hi
			if pairHasEdge[[2]string{lo, hi}] {
				continue // author already ordered this pair
			}
			shared := intersect(sorted[i].Files, sorted[j].Files)
			if len(shared) == 0 {
				continue
			}
			if anyStructural(shared, structural) || len(shared) > overlapMax {
				addEdge(hi, lo) // later ref depends on earlier; earlier lands first
			} else {
				tolerated = append(tolerated, Overlap{A: lo, B: hi, Shared: shared})
			}
		}
	}
	return Derivation{Edges: edges, Tolerated: tolerated}
}

// intersect returns the sorted, de-duplicated intersection of two path lists.
func intersect(a, b []string) []string {
	set := map[string]bool{}
	for _, x := range a {
		set[x] = true
	}
	out := []string{}
	added := map[string]bool{}
	for _, y := range b {
		if set[y] && !added[y] {
			out = append(out, y)
			added[y] = true
		}
	}
	sort.Strings(out)
	return out
}

// anyStructural reports whether any path matches a structural glob.
func anyStructural(paths, globs []string) bool {
	for _, p := range paths {
		if isStructural(p, globs) {
			return true
		}
	}
	return false
}

// isStructural matches a path against structural globs by full path or basename
// (filepath.Match; ** is unsupported — a §8 refinement).
func isStructural(path string, globs []string) bool {
	base := filepath.Base(path)
	for _, g := range globs {
		if ok, _ := filepath.Match(g, path); ok {
			return true
		}
		if ok, _ := filepath.Match(g, base); ok {
			return true
		}
	}
	return false
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/plan/ -run TestDerive`
Expected: PASS.

- [ ] **Step 5: Commit**

Run: `jj commit -m "feat(plan): file-overlap dependency derivation + advisory policy (seam 2 §4.2)"`

---

### Task 4: `internal/plan/emit.go` — first-emit graph payload

**Files:**
- Create: `internal/plan/emit.go`
- Test: `internal/plan/emit_test.go`

- [ ] **Step 1: Write the failing test**

```go
// internal/plan/emit_test.go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Weft Contributors

package plan

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestGraphJSONShape(t *testing.T) {
	pr := 1
	wp := WarpPlan{
		Epic: Epic{Title: "E", Description: "ed", Acceptance: "done when X"},
		Picks: []Pick{
			{Ref: "p1", Title: "A", Description: "a", Labels: []string{"phase:build"}, Priority: &pr},
			{Ref: "p2", Title: "B", Description: "b", Needs: []string{"p1"}},
		},
	}
	d := Derive(wp.Picks, nil, 1)
	raw, err := GraphJSON(wp, d)
	if err != nil {
		t.Fatalf("GraphJSON: %v", err)
	}
	var gp struct {
		Nodes []map[string]any `json:"nodes"`
		Edges []map[string]any `json:"edges"`
	}
	if err := json.Unmarshal(raw, &gp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if gp.Nodes[0]["key"] != EpicKey || gp.Nodes[0]["type"] != "epic" {
		t.Errorf("epic node = %v", gp.Nodes[0])
	}
	if !strings.Contains(gp.Nodes[0]["description"].(string), "## Acceptance") {
		t.Errorf("acceptance not folded into epic description: %v", gp.Nodes[0]["description"])
	}
	var p1 map[string]any
	for _, n := range gp.Nodes {
		if n["key"] == "p1" {
			p1 = n
		}
	}
	if p1 == nil || p1["parent_key"] != EpicKey {
		t.Fatalf("p1 node bad: %v", p1)
	}
	labels := p1["labels"].([]any)
	if labels[0] != RefLabelPrefix+"p1" {
		t.Errorf("p1 first label must be %sp1, got %v", RefLabelPrefix, labels)
	}
	if p1["priority"].(float64) != 1 {
		t.Errorf("p1 priority = %v", p1["priority"])
	}
	if len(gp.Edges) != 1 || gp.Edges[0]["from_key"] != "p2" || gp.Edges[0]["to_key"] != "p1" || gp.Edges[0]["type"] != EdgeType {
		t.Errorf("edges = %v", gp.Edges)
	}
}

func TestGraphJSONDefaultsPriority(t *testing.T) {
	wp := WarpPlan{Epic: Epic{Title: "E"}, Picks: []Pick{{Ref: "p1", Title: "A", Description: "a"}}}
	raw, _ := GraphJSON(wp, Derive(wp.Picks, nil, 1))
	if !strings.Contains(string(raw), `"priority": 2`) {
		t.Errorf("expected default priority 2, got %s", raw)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/plan/ -run TestGraphJSON`
Expected: FAIL — `undefined: GraphJSON`.

- [ ] **Step 3: Write the implementation**

```go
// internal/plan/emit.go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Weft Contributors

package plan

import (
	"encoding/json"
	"sort"
	"strings"
)

// RefLabelPrefix stamps a pick's stable ref onto its bead as a label, carrying
// the ref->bead-id identity map in the warp itself (spec §3/§7). Re-plan reads
// it back to resolve refs to ids. No sidecar state; the plan file is never
// mutated post-emit.
const RefLabelPrefix = "weft-ref:"

// EdgeType is the bd dependency type weft emits for authored + derived edges.
const EdgeType = "blocks"

// DefaultPriority mirrors bd's default when a pick omits one.
const DefaultPriority = 2

// graphNode / graphEdge / graphPlan mirror the bd create --graph wire schema
// (grounded via --help/--dry-run: nodes keyed by "key", deps as top-level
// edges with from_key/to_key/type; UNKNOWN FIELDS ARE SILENTLY DROPPED — so
// these field names must match bd exactly).
type graphNode struct {
	Key         string   `json:"key"`
	Title       string   `json:"title"`
	Description string   `json:"description,omitempty"`
	Type        string   `json:"type,omitempty"`
	ParentKey   string   `json:"parent_key,omitempty"`
	Labels      []string `json:"labels,omitempty"`
	Priority    int      `json:"priority"`
}

type graphEdge struct {
	FromKey string `json:"from_key"`
	ToKey   string `json:"to_key"`
	Type    string `json:"type"`
}

type graphPlan struct {
	Nodes []graphNode `json:"nodes"`
	Edges []graphEdge `json:"edges"`
}

// GraphJSON builds the bd create --graph payload for a first emit (spec §6).
// The epic becomes an epic node; each pick a task node parented to it, carrying
// its weft-ref:<ref> identity label plus any authored labels.
func GraphJSON(p WarpPlan, d Derivation) ([]byte, error) {
	desc := p.Epic.Description
	if p.Epic.Acceptance != "" {
		// The graph node schema's acceptance field is unconfirmed (§8); fold it
		// into the description so it is never silently dropped.
		desc = strings.TrimRight(desc, "\n") + "\n\n## Acceptance\n" + p.Epic.Acceptance
	}
	nodes := []graphNode{{
		Key:         EpicKey,
		Title:       p.Epic.Title,
		Description: desc,
		Type:        "epic",
		Priority:    DefaultPriority,
	}}
	for _, pk := range sortedPicks(p.Picks) {
		nodes = append(nodes, graphNode{
			Key:         pk.Ref,
			Title:       pk.Title,
			Description: pk.Description,
			Type:        "task",
			ParentKey:   EpicKey,
			Labels:      labelsFor(pk),
			Priority:    priorityOf(pk),
		})
	}
	edges := []graphEdge{}
	for _, e := range d.Edges {
		edges = append(edges, graphEdge{FromKey: e.From, ToKey: e.To, Type: EdgeType})
	}
	return json.MarshalIndent(graphPlan{Nodes: nodes, Edges: edges}, "", "  ")
}

// labelsFor returns a pick's emitted labels: the weft-ref identity label first,
// then any authored labels (deduped, stable order).
func labelsFor(pk Pick) []string {
	out := []string{RefLabelPrefix + pk.Ref}
	seen := map[string]bool{out[0]: true}
	for _, l := range pk.Labels {
		if !seen[l] {
			out = append(out, l)
			seen[l] = true
		}
	}
	return out
}

func priorityOf(pk Pick) int {
	if pk.Priority != nil {
		return *pk.Priority
	}
	return DefaultPriority
}

func sortedPicks(picks []Pick) []Pick {
	out := append([]Pick{}, picks...)
	sort.Slice(out, func(i, j int) bool { return out[i].Ref < out[j].Ref })
	return out
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/plan/ -run TestGraphJSON`
Expected: PASS.

- [ ] **Step 5: Commit**

Run: `jj commit -m "feat(plan): bd create --graph payload builder (seam 2 §6)"`

---

### Task 5: `internal/plan/emit.go` — re-plan upsert payload + deltas

**Files:**
- Modify: `internal/plan/emit.go`
- Test: `internal/plan/emit_test.go`

- [ ] **Step 1: Write the failing test**

Append to `internal/plan/emit_test.go` (add `"bytes"` and `"reflect"` to its imports):

```go
func TestBuildReplanMatchedCreatedDeferred(t *testing.T) {
	wp := WarpPlan{
		Epic: Epic{Title: "E"},
		Picks: []Pick{
			{Ref: "a", Title: "A", Description: "a"},
			{Ref: "b", Title: "B", Description: "b", Needs: []string{"a"}}, // a matched, b new => edge deferred
		},
	}
	d := Derive(wp.Picks, nil, 1)
	existing := map[string]ExistingBead{"a": {ID: "e.1", Status: "in_progress"}}
	rp, err := BuildReplan(wp, d, "e", existing)
	if err != nil {
		t.Fatalf("BuildReplan: %v", err)
	}
	if !reflect.DeepEqual(rp.Updated, []string{"a"}) {
		t.Errorf("updated = %v", rp.Updated)
	}
	if !reflect.DeepEqual(rp.Created, []string{"b"}) {
		t.Errorf("created = %v", rp.Created)
	}
	// b->a touches new ref b (no id yet) => deferred, not in the import payload.
	if len(rp.DeferredEdges) != 1 || rp.DeferredEdges[0].From != "b" || rp.DeferredEdges[0].To != "a" {
		t.Errorf("deferred = %v", rp.DeferredEdges)
	}
	// matched record must carry id + preserved status + weft-ref label.
	if !bytes.Contains(rp.JSONL, []byte(`"id":"e.1"`)) {
		t.Errorf("expected matched id in JSONL: %s", rp.JSONL)
	}
	if !bytes.Contains(rp.JSONL, []byte(`"in_progress"`)) {
		t.Errorf("expected preserved status in JSONL: %s", rp.JSONL)
	}
	if !bytes.Contains(rp.JSONL, []byte(`weft-ref:a`)) {
		t.Errorf("expected weft-ref label in JSONL: %s", rp.JSONL)
	}
}

func TestBuildReplanMatchedEdgeBecomesDependency(t *testing.T) {
	// Both refs matched => the edge is expressed as a dependency by real id.
	wp := WarpPlan{
		Epic: Epic{Title: "E"},
		Picks: []Pick{
			{Ref: "a", Title: "A", Description: "a"},
			{Ref: "b", Title: "B", Description: "b", Needs: []string{"a"}},
		},
	}
	d := Derive(wp.Picks, nil, 1)
	existing := map[string]ExistingBead{"a": {ID: "e.1", Status: "open"}, "b": {ID: "e.2", Status: "open"}}
	rp, err := BuildReplan(wp, d, "e", existing)
	if err != nil {
		t.Fatalf("BuildReplan: %v", err)
	}
	if len(rp.DeferredEdges) != 0 {
		t.Errorf("no edges should defer when both matched: %v", rp.DeferredEdges)
	}
	if !bytes.Contains(rp.JSONL, []byte(`"depends_on_id":"e.1"`)) {
		t.Errorf("expected b->a dependency by id: %s", rp.JSONL)
	}
}

func TestBuildReplanRemovedRefs(t *testing.T) {
	wp := WarpPlan{Epic: Epic{Title: "E"}, Picks: []Pick{{Ref: "a", Title: "A", Description: "a"}}}
	d := Derive(wp.Picks, nil, 1)
	existing := map[string]ExistingBead{"a": {ID: "e.1"}, "old": {ID: "e.9"}}
	rp, _ := BuildReplan(wp, d, "e", existing)
	if !reflect.DeepEqual(rp.Removed, []string{"old"}) {
		t.Errorf("removed = %v", rp.Removed)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/plan/ -run TestBuildReplan`
Expected: FAIL — `undefined: BuildReplan` / `undefined: ExistingBead`.

- [ ] **Step 3: Add the re-plan builder to `internal/plan/emit.go`**

Add `"bytes"` to the import block, then append:

```go
// ExistingBead is one live pick in an emitted warp, keyed by its weft-ref label.
type ExistingBead struct {
	ID     string
	Status string
}

// importRecord mirrors the subset of the bd export/import schema weft writes
// per pick (spec §7): upsert keyed by id (empty id => create). Status is set
// only for matched picks, to preserve their lifecycle state across a re-plan.
type importRecord struct {
	ID           string      `json:"id,omitempty"`
	Title        string      `json:"title"`
	Description  string      `json:"description,omitempty"`
	IssueType    string      `json:"issue_type"`
	Priority     int         `json:"priority"`
	Status       string      `json:"status,omitempty"`
	Parent       string      `json:"parent,omitempty"`
	Labels       []string    `json:"labels"`
	Dependencies []importDep `json:"dependencies,omitempty"`
}

type importDep struct {
	IssueID     string `json:"issue_id"`
	DependsOnID string `json:"depends_on_id"`
	Type        string `json:"type"`
}

// Replan is the computed re-plan delta against an existing warp (spec §7).
type Replan struct {
	JSONL         []byte   // bd import payload (one record per pick, newline-delimited)
	Created       []string // refs with no existing bead (created by import, fields + parent only)
	Updated       []string // refs matched to an existing bead (fields/labels/edges updated)
	DeferredEdges []Edge   // edges touching a not-yet-created pick (wired post-import — §8)
	Removed       []string // refs present in the warp but absent from the plan (supersede is §8)
}

// BuildReplan computes the bd import upsert payload and the deltas for a re-plan
// (spec §7). refToID maps known refs to existing beads (from weft-ref labels);
// epicID parents every record. Edges are expressed as dependencies only when
// BOTH endpoints already have ids; edges touching a newly created pick are
// reported as DeferredEdges (their bead-id does not exist until import runs).
func BuildReplan(p WarpPlan, d Derivation, epicID string, refToID map[string]ExistingBead) (Replan, error) {
	rp := Replan{Created: []string{}, Updated: []string{}, DeferredEdges: []Edge{}, Removed: []string{}}

	// Group resolvable edges (both endpoints matched) by dependent ref.
	depsByRef := map[string][]importDep{}
	for _, e := range d.Edges {
		from, fok := refToID[e.From]
		to, tok := refToID[e.To]
		if fok && tok {
			depsByRef[e.From] = append(depsByRef[e.From], importDep{IssueID: from.ID, DependsOnID: to.ID, Type: EdgeType})
		} else {
			rp.DeferredEdges = append(rp.DeferredEdges, e)
		}
	}

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf) // one JSON object per line (JSONL)
	for _, pk := range sortedPicks(p.Picks) {
		bead, matched := refToID[pk.Ref]
		rec := importRecord{
			ID:           bead.ID, // "" when unmatched => create
			Title:        pk.Title,
			Description:  pk.Description,
			IssueType:    "task",
			Priority:     priorityOf(pk),
			Parent:       epicID,
			Labels:       labelsFor(pk),
			Dependencies: depsByRef[pk.Ref],
		}
		if matched {
			rec.Status = bead.Status // preserve lifecycle state (never silently reopen)
			rp.Updated = append(rp.Updated, pk.Ref)
		} else {
			rp.Created = append(rp.Created, pk.Ref)
		}
		if err := enc.Encode(rec); err != nil {
			return Replan{}, err
		}
	}
	rp.JSONL = buf.Bytes()

	inPlan := map[string]bool{}
	for _, pk := range p.Picks {
		inPlan[pk.Ref] = true
	}
	for ref := range refToID {
		if !inPlan[ref] {
			rp.Removed = append(rp.Removed, ref)
		}
	}
	sort.Strings(rp.Removed)
	return rp, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/plan/ -run TestBuildReplan`
Expected: PASS.

- [ ] **Step 5: Commit**

Run: `jj commit -m "feat(plan): bd import re-plan upsert payload + deltas (seam 2 §7)"`

---

### Task 6: `weft plan check` — validate warp-plan.json

**Files:**
- Create: `internal/cli/plan.go`
- Modify: `internal/cli/root.go` (register `plan`)
- Test: `internal/cli/plan_test.go`

- [ ] **Step 1: Write the failing test**

```go
// internal/cli/plan_test.go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Weft Contributors

package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/cli/ -run TestPlanCheck`
Expected: FAIL — `plan` command not registered.

- [ ] **Step 3: Write `plan.go` with the `check` verb**

```go
// internal/cli/plan.go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Weft Contributors

package cli

import (
	"fmt"

	"github.com/seanb4t/weft/internal/exit"
	"github.com/seanb4t/weft/internal/plan"
	"github.com/spf13/cobra"
)

func (a *App) newPlanCmd() *cobra.Command {
	p := &cobra.Command{Use: "plan", Short: "Planning -> warp emission (spec seam 2)"}
	p.AddCommand(a.newPlanCheckCmd())
	return p
}

func (a *App) newPlanCheckCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "check <file>",
		Short: "Validate warp-plan.json; validity is data (exit 0, no mutation)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			wp, err := plan.Load(args[0])
			if err != nil {
				return exit.Invocationf("%v", err)
			}
			issues := plan.Validate(wp)
			data := map[string]any{"valid": len(issues) == 0, "issues": issues}
			text := fmt.Sprintf("valid: %d pick(s), no issues", len(wp.Picks))
			if len(issues) > 0 {
				text = fmt.Sprintf("INVALID: %d issue(s)", len(issues))
				for _, is := range issues {
					if is.Ref != "" {
						text += fmt.Sprintf("\n  - [%s] %s", is.Ref, is.Message)
					} else {
						text += fmt.Sprintf("\n  - %s", is.Message)
					}
				}
			}
			return Emit(cmd, "plan.check", data, text)
		},
	}
}
```

- [ ] **Step 4: Register `plan` in `root.go`**

In `internal/cli/root.go`'s `NewRootCmd`, add after `app.newResumeCmd()`:

```go
	root.AddCommand(app.newResumeCmd())
	root.AddCommand(app.newPlanCmd())
	return root
```

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./internal/cli/ -run TestPlanCheck`
Expected: PASS.

- [ ] **Step 6: Commit**

Run: `jj commit -m "feat(cli): plan check — validate warp-plan.json (seam 2 §5)"`

---

### Task 7: `weft plan emit` — first emit (dry-run + bd create --graph)

**Files:**
- Modify: `internal/cli/plan.go`
- Test: `internal/cli/plan_test.go`

- [ ] **Step 1: Write the failing test**

Append to `internal/cli/plan_test.go` (add `"github.com/seanb4t/weft/internal/exit"` to imports):

```go
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/cli/ -run TestPlanEmit`
Expected: FAIL — `plan` has no `emit` subcommand.

- [ ] **Step 3: Add the `emit` verb + first-emit path to `plan.go`**

Add `"os"`, `"strings"`, `"github.com/seanb4t/weft/internal/run"` to `plan.go`'s imports. Register the verb in `newPlanCmd`:

```go
func (a *App) newPlanCmd() *cobra.Command {
	p := &cobra.Command{Use: "plan", Short: "Planning -> warp emission (spec seam 2)"}
	p.AddCommand(a.newPlanCheckCmd(), a.newPlanEmitCmd())
	return p
}
```

Add the verb and the first-emit path:

```go
func (a *App) newPlanEmitCmd() *cobra.Command {
	var dryRun bool
	var epic string
	c := &cobra.Command{
		Use:   "emit <file>",
		Short: "Emit the warp from warp-plan.json (derive edges, preview, create/upsert)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			wp, err := plan.Load(args[0])
			if err != nil {
				return exit.Invocationf("%v", err)
			}
			if issues := plan.Validate(wp); len(issues) > 0 {
				return exit.Invocationf("warp-plan is invalid (%d issue(s)); run 'weft plan check' first", len(issues))
			}
			d := plan.Derive(wp.Picks, a.Config.PlanStructural(), a.Config.PlanOverlapMax())
			if epic != "" {
				return a.planReplan(cmd, wp, d, epic, dryRun) // Task 8
			}
			return a.planFirstEmit(cmd, wp, d, dryRun)
		},
	}
	c.Flags().BoolVar(&dryRun, "dry-run", false, "preview the warp without mutating beads")
	c.Flags().StringVar(&epic, "epic", "", "existing epic id to re-plan against (bd import upsert)")
	return c
}

// planFirstEmit creates a brand-new warp via bd create --graph (spec §6).
func (a *App) planFirstEmit(cmd *cobra.Command, wp plan.WarpPlan, d plan.Derivation, dryRun bool) error {
	graph, err := plan.GraphJSON(wp, d)
	if err != nil {
		return exit.Hardf("build graph payload: %v", err)
	}
	if dryRun {
		data := map[string]any{
			"dry_run": true, "mode": "create", "epic": wp.Epic.Title,
			"picks": len(wp.Picks), "edges": d.Edges, "tolerated": d.Tolerated,
		}
		return Emit(cmd, "plan.emit", data, planPreviewText("create", wp, d))
	}
	// bd create --graph takes a file path (no stdin), so stage the payload.
	path, cleanup, err := writeTempPayload("weft-warp-*.json", graph)
	if err != nil {
		return err
	}
	defer cleanup()
	res, err := run.BD(a.Runner, "create", "--graph", path)
	if err != nil {
		return exit.Hardf("bd create --graph could not run: %v", err)
	}
	if res.Code != 0 {
		return exit.Hardf("bd create --graph failed: %s", strings.TrimSpace(res.Stderr))
	}
	data := map[string]any{
		"mode": "create", "created": len(wp.Picks), "edges": d.Edges,
		"tolerated": d.Tolerated, "bd_output": strings.TrimSpace(res.Stdout),
	}
	text := fmt.Sprintf("emitted warp: %d pick(s), %d edge(s), %d tolerated overlap(s)\n%s",
		len(wp.Picks), len(d.Edges), len(d.Tolerated), strings.TrimSpace(res.Stdout))
	return Emit(cmd, "plan.emit", data, text)
}

// writeTempPayload stages a payload weft must hand to bd as a file path.
func writeTempPayload(pattern string, payload []byte) (string, func(), error) {
	f, err := os.CreateTemp("", pattern)
	if err != nil {
		return "", func() {}, exit.Hardf("temp payload file: %v", err)
	}
	if _, err := f.Write(payload); err != nil {
		f.Close()
		os.Remove(f.Name())
		return "", func() {}, exit.Hardf("write payload: %v", err)
	}
	f.Close()
	return f.Name(), func() { os.Remove(f.Name()) }, nil
}

// planPreviewText renders the dry-run human gate (spec §5): edges + the
// warn+tolerate overlaps the human is approving.
func planPreviewText(mode string, wp plan.WarpPlan, d plan.Derivation) string {
	var b strings.Builder
	fmt.Fprintf(&b, "DRY RUN (%s) — epic %q, %d pick(s), %d edge(s)\n", mode, wp.Epic.Title, len(wp.Picks), len(d.Edges))
	for _, e := range d.Edges {
		fmt.Fprintf(&b, "  edge: %s depends on %s\n", e.From, e.To)
	}
	if len(d.Tolerated) > 0 {
		fmt.Fprintf(&b, "  %d tolerated overlap(s) (same shed; conflict resolved via seam 4):\n", len(d.Tolerated))
		for _, o := range d.Tolerated {
			fmt.Fprintf(&b, "    %s ~ %s share %v\n", o.A, o.B, o.Shared)
		}
	}
	b.WriteString("  (no mutation — re-run without --dry-run to emit)")
	return b.String()
}
```

> Note: `planReplan` is added in Task 8. To keep this task compiling on its own, add a temporary stub at the bottom of `plan.go` and **replace it in Task 8**:
>
> ```go
> func (a *App) planReplan(cmd *cobra.Command, wp plan.WarpPlan, d plan.Derivation, epic string, dryRun bool) error {
> 	return exit.Invocationf("re-plan (--epic) not yet implemented")
> }
> ```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/cli/ -run TestPlanEmit`
Expected: PASS.

- [ ] **Step 5: Commit**

Run: `jj commit -m "feat(cli): plan emit — first emit (dry-run + bd create --graph) (seam 2 §6)"`

---

### Task 8: `weft plan emit --epic` — re-plan upsert via bd import

**Files:**
- Modify: `internal/cli/plan.go`
- Test: `internal/cli/plan_test.go`

- [ ] **Step 1: Write the failing test**

Append to `internal/cli/plan_test.go`:

```go
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/cli/ -run TestPlanEmitReplan`
Expected: FAIL — the stub returns an invocation error (exit 1), so the tests fail.

- [ ] **Step 3: Replace the `planReplan` stub + add `warpRefMap`**

Add `"encoding/json"` to `plan.go`'s imports. Replace the Task-7 stub `planReplan` with:

```go
// planReplan upserts an existing warp via bd import (spec §7): resolve the
// ref->bead map from the epic's weft-ref labels, build the upsert payload, and
// (unless --dry-run) apply it. New-pick dependency edges and removed-pick
// supersede are surfaced as data but NOT applied (§8 sub-seams).
func (a *App) planReplan(cmd *cobra.Command, wp plan.WarpPlan, d plan.Derivation, epic string, dryRun bool) error {
	existing, err := a.warpRefMap(epic)
	if err != nil {
		return err
	}
	rp, err := plan.BuildReplan(wp, d, epic, existing)
	if err != nil {
		return exit.Hardf("build re-plan payload: %v", err)
	}
	if dryRun {
		data := map[string]any{
			"dry_run": true, "mode": "upsert", "epic": epic,
			"updated": rp.Updated, "created": rp.Created, "removed": rp.Removed,
			"deferred_edges": rp.DeferredEdges, "tolerated": d.Tolerated,
		}
		return Emit(cmd, "plan.emit", data, replanText(epic, rp, true))
	}
	path, cleanup, err := writeTempPayload("weft-replan-*.jsonl", rp.JSONL)
	if err != nil {
		return err
	}
	defer cleanup()
	res, err := run.BD(a.Runner, "import", path)
	if err != nil {
		return exit.Hardf("bd import could not run: %v", err)
	}
	if res.Code != 0 {
		return exit.Hardf("bd import failed: %s", strings.TrimSpace(res.Stderr))
	}
	data := map[string]any{
		"mode": "upsert", "epic": epic,
		"updated": rp.Updated, "created": rp.Created, "removed": rp.Removed,
		"deferred_edges": rp.DeferredEdges, "tolerated": d.Tolerated,
		"bd_output": strings.TrimSpace(res.Stdout),
	}
	return Emit(cmd, "plan.emit", data, replanText(epic, rp, false))
}

// warpRefMap reads an epic's children and rebuilds the ref->bead map from their
// weft-ref:<ref> labels (spec §3/§7) in a single bd list call.
func (a *App) warpRefMap(epic string) (map[string]plan.ExistingBead, error) {
	res, err := run.BD(a.Runner, "list", "--parent", epic, "--json")
	if err != nil {
		return nil, exit.Hardf("bd list could not run: %v", err)
	}
	if res.Code != 0 {
		return nil, exit.Hardf("bd list failed: %s", strings.TrimSpace(res.Stderr))
	}
	var arr []struct {
		ID     string   `json:"id"`
		Status string   `json:"status"`
		Labels []string `json:"labels"`
	}
	if err := json.Unmarshal([]byte(res.Stdout), &arr); err != nil {
		return nil, exit.Hardf("parse bd list json: %v", err)
	}
	m := map[string]plan.ExistingBead{}
	for _, it := range arr {
		for _, l := range it.Labels {
			if strings.HasPrefix(l, plan.RefLabelPrefix) {
				ref := strings.TrimPrefix(l, plan.RefLabelPrefix)
				m[ref] = plan.ExistingBead{ID: it.ID, Status: it.Status}
			}
		}
	}
	return m, nil
}

// replanText renders the re-plan summary, flagging the §8-deferred items.
func replanText(epic string, rp plan.Replan, dry bool) string {
	prefix := "re-planned"
	if dry {
		prefix = "DRY RUN (upsert)"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s epic %s — %d updated, %d created, %d removed\n",
		prefix, epic, len(rp.Updated), len(rp.Created), len(rp.Removed))
	if len(rp.DeferredEdges) > 0 {
		fmt.Fprintf(&b, "  %d edge(s) touch a new pick — wire after creation (§8): ", len(rp.DeferredEdges))
		for i, e := range rp.DeferredEdges {
			if i > 0 {
				b.WriteString(", ")
			}
			fmt.Fprintf(&b, "%s->%s", e.From, e.To)
		}
		b.WriteString("\n")
	}
	if len(rp.Removed) > 0 {
		fmt.Fprintf(&b, "  removed ref(s) need supersede (§8): %s\n", strings.Join(rp.Removed, ", "))
	}
	if dry {
		b.WriteString("  (no mutation — re-run without --dry-run to upsert)")
	}
	return b.String()
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/cli/ -run TestPlanEmitReplan`
Expected: PASS.

- [ ] **Step 5: Run the full suite + vet + build**

Run: `go vet ./... && go test ./... && go build ./cmd/weft`
Expected: PASS across all packages; no vet complaints; binary builds.

- [ ] **Step 6: Commit**

Run: `jj commit -m "feat(cli): plan emit --epic — re-plan upsert via bd import (seam 2 §7)"`

---

## Done criteria

- `go test ./...` passes; `go vet ./...` clean; `go build ./cmd/weft` builds.
- `weft plan check <file>` reports `{valid, issues}` on exit 0 (validity is data); a file that can't be read is an exit-1 invocation error.
- `weft plan emit <file> --dry-run` prints the derived edges **and** the warn+tolerate overlaps without mutating; refuses an invalid plan (exit 1).
- `weft plan emit <file>` builds the `bd create --graph` payload (epic node + pick nodes parented to it, each stamped with `weft-ref:<ref>` + authored labels, edges as `{from_key,to_key,type:blocks}`) and creates the warp atomically.
- `weft plan emit <file> --epic <id>` rebuilds the `ref→bead-id` map from the epic's `weft-ref` labels and upserts via `bd import` (matched refs update with preserved status + matched-ref edges; unmatched refs create); surfaces `deferred_edges` and `removed` as data.
- Derived edges are deterministic (ref-lexicographic); structural overlap or count > `plan.overlap_max` serializes, lesser incidental overlap tolerates; all list fields serialize as `[]`, never `null`.

## Out of scope (follow-on / §8 sub-seams)

- Wiring re-plan `deferred_edges` (new-pick dependencies) after import via a second `bd dep add --file` pass; superseding `removed` picks via `bd supersede` — the **re-plan reconciliation** sub-seam.
- Formal `warp-plan.json` JSON Schema; `**` glob support in `[plan].structural`; declared-vs-actual file-drift detection; `has_checkpoint` → `bd human`.
- Passing the graph/import payload via stdin instead of a temp file (would need a `run.Runner` stdin extension).
- Seam-4 conflict UX; the ship verbs (`finish`, `shed abandon/status`).

<!-- adr-capture: sha256=b58c9e016d5549d6; session=cli; ts=2026-06-03T16:07:52Z; adrs= -->
