// internal/plan/integration_test.go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Weft Contributors

//go:build integration

package plan_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/seanb4t/weft/internal/plan"
)

// TestGraphJSONNoDropAgainstLiveBD asserts a representative GraphJSON produces
// zero unknown-field warnings from the installed bd (the drift sentinel for a bd
// graph-schema change). Run with: go test -tags integration ./internal/plan/.
func TestGraphJSONNoDropAgainstLiveBD(t *testing.T) {
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
	if err := os.WriteFile(f, b, 0o644); err != nil {
		t.Fatal(err)
	}
	out, execErr := exec.Command(bdPath, "create", "--graph", f, "--dry-run", "--json").CombinedOutput()
	if execErr != nil {
		t.Fatalf("bd exited with error: %v\n%s", execErr, out)
	}
	if strings.Contains(string(out), "unknown field(s)") {
		t.Fatalf("live bd dropped fields from GraphJSON output:\n%s", out)
	}
}
