// internal/weave/fixture_seed_test.go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Weft Contributors

//go:build integration

package weave_test

import (
	"encoding/json"
	"path/filepath"
	"runtime"
	"testing"
)

// seededFixture is the result of seeding: the epic id and a ref→bead-id map.
type seededFixture struct {
	epic  string
	byRef map[string]string // "p1" → bead-id
}

// seedFixture emits the committed fixture warp into the scratch repo's beads DB
// via `weft plan emit`, then reads the ids map directly from the emit envelope.
// The plan emit data field carries an "ids" map (node key → created bead id),
// where "@epic" is the project epic and each pick ref is its own key.
func (r *scratchRepo) seedFixture(t *testing.T) seededFixture {
	t.Helper()
	// Locate the committed warp relative to this source file (robust to cwd).
	_, thisFile, _, _ := runtime.Caller(0)
	warp := filepath.Join(filepath.Dir(thisFile), "..", "..", "testdata", "weave-fixture", "warp-plan.json")

	// Run plan emit; the envelope carries the ids map under data.ids.
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
	for _, ref := range []string{"p1", "p2a", "p2b", "p3", "p4a", "p4b"} {
		if byRef[ref] == "" {
			t.Fatalf("seedFixture: ref %q not found in ids map", ref)
		}
	}
	return seededFixture{epic: epicID, byRef: byRef}
}

// TestSeedFixture verifies that seedFixture seeds exactly six picks under one
// epic in a fresh scratch repo.
func TestSeedFixture(t *testing.T) {
	requireSubstrate(t)
	r := newScratchRepo(t)
	fx := r.seedFixture(t)
	if fx.epic == "" {
		t.Fatal("seedFixture: epic id is empty")
	}
	if len(fx.byRef) != 6 {
		t.Fatalf("seedFixture: len(byRef) = %d, want 6; byRef = %v", len(fx.byRef), fx.byRef)
	}
}
