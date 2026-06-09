// internal/cli/overlap.go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Weft Contributors

package cli

// overlapGroups partitions changes into connected components over the
// "share at least one file" relation. Two changes that touch no common file
// cannot conflict (jj conflicts are per-file), so they need never be stacked
// together. `changes` is the deterministic (lex-sorted) change order the caller
// established; `files[ch]` is that change's touched file set. Returns groups,
// each a slice of change-ids preserving the input (lex) order, with groups
// ordered by their lexicographically-smallest member. Pure — no jj.
func overlapGroups(changes []string, files map[string][]string) [][]string {
	parent := make([]int, len(changes))
	for i := range parent {
		parent[i] = i
	}
	var find func(int) int
	find = func(i int) int {
		for parent[i] != i {
			parent[i] = parent[parent[i]] // path-halving
			i = parent[i]
		}
		return i
	}
	union := func(a, b int) {
		ra, rb := find(a), find(b)
		if ra != rb {
			parent[rb] = ra
		}
	}
	// First change index that claimed each file; union on every later collision.
	owner := map[string]int{}
	for i, ch := range changes {
		for _, f := range files[ch] {
			if j, ok := owner[f]; ok {
				union(i, j)
			} else {
				owner[f] = i
			}
		}
	}
	// Bucket indices by representative root, preserving first-appearance order.
	// Because `changes` is lex-sorted, first-appearance order == lex-smallest-member
	// order for the groups, and each bucket stays in lex order internally.
	buckets := map[int][]string{}
	order := []int{}
	for i, ch := range changes {
		r := find(i)
		if _, seen := buckets[r]; !seen {
			order = append(order, r)
		}
		buckets[r] = append(buckets[r], ch)
	}
	groups := make([][]string, 0, len(order))
	for _, r := range order {
		groups = append(groups, buckets[r])
	}
	return groups
}
