// internal/weave/conflict_proof_test.go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Weft Contributors

//go:build integration

package weave_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// TestDeterministicConflictAtIntegrate is the §7 de-risking spike: two picks
// that add the SAME new path with different content must produce a first-class
// jj conflict at `weft shed integrate`, surfaced in data.conflicts.
func TestDeterministicConflictAtIntegrate(t *testing.T) {
	requireSubstrate(t)
	r := newScratchRepo(t)

	// Two sibling beads under one epic, no deps → both ready in one wave.
	epic := r.mustCreateEpic(t, "conflict-proof")
	a := r.mustCreateChild(t, epic, "Pick A", "ca")
	b := r.mustCreateChild(t, epic, "Pick B", "cb")

	// Form the wave; expect both beads.
	form := r.runWeft(t, "", "shed", "form", "--epic", epic)
	wave := dataStringSlice(t, form.Data, "wave")
	if len(wave) != 2 {
		t.Fatalf("wave = %v, want 2 picks", wave)
	}

	// Isolate, then for each pick write the SAME path with DIFFERENT content
	// in its own workspace and seal.
	r.runWeft(t, "", "shed", "isolate", a, b)
	r.sealWith(t, a, map[string]string{"collide.txt": "content-from-A\n"})
	r.sealWith(t, b, map[string]string{"collide.txt": "content-from-B\n"})

	// Integrate the wave; a conflict must surface in data.conflicts.
	integ := r.runWeft(t, "", "shed", "integrate", a, b)
	var d struct {
		Conflicts []struct {
			Bead   string `json:"bead"`
			Change string `json:"change"`
		} `json:"conflicts"`
	}
	if err := json.Unmarshal(integ.Data, &d); err != nil {
		t.Fatalf("parse integrate data: %v", err)
	}
	if len(d.Conflicts) == 0 {
		t.Fatalf("expected a conflict in data.conflicts, got none: %s", integ.Data)
	}
}

// --- helpers used here and reused by later tasks ---

// mustCreateEpic creates an open epic and returns its bead-id.
func (r *scratchRepo) mustCreateEpic(t *testing.T, title string) string {
	t.Helper()
	return r.bdCreateID(t, "--type", "epic", "--title", title, "--description", "d", "--priority", "2")
}

// mustCreateChild creates an open task child of epic, stamped with a
// weft-ref:<ref> label so the harness can identify it later.
func (r *scratchRepo) mustCreateChild(t *testing.T, epic, title, ref string) string {
	t.Helper()
	id := r.bdCreateID(t, "--type", "task", "--title", title, "--description", "d",
		"--priority", "2", "--labels", "weft-ref:"+ref)
	r.mustBD(t, "update", id, "--parent", epic)
	return id
}

// bdCreateID runs `bd create ... --json` and returns the new bead id.
// Uses separate stdout/stderr buffers (matching onlyEpicID/childBeads) so that
// any bd warnings or chatter on stderr cannot corrupt the JSON parse.
// Routes stdout through lastJSONLine because bd --json output may be
// multi-line pretty-printed JSON.
func (r *scratchRepo) bdCreateID(t *testing.T, args ...string) string {
	t.Helper()
	cmd := execBD(r, append([]string{"create", "--json"}, args...)...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("bd create: %v\nstderr: %s", err, stderr.String())
	}
	var res struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal([]byte(lastJSONLine(stdout.Bytes())), &res); err != nil {
		t.Fatalf("parse bd create json: %v\nstdout: %s\nstderr: %s", err, stdout.String(), stderr.String())
	}
	if res.ID == "" {
		t.Fatalf("bd create returned empty id:\nstdout: %s\nstderr: %s", stdout.String(), stderr.String())
	}
	return res.ID
}

// sealWith writes files into bead's workspace then seals the pick there.
func (r *scratchRepo) sealWith(t *testing.T, bead string, files map[string]string) {
	t.Helper()
	ws := r.workspacePath(t, bead)
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(ws, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	r.runWeft(t, ws, "pick", "seal", bead)
}
