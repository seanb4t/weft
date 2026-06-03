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
