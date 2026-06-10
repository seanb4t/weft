// internal/plan/emit_test.go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Weft Contributors

package plan

import (
	"bytes"
	"encoding/json"
	"reflect"
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
	if !bytes.Contains(rp.JSONL(), []byte(`"id":"e.1"`)) {
		t.Errorf("expected matched id in JSONL: %s", rp.JSONL())
	}
	if !bytes.Contains(rp.JSONL(), []byte(`"in_progress"`)) {
		t.Errorf("expected preserved status in JSONL: %s", rp.JSONL())
	}
	if !bytes.Contains(rp.JSONL(), []byte(`weft-ref:a`)) {
		t.Errorf("expected weft-ref label in JSONL: %s", rp.JSONL())
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
	if !bytes.Contains(rp.JSONL(), []byte(`"depends_on_id":"e.1"`)) {
		t.Errorf("expected b->a dependency by id: %s", rp.JSONL())
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

// TestBuildReplanMatchedStatusPreserved documents the matched-bead Status
// invariant: a matched bead's live status (always non-empty from `bd list
// --json`) must appear verbatim in the JSONL so bd import preserves it, and
// must NOT appear in the record for an unmatched (create) pick (omitempty).
func TestBuildReplanMatchedStatusPreserved(t *testing.T) {
	wp := WarpPlan{
		Epic: Epic{Title: "E"},
		Picks: []Pick{
			{Ref: "a", Title: "A", Description: "a"},
			{Ref: "b", Title: "B", Description: "b"},
		},
	}
	d := Derive(wp.Picks, nil, 1)
	existing := map[string]ExistingBead{"a": {ID: "e.1", Status: "in_progress"}}
	rp, err := BuildReplan(wp, d, "e", existing)
	if err != nil {
		t.Fatalf("BuildReplan: %v", err)
	}

	lines := bytes.Split(bytes.TrimRight(rp.JSONL(), "\n"), []byte("\n"))
	if len(lines) != 2 {
		t.Fatalf("expected 2 JSONL lines, got %d: %s", len(lines), rp.JSONL())
	}

	recFor := func(ref string) map[string]any {
		for _, line := range lines {
			var r map[string]any
			if err := json.Unmarshal(line, &r); err != nil {
				continue
			}
			for _, lbl := range r["labels"].([]any) {
				if lbl == RefLabelPrefix+ref {
					return r
				}
			}
		}
		return nil
	}

	aRec := recFor("a")
	if aRec == nil {
		t.Fatal("record for a not found")
	}
	if aRec["status"] != "in_progress" {
		t.Errorf("matched bead a: status = %v, want in_progress", aRec["status"])
	}

	bRec := recFor("b")
	if bRec == nil {
		t.Fatal("record for b not found")
	}
	if _, ok := bRec["status"]; ok {
		t.Errorf("unmatched bead b: status field should be absent (omitempty), got %v", bRec["status"])
	}
}

// TestReplanJSONLImmutable verifies that JSONL() returns a defensive copy: mutating
// the returned slice must not affect subsequent calls.
func TestReplanJSONLImmutable(t *testing.T) {
	wp := WarpPlan{
		Epic:  Epic{Title: "E"},
		Picks: []Pick{{Ref: "a", Title: "A", Description: "a"}},
	}
	d := Derive(wp.Picks, nil, 1)
	rp, err := BuildReplan(wp, d, "e", nil)
	if err != nil {
		t.Fatalf("BuildReplan: %v", err)
	}

	first := rp.JSONL()
	if len(first) == 0 {
		t.Fatal("JSONL() returned empty slice")
	}
	// Snapshot the expected bytes independently of any slice the accessor returns.
	want := append([]byte(nil), first...)

	// Mutate the first returned slice's backing array.
	first[0] = 'X'

	// A fresh call must be unaffected. With a value receiver and a shared backing
	// array (no defensive copy), this second call would alias `first` and observe
	// the mutation; the copy is what makes them independent.
	second := rp.JSONL()
	if !bytes.Equal(second, want) {
		t.Errorf("JSONL() is not a defensive copy: a later call observed a mutation of an earlier return (want %q, got %q)", want, second)
	}
}

// TestImportRecordFieldsAccountedForInReplanExpect is a structural drift guard.
// BuildReplan hand-copies importRecord fields into ReplanExpect with no
// compile-time linkage, so a new authored importRecord field can silently escape
// ReplanExpect/VerifyReplan and never be checked after import. This test
// reflects over importRecord's fields and requires each to be classified below
// as verified or waived-with-reason; adding or removing a field without updating
// the map fails the test.
func TestImportRecordFieldsAccountedForInReplanExpect(t *testing.T) {
	// classification for every importRecord field.
	//   verified=true  => the field round-trips and ReplanExpect+VerifyReplan check it.
	//   verified=false => intentionally not round-trip verified; reason says why.
	type fieldClass struct {
		verified bool
		reason   string
	}
	accounted := map[string]fieldClass{
		"ID":           {false, "bead identity / match key, not authored content; not a value to round-trip"},
		"Title":        {true, "ReplanExpect.Title; VerifyReplan title check"},
		"Description":  {true, "ReplanExpect.HasDesc; VerifyReplan description-presence check"},
		"IssueType":    {false, `hardcoded "task" for every pick, not per-pick authored`},
		"Priority":     {true, "ReplanExpect.Priority; VerifyReplan priority check"},
		"Status":       {false, "preserved from the existing bead, not authored by the plan"},
		"Parent":       {false, "always epicID (structural); enforced by the parent-scoped read-back, not a per-field check"},
		"Labels":       {true, "ReplanExpect.Labels; VerifyReplan label-subset check"},
		"Dependencies": {false, "edges, tracked via Replan.DeferredEdges and the dep round-trip, not a ReplanExpect scalar"},
	}

	rt := reflect.TypeOf(importRecord{})

	present := map[string]bool{}
	for i := 0; i < rt.NumField(); i++ {
		name := rt.Field(i).Name
		present[name] = true
		if _, ok := accounted[name]; !ok {
			t.Errorf("importRecord field %q is unaccounted for in the ReplanExpect drift guard.\n"+
				"A new importRecord field requires a decision: verify it in ReplanExpect/VerifyReplan, "+
				"or add it here with verified=false and a reason.", name)
		}
	}

	for name := range accounted {
		if !present[name] {
			t.Errorf("drift guard classifies %q but importRecord has no such field; remove the stale entry", name)
		}
	}

	// The verified=true set must equal the fields VerifyReplan actually inspects,
	// so flipping a classification without wiring the check (or vice versa) fails.
	wantVerified := map[string]bool{"Title": true, "Description": true, "Priority": true, "Labels": true}
	for name, fc := range accounted {
		if fc.verified != wantVerified[name] {
			t.Errorf("field %q verified=%v but VerifyReplan coverage says %v; keep the classification and the checker in sync",
				name, fc.verified, wantVerified[name])
		}
	}
}

// TestDeriveIsDeterministic verifies that Derive produces identical Edges
// regardless of the input pick order. A stray map iteration would break this.
func TestDeriveIsDeterministic(t *testing.T) {
	picks := []Pick{
		{Ref: "c", Title: "C", Needs: []string{"a", "b"}},
		{Ref: "b", Title: "B", Needs: []string{"a"}},
		{Ref: "a", Title: "A"},
	}
	reversed := []Pick{picks[2], picks[1], picks[0]}

	d1 := Derive(picks, nil, 1)
	d2 := Derive(reversed, nil, 1)

	if !reflect.DeepEqual(d1.Edges, d2.Edges) {
		t.Errorf("Derive not deterministic:\n  forward  = %v\n  reversed = %v", d1.Edges, d2.Edges)
	}
}

// TestGraphJSONIsDeterministic verifies that GraphJSON produces byte-identical
// output regardless of input pick order.
func TestGraphJSONIsDeterministic(t *testing.T) {
	picks := []Pick{
		{Ref: "z", Title: "Z"},
		{Ref: "m", Title: "M", Needs: []string{"a"}},
		{Ref: "a", Title: "A"},
	}
	wp := WarpPlan{Epic: Epic{Title: "E", Description: "d"}, Picks: picks}
	reversed := WarpPlan{Epic: wp.Epic, Picks: []Pick{picks[2], picks[1], picks[0]}}

	d1 := Derive(wp.Picks, nil, 1)
	d2 := Derive(reversed.Picks, nil, 1)

	b1, err := GraphJSON(wp, d1)
	if err != nil {
		t.Fatalf("GraphJSON forward: %v", err)
	}
	b2, err := GraphJSON(reversed, d2)
	if err != nil {
		t.Fatalf("GraphJSON reversed: %v", err)
	}
	if !bytes.Equal(b1, b2) {
		t.Errorf("GraphJSON not deterministic:\n  forward  = %s\n  reversed = %s", b1, b2)
	}
}

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
	// Phase has no Labels field — only the weft-ref identity label is emitted.
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
	// A pick plan must have the correct structural shape (epic node + task node)
	// when no phases are present; byte-identical determinism is covered by
	// TestGraphJSONIsDeterministic.
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

// TestBuildReplanIsDeterministic verifies that BuildReplan produces
// byte-identical JSONL regardless of input pick order.
func TestBuildReplanIsDeterministic(t *testing.T) {
	picks := []Pick{
		{Ref: "z", Title: "Z"},
		{Ref: "m", Title: "M", Needs: []string{"a"}},
		{Ref: "a", Title: "A"},
	}
	wp := WarpPlan{Epic: Epic{Title: "E"}, Picks: picks}
	reversed := WarpPlan{Epic: wp.Epic, Picks: []Pick{picks[2], picks[1], picks[0]}}

	existing := map[string]ExistingBead{
		"a": {ID: "e.1", Status: "open"},
		"m": {ID: "e.2", Status: "in_progress"},
	}

	d1 := Derive(wp.Picks, nil, 1)
	d2 := Derive(reversed.Picks, nil, 1)

	rp1, err := BuildReplan(wp, d1, "e", existing)
	if err != nil {
		t.Fatalf("BuildReplan forward: %v", err)
	}
	rp2, err := BuildReplan(reversed, d2, "e", existing)
	if err != nil {
		t.Fatalf("BuildReplan reversed: %v", err)
	}
	if !bytes.Equal(rp1.JSONL(), rp2.JSONL()) {
		t.Errorf("BuildReplan not deterministic:\n  forward  = %s\n  reversed = %s", rp1.JSONL(), rp2.JSONL())
	}
}
