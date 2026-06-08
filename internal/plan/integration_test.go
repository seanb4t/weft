// internal/plan/integration_test.go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Weft Contributors

//go:build integration

package plan_test

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/seanb4t/weft/internal/plan"
)

// TestGraphJSONNoDrop asserts a representative GraphJSON produces zero
// unknown-field warnings from the installed bd (the drift sentinel for a bd
// graph-schema change). Run with: go test -tags integration ./internal/plan/.
func TestGraphJSONNoDrop(t *testing.T) {
	wp := plan.WarpPlan{
		Epic:  plan.Epic{Title: "E", Description: "d", Acceptance: "AC"},
		Picks: []plan.Pick{{Ref: "a", Title: "A", Description: "a", Labels: []string{"phase:impl"}}},
	}
	b, err := plan.GraphJSON(wp, plan.Derive(wp.Picks, nil, 1))
	if err != nil {
		t.Fatalf("graph: %v", err)
	}
	bdPath, err := exec.LookPath("bd")
	if err != nil {
		t.Skip("bd not in PATH — skipping live-bd integration test")
	}
	dir := t.TempDir()
	f := filepath.Join(dir, "g.json")
	// 0o600 matches writeTempPayload in internal/cli/plan.go: the payload
	// carries pick titles/descriptions and the system temp dir is
	// world-readable, so owner-only perms are correct.
	if err := os.WriteFile(f, b, 0o600); err != nil {
		t.Fatal(err)
	}
	// Capture stdout and stderr separately so the drop check matches ONLY
	// stderr (a future bd stdout mention of "unknown field(s)" must not
	// false-trigger). On nonzero exit, report both streams.
	cmd := exec.Command(bdPath, "create", "--graph", f, "--dry-run", "--json")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("bd exited with error: %v\nstdout: %s\nstderr: %s", err, stdout.String(), stderr.String())
	}
	if strings.Contains(stderr.String(), "unknown field(s)") {
		t.Fatalf("live bd dropped fields from GraphJSON output:\n%s", stderr.String())
	}
}

// TestReplanJSONLNoDrop is the drift sentinel for the bd IMPORT payload schema —
// the sibling of TestGraphJSONNoDrop, which only guards the bd create --graph
// path. BuildReplan emits the replan upsert JSONL (importRecord) that planReplan
// feeds to `bd import`; a bd schema change on the import endpoint would silently
// drop authored fields and surface only at replan runtime. Crucially, bd import
// (unlike create --graph) emits NO "unknown field(s)" warning, so a stderr scan
// cannot detect a drop. This guard therefore imports into a throwaway DB and
// reads the fields back through VerifyReplan — the exact check planReplan runs
// live. Run with: go test -tags integration ./internal/plan/.
func TestReplanJSONLNoDrop(t *testing.T) {
	pr1, pr2 := 1, 2
	wp := plan.WarpPlan{
		Epic: plan.Epic{Title: "E", Description: "d", Acceptance: "AC"},
		Picks: []plan.Pick{
			{Ref: "a", Title: "Pick A", Description: "desc a", Labels: []string{"phase:impl"}, Priority: &pr1},
			{Ref: "b", Title: "Pick B", Labels: []string{"phase:test"}, Priority: &pr2},
		},
	}
	// Create path (refToID nil): every pick is a new bead, so created == len(picks).
	rp, err := plan.BuildReplan(wp, plan.Derive(wp.Picks, nil, 1), "e", nil)
	if err != nil {
		t.Fatalf("BuildReplan: %v", err)
	}

	bdPath, err := exec.LookPath("bd")
	if err != nil {
		t.Skip("bd not in PATH — skipping live-bd integration test")
	}

	// Hermetic bd DB: the real import below must never touch the CI job's shared
	// workspace, so pin BEADS_DIR (overriding any ambient value) to a temp dir.
	dir := t.TempDir()
	env := append(os.Environ(), "BEADS_DIR="+filepath.Join(dir, ".beads"))
	runBD := func(args ...string) (string, string, error) {
		cmd := exec.Command(bdPath, args...)
		cmd.Dir = dir
		cmd.Env = env
		var stdout, stderr bytes.Buffer
		cmd.Stdout, cmd.Stderr = &stdout, &stderr
		runErr := cmd.Run()
		return stdout.String(), stderr.String(), runErr
	}

	if _, stderr, err := runBD("init", "--non-interactive", "-p", "weftimp"); err != nil {
		t.Fatalf("bd init: %v\nstderr: %s", err, stderr)
	}

	payload := filepath.Join(dir, "replan.jsonl")
	// 0o600 matches writeTempPayload in internal/cli/plan.go: the payload carries
	// pick titles/descriptions and the temp dir is world-readable.
	if err := os.WriteFile(payload, rp.JSONL(), 0o600); err != nil {
		t.Fatal(err)
	}

	// Leg 1 (structural): bd accepts the payload and recognises every record. A
	// schema change that rejects or skips records moves these counts.
	stdout, stderr, err := runBD("import", "--dry-run", "--json", payload)
	if err != nil {
		t.Fatalf("bd import --dry-run failed: %v\nstdout: %s\nstderr: %s", err, stdout, stderr)
	}
	var summary struct {
		Created int `json:"created"`
		Skipped int `json:"skipped"`
	}
	if err := json.Unmarshal([]byte(stdout), &summary); err != nil {
		t.Fatalf("parse bd import --dry-run --json: %v\nstdout: %s", err, stdout)
	}
	if summary.Created != len(wp.Picks) || summary.Skipped != 0 {
		t.Fatalf("bd import dry-run drift: created=%d skipped=%d, want created=%d skipped=0\nstdout: %s",
			summary.Created, summary.Skipped, len(wp.Picks), stdout)
	}

	// Leg 2 (field-level): import for real, then read the fields back through the
	// same VerifyReplan the production replan runs. Keyed by the weft-ref label
	// (labelsFor always injects it), so parent linkage — which bd import models as
	// a parent-child dependency, not a payload field — is irrelevant here. Any
	// dropped authored field surfaces as a discrepancy.
	if _, stderr, err := runBD("import", payload); err != nil {
		t.Fatalf("bd import failed: %v\nstderr: %s", err, stderr)
	}
	listOut, stderr, err := runBD("list", "--json")
	if err != nil {
		t.Fatalf("bd list --json failed: %v\nstderr: %s", err, stderr)
	}
	var issues []struct {
		Title       string   `json:"title"`
		Priority    int      `json:"priority"`
		Labels      []string `json:"labels"`
		Description string   `json:"description"`
	}
	if err := json.Unmarshal([]byte(listOut), &issues); err != nil {
		t.Fatalf("parse bd list --json: %v\noutput: %s", err, listOut)
	}
	readback := map[string]plan.ReadbackBead{}
	for _, is := range issues {
		for _, l := range is.Labels {
			if ref, ok := strings.CutPrefix(l, "weft-ref:"); ok {
				readback[ref] = plan.ReadbackBead{
					Title:       is.Title,
					Priority:    is.Priority,
					Labels:      is.Labels,
					Description: is.Description,
				}
			}
		}
	}
	if disc := plan.VerifyReplan(rp.Expect, readback); len(disc) > 0 {
		t.Fatalf("live bd dropped authored field(s) from the import payload:\n%s", strings.Join(disc, "\n"))
	}
}
