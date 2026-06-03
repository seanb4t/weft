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
