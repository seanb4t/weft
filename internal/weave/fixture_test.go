// internal/weave/fixture_test.go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Weft Contributors

package weave_test

import (
	"sort"
	"testing"

	"github.com/seanb4t/weft/internal/plan"
)

// TestFixtureWarpParses asserts the committed fixture warp loads and declares
// exactly the six refs the weave E2E harness drives. A non-integration guard
// so a broken fixture is caught by the default test run.
func TestFixtureWarpParses(t *testing.T) {
	wp, err := plan.Load("../../testdata/weave-fixture/warp-plan.json")
	if err != nil {
		t.Fatalf("load fixture warp: %v", err)
	}
	got := make([]string, 0, len(wp.Picks))
	for _, p := range wp.Picks {
		got = append(got, p.Ref)
	}
	sort.Strings(got)
	want := []string{"p1", "p2a", "p2b", "p3", "p4a", "p4b"}
	if len(got) != len(want) {
		t.Fatalf("refs = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("refs = %v, want %v", got, want)
		}
	}
	if wp.Epic.Title == "" {
		t.Fatal("fixture epic has no title")
	}
}
