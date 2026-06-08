// internal/plan/integration_test.go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Weft Contributors

//go:build integration

package plan_test

import (
	"bytes"
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
