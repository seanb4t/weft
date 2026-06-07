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
// graph plan (grounded against bd 1.0.5). Classify on this marker, not loose
// English — mirrors the gh-api error-classification convention.
const dropMarker = "unknown field(s)"

// Preflight is the parsed result of `bd create --graph --dry-run --json`.
type Preflight struct {
	NodeCount     int
	EdgeCount     int
	SchemaVersion int
	Drops         []string // verbatim bd warning lines naming dropped fields
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
	for _, line := range strings.Split(string(stderr), "\n") {
		line = strings.TrimSpace(line)
		if line != "" && strings.Contains(line, dropMarker) {
			drops = append(drops, line)
		}
	}
	return Preflight{
		NodeCount:     env.NodeCount,
		EdgeCount:     env.EdgeCount,
		SchemaVersion: env.SchemaVersion,
		Drops:         drops,
	}, nil
}
