// internal/weave/roadmap_integration_test.go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Weft Contributors

//go:build integration

package weave_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// bdJSON runs bd against the scratch repo and returns stdout (for assertions
// the harness's mustBD, which discards output, cannot make). On failure it
// includes bd's stderr in the fatal message so the error is diagnosable.
func bdJSON(t *testing.T, r *scratchRepo, args ...string) []byte {
	t.Helper()
	var stdout, stderr strings.Builder
	cmd := exec.Command("bd", args...)
	cmd.Dir = r.root
	cmd.Env = append(os.Environ(), "BEADS_DIR="+r.beadsDir)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("bd %s: %v\nstderr: %s", strings.Join(args, " "), err, stderr.String())
	}
	return []byte(stdout.String())
}

func readyIDs(t *testing.T, r *scratchRepo) map[string]bool {
	t.Helper()
	var arr []struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(bdJSON(t, r, "ready", "--json"), &arr); err != nil {
		t.Fatalf("parse bd ready: %v", err)
	}
	m := map[string]bool{}
	for _, it := range arr {
		m[it.ID] = true
	}
	return m
}

// TestRoadmapEmitAndTransitiveGating pins bd's transitive epic gating behavior:
// a pick under a blocked phase epic must not be ready until the blocker closes.
// This is the contract the phased-emission model depends on (see ADR weft-4hq).
// If bd regresses on transitive gating, this test fails, not a live weft run.
func TestRoadmapEmitAndTransitiveGating(t *testing.T) {
	requireSubstrate(t)
	r := newScratchRepo(t)

	// 1. Roadmap emit: project epic -> two phase sub-epics, p2 blocked by p1.
	warp := filepath.Join(r.root, "roadmap.json")
	if err := os.WriteFile(warp, []byte(`{"epic":{"title":"Proj","description":"d"},"phases":[`+
		`{"ref":"p1","title":"Phase 1","description":"first"},`+
		`{"ref":"p2","title":"Phase 2","description":"second","needs":["p1"]}]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	env := r.runWeft(t, "", "plan", "emit", warp)
	var data struct {
		IDs map[string]string `json:"ids"`
	}
	if err := json.Unmarshal(env.Data, &data); err != nil {
		t.Fatalf("parse emit data: %v", err)
	}
	for _, k := range []string{"@epic", "p1", "p2"} {
		if data.IDs[k] == "" {
			t.Fatalf("ids map missing %q: %v", k, data.IDs)
		}
	}

	// 2. Gating pin: a pick created under the BLOCKED p2 epic must not be
	// ready (bd's transitive epic gating — the behavior the phased model
	// depends on; if bd regresses, this test fails, not a live weave).
	r.mustBD(t, "create", "--title", "early pick", "--type", "task", "--parent", data.IDs["p2"])
	var children []struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(bdJSON(t, r, "list", "--parent", data.IDs["p2"], "--json"), &children); err != nil || len(children) != 1 {
		t.Fatalf("expected exactly the early pick under p2: %v err=%v", children, err)
	}
	early := children[0].ID
	if readyIDs(t, r)[early] {
		t.Fatalf("pick %s under blocked phase epic must NOT be ready", early)
	}

	// 3. Close phase 1 -> the early pick (and p2) become ready.
	r.mustBD(t, "close", data.IDs["p1"], "--reason=phase 1 shipped")
	if !readyIDs(t, r)[early] {
		t.Fatalf("pick %s must be ready once the blocking phase closes", early)
	}
}

// TestPerPhaseReplanAppliesNewPickEdges verifies that a JIT per-phase plan emit
// wires the b->a dependency edge post-import via bd dep add (applied_edges).
// This pins the §8 applied-edges contract that ADR weft-4hq depends on.
func TestPerPhaseReplanAppliesNewPickEdges(t *testing.T) {
	requireSubstrate(t)
	r := newScratchRepo(t)

	// Roadmap with one phase, then JIT-plan two picks (b needs a) into it.
	warp := filepath.Join(r.root, "roadmap.json")
	if err := os.WriteFile(warp, []byte(`{"epic":{"title":"Proj","description":"d"},"phases":[`+
		`{"ref":"p1","title":"Phase 1","description":"first"}]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	env := r.runWeft(t, "", "plan", "emit", warp)
	var data struct {
		IDs map[string]string `json:"ids"`
	}
	if err := json.Unmarshal(env.Data, &data); err != nil {
		t.Fatalf("parse emit data: %v", err)
	}
	phase := data.IDs["p1"]

	picks := filepath.Join(r.root, "phase1-picks.json")
	if err := os.WriteFile(picks, []byte(`{"epic":{"title":"Phase 1","description":"d"},"picks":[`+
		`{"ref":"a","title":"A","description":"a"},`+
		`{"ref":"b","title":"B","description":"b","needs":["a"]}]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	env = r.runWeft(t, "", "plan", "emit", picks, "--epic", phase)
	var rep struct {
		Applied []any `json:"applied_edges"`
	}
	if err := json.Unmarshal(env.Data, &rep); err != nil {
		t.Fatalf("parse replan data: %v", err)
	}
	if len(rep.Applied) != 1 {
		t.Fatalf("want 1 applied edge, got %v", rep.Applied)
	}

	// The b->a edge is live iff bd ready scoped to the phase shows ONLY a.
	ready := readyIDs(t, r)
	var children []struct {
		ID     string   `json:"id"`
		Labels []string `json:"labels"`
	}
	if err := json.Unmarshal(bdJSON(t, r, "list", "--parent", phase, "--json"), &children); err != nil {
		t.Fatal(err)
	}
	byRef := map[string]string{}
	for _, c := range children {
		for _, l := range c.Labels {
			if strings.HasPrefix(l, "weft-ref:") {
				byRef[strings.TrimPrefix(l, "weft-ref:")] = c.ID
			}
		}
	}
	if !ready[byRef["a"]] {
		t.Fatalf("pick a must be ready")
	}
	if ready[byRef["b"]] {
		t.Fatalf("pick b must be blocked by the applied b->a edge")
	}
}
