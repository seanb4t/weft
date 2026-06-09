// internal/cli/overlap_test.go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Weft Contributors

package cli

import (
	"reflect"
	"testing"
)

func TestOverlapGroups(t *testing.T) {
	cases := []struct {
		name    string
		changes []string
		files   map[string][]string
		want    [][]string
	}{
		{"empty", nil, map[string][]string{}, [][]string{}},
		{"singleton", []string{"a"}, map[string][]string{"a": {"f"}}, [][]string{{"a"}}},
		{
			"all-independent",
			[]string{"a", "b", "c"},
			map[string][]string{"a": {"x"}, "b": {"y"}, "c": {"z"}},
			[][]string{{"a"}, {"b"}, {"c"}},
		},
		{
			"one-shared-pair",
			[]string{"a", "b"},
			map[string][]string{"a": {"x"}, "b": {"x"}},
			[][]string{{"a", "b"}},
		},
		{
			"two-disjoint-pairs",
			[]string{"a", "b", "c", "d"},
			map[string][]string{"a": {"p"}, "b": {"p"}, "c": {"q"}, "d": {"q"}},
			[][]string{{"a", "b"}, {"c", "d"}},
		},
		{
			"transitive-chain",
			[]string{"a", "b", "c"},
			map[string][]string{"a": {"x"}, "b": {"x", "y"}, "c": {"y"}},
			[][]string{{"a", "b", "c"}},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := overlapGroups(tc.changes, tc.files)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("overlapGroups = %v, want %v", got, tc.want)
			}
		})
	}
}
