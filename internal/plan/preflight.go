// internal/plan/preflight.go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Weft Contributors

package plan

import (
	"encoding/json"
	"fmt"
	"strings"
)

// dropMarker is bd's stable phrase for a silently-dropped unknown field in a
// graph plan (grounded against bd 1.0.5; CI pins 1.1.0-rc.1, which emits the
// same graph schema_version 1). Classify on this marker, not loose English —
// mirrors the gh-api error-classification convention.
const dropMarker = "unknown field(s)"

// Preflight is the parsed result of `bd create --graph --dry-run --json`.
type Preflight struct {
	NodeCount     int
	EdgeCount     int
	SchemaVersion int
	Drops         []string // verbatim bd warning lines naming dropped fields
	Notes         []string // verbatim non-empty stderr lines that are NOT drop warnings
}

// dryRunEnvelope is the subset of bd's dry-run JSON weft reads.
type dryRunEnvelope struct {
	NodeCount     int `json:"node_count"`
	EdgeCount     int `json:"edge_count"`
	SchemaVersion int `json:"schema_version"`
}

// ParsePreflight parses a bd graph dry-run: counts + schema_version from the
// JSON stdout, dropped-field warnings from stderr (one verbatim line each).
func ParsePreflight(stdout, stderr []byte) (Preflight, error) {
	var env dryRunEnvelope
	if err := json.Unmarshal(stdout, &env); err != nil {
		return Preflight{}, fmt.Errorf("parse bd graph dry-run json: %w", err)
	}
	drops := []string{}
	notes := []string{}
	for _, line := range strings.Split(string(stderr), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.Contains(line, dropMarker) {
			drops = append(drops, line)
		} else {
			notes = append(notes, line)
		}
	}
	return Preflight{
		NodeCount:     env.NodeCount,
		EdgeCount:     env.EdgeCount,
		SchemaVersion: env.SchemaVersion,
		Drops:         drops,
		Notes:         notes,
	}, nil
}

// ExpectedGraphSchemaVersion is the bd graph schema version this build was
// grounded against. A mismatch is a soft signal to re-ground weft, not a stop.
const ExpectedGraphSchemaVersion = 1

// PreflightIssues categorizes a preflight against weft's expectations. It is
// data only; enforcement policy (what is hard vs soft) is the caller's
// (planFirstEmit): CountMismatch and Drops are hard-enforced there, SchemaNote
// is soft.
type PreflightIssues struct {
	Drops         []string // unknown-field drop warnings (verbatim)
	CountMismatch string   // "" when node/edge counts match; else a description
	SchemaNote    string   // "" when schema_version matches; else a soft note
}

// CheckPreflight compares a parsed preflight to the node/edge counts weft built.
func CheckPreflight(pf Preflight, wantNodes, wantEdges int) PreflightIssues {
	issues := PreflightIssues{Drops: pf.Drops}
	if pf.NodeCount != wantNodes || pf.EdgeCount != wantEdges {
		issues.CountMismatch = fmt.Sprintf(
			"bd parsed %d node(s)/%d edge(s); weft built %d/%d (graph shape drift)",
			pf.NodeCount, pf.EdgeCount, wantNodes, wantEdges)
	}
	if pf.SchemaVersion != ExpectedGraphSchemaVersion {
		issues.SchemaNote = fmt.Sprintf(
			"bd graph schema_version %d != expected %d — re-ground weft (proceeding)",
			pf.SchemaVersion, ExpectedGraphSchemaVersion)
	}
	return issues
}
