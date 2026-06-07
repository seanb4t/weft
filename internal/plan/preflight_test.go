// internal/plan/preflight_test.go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Weft Contributors

package plan

import "testing"

const sampleDryRunStdout = `{
  "dry_run": true, "node_count": 2, "edge_count": 1,
  "parent_deps": 1, "schema_version": 1,
  "nodes": [{"key":"@epic"}], "validation_notes": []
}`

const sampleDryRunStderr = `warning: graph plan node["@epic"] has unknown field(s): [acceptance] (silently dropped — see 'bd create --graph' schema)
warning: graph plan edge[0] has unknown field(s): [bogus] (silently dropped — see 'bd create --graph' schema)`

func TestParsePreflightCountsAndDrops(t *testing.T) {
	pf, err := ParsePreflight([]byte(sampleDryRunStdout), []byte(sampleDryRunStderr))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if pf.NodeCount != 2 || pf.EdgeCount != 1 || pf.SchemaVersion != 1 {
		t.Errorf("counts = nodes %d edges %d schema %d; want 2/1/1", pf.NodeCount, pf.EdgeCount, pf.SchemaVersion)
	}
	if len(pf.Drops) != 2 {
		t.Fatalf("Drops = %v; want 2 lines", pf.Drops)
	}
}

func TestParsePreflightCleanHasEmptyDrops(t *testing.T) {
	pf, err := ParsePreflight([]byte(sampleDryRunStdout), []byte(""))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	// Must be a non-nil empty slice (JSON null breaks --json consumers).
	if pf.Drops == nil || len(pf.Drops) != 0 {
		t.Errorf("clean stderr must yield empty (non-nil) Drops, got %#v", pf.Drops)
	}
}

func TestParsePreflightBadStdoutErrors(t *testing.T) {
	if _, err := ParsePreflight([]byte("not json"), []byte("")); err == nil {
		t.Error("unparseable stdout must return an error")
	}
}
