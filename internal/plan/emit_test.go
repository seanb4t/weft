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
