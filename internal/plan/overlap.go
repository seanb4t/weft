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
