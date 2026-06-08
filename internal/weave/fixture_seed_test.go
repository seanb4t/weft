// internal/weave/fixture_seed_test.go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Weft Contributors

//go:build integration

package weave_test

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// seededFixture is the result of seeding: the epic id and a ref→bead-id map.
type seededFixture struct {
	epic  string
	byRef map[string]string // "p1" → bead-id
}

// seedFixture emits the committed fixture warp into the scratch repo's beads DB
// via `weft plan emit`, then resolves the ref→bead-id map by reading the
// weft-ref:<ref> label off each child of the created epic.
//
// The plan emit data field does NOT contain the epic bead id (the create path
// emits mode/created/edges/tolerated/schema_version/warnings/bd_output only).
// We recover the epic id by querying bd for the single epic that now exists.
func (r *scratchRepo) seedFixture(t *testing.T) seededFixture {
	t.Helper()
	// Locate the committed warp relative to this source file (robust to cwd).
	_, thisFile, _, _ := runtime.Caller(0)
	warp := filepath.Join(filepath.Dir(thisFile), "..", "..", "testdata", "weave-fixture", "warp-plan.json")

	// Run plan emit for its side effect of creating the epic + children in the
	// scratch BD; runWeft fatals internally on ok=false.
	r.runWeft(t, "", "plan", "emit", warp)

	// Recover the epic id: exactly one epic exists in the freshly-seeded DB.
	epicID := r.onlyEpicID(t)

	// Map ref → bead-id via the weft-ref label on each child.
	byRef := map[string]string{}
	for _, child := range r.childBeads(t, epicID) {
		for _, lbl := range child.Labels {
			if strings.HasPrefix(lbl, "weft-ref:") {
				byRef[strings.TrimPrefix(lbl, "weft-ref:")] = child.ID
			}
		}
	}
	for _, ref := range []string{"p1", "p2a", "p2b", "p3", "p4a", "p4b"} {
		if byRef[ref] == "" {
			t.Fatalf("seedFixture: ref %q not found among epic children", ref)
		}
	}
	return seededFixture{epic: epicID, byRef: byRef}
}

// onlyEpicID queries the scratch BD for epics and returns the single epic's id.
// Fatals if there is not exactly one.
// bd list --json outputs a JSON array directly (not wrapped in an envelope object),
// so we parse stdout only rather than using lastJSONLine (which finds objects).
// Stderr is captured separately so any bd chatter cannot corrupt the JSON parse.
func (r *scratchRepo) onlyEpicID(t *testing.T) string {
	t.Helper()
	cmd := execBD(r, "list", "--type", "epic", "--json")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("bd list --type epic --json: %v\nstderr:\n%s", err, stderr.String())
	}
	raw := strings.TrimSpace(stdout.String())
	var epics []struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal([]byte(raw), &epics); err != nil {
		t.Fatalf("parse bd list --type epic json: %v\nstdout: %s\nstderr: %s", err, stdout.String(), stderr.String())
	}
	if len(epics) != 1 {
		t.Fatalf("want exactly 1 epic, got %d: %s", len(epics), stdout.String())
	}
	return epics[0].ID
}

type childBead struct {
	ID     string   `json:"id"`
	Labels []string `json:"labels"`
}

// childBeads returns the epic's children via `bd list --parent <epic> --json`.
// bd list --json outputs a JSON array directly; stdout is parsed separately so
// any bd stderr chatter cannot corrupt the JSON parse.
func (r *scratchRepo) childBeads(t *testing.T, epic string) []childBead {
	t.Helper()
	cmd := execBD(r, "list", "--parent", epic, "--json")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("bd list --parent: %v\nstderr:\n%s", err, stderr.String())
	}
	raw := strings.TrimSpace(stdout.String())
	var beads []childBead
	if err := json.Unmarshal([]byte(raw), &beads); err != nil {
		t.Fatalf("parse bd list json: %v\nstdout: %s\nstderr: %s", err, stdout.String(), stderr.String())
	}
	return beads
}

// TestSeedFixture verifies that seedFixture seeds exactly six picks under one
// epic in a fresh scratch repo.
func TestSeedFixture(t *testing.T) {
	r := newScratchRepo(t)
	fx := r.seedFixture(t)
	if fx.epic == "" {
		t.Fatal("seedFixture: epic id is empty")
	}
	if len(fx.byRef) != 6 {
		t.Fatalf("seedFixture: len(byRef) = %d, want 6; byRef = %v", len(fx.byRef), fx.byRef)
	}
}
