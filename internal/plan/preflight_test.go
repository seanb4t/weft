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

func TestParsePreflightCleanHasEmptyDropsAndNotes(t *testing.T) {
	pf, err := ParsePreflight([]byte(sampleDryRunStdout), []byte(""))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	// Must be non-nil empty slices (JSON null breaks --json consumers).
	if pf.Drops == nil || len(pf.Drops) != 0 {
		t.Errorf("clean stderr must yield empty (non-nil) Drops, got %#v", pf.Drops)
	}
	if pf.Notes == nil || len(pf.Notes) != 0 {
		t.Errorf("clean stderr must yield empty (non-nil) Notes, got %#v", pf.Notes)
	}
}

func TestParsePreflightMixedStderrSeparatesDropsAndNotes(t *testing.T) {
	mixed := `warning: graph plan node["@epic"] has unknown field(s): [acceptance] (silently dropped — see 'bd create --graph' schema)
warning: bd deprecation notice`
	pf, err := ParsePreflight([]byte(sampleDryRunStdout), []byte(mixed))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(pf.Drops) != 1 {
		t.Errorf("Drops = %v; want 1 drop line", pf.Drops)
	}
	if len(pf.Notes) != 1 {
		t.Errorf("Notes = %v; want 1 note line", pf.Notes)
	}
}

func TestParsePreflightBadStdoutErrors(t *testing.T) {
	if _, err := ParsePreflight([]byte("not json"), []byte("")); err == nil {
		t.Error("unparseable stdout must return an error")
	}
}

func TestCheckPreflightCleanIsAllZero(t *testing.T) {
	pf := Preflight{NodeCount: 3, EdgeCount: 2, SchemaVersion: ExpectedGraphSchemaVersion, Drops: []string{}}
	got := CheckPreflight(pf, 3, 2)
	if len(got.Drops) != 0 || got.CountMismatch != "" || got.SchemaNote != "" {
		t.Errorf("clean preflight must yield no issues, got %#v", got)
	}
}

func TestCheckPreflightCountMismatch(t *testing.T) {
	pf := Preflight{NodeCount: 2, EdgeCount: 2, SchemaVersion: ExpectedGraphSchemaVersion, Drops: []string{}}
	got := CheckPreflight(pf, 3, 2) // want 3 nodes, bd saw 2
	if got.CountMismatch == "" {
		t.Error("node-count mismatch must be reported")
	}
}

func TestCheckPreflightSchemaMismatchIsSoft(t *testing.T) {
	pf := Preflight{NodeCount: 3, EdgeCount: 2, SchemaVersion: ExpectedGraphSchemaVersion + 99, Drops: []string{}}
	got := CheckPreflight(pf, 3, 2)
	if got.SchemaNote == "" {
		t.Error("schema_version mismatch must produce a soft note")
	}
	if got.CountMismatch != "" {
		t.Error("schema mismatch must NOT be a count error")
	}
}

func TestCheckPreflightDropsPassThrough(t *testing.T) {
	pf := Preflight{NodeCount: 3, EdgeCount: 2, SchemaVersion: ExpectedGraphSchemaVersion, Drops: []string{"warning: … unknown field(s): [x]"}}
	got := CheckPreflight(pf, 3, 2)
	if len(got.Drops) != 1 {
		t.Errorf("drops must pass through for the caller's --allow-drop policy, got %#v", got.Drops)
	}
}
