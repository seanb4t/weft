// internal/plan/plan.go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Weft Contributors

// Package plan models warp-plan.json (spec §3) and the pure transforms that
// turn it into the bead graph: validation (§5 plan check), file-overlap
// dependency derivation (§4), and the bd create --graph / bd import payloads
// (§6/§7). The CLI verbs in internal/cli/plan.go are thin wrappers over this.
package plan

import (
	"encoding/json"
	"fmt"
	"os"
)

// Epic is the warp-plan.json epic block (spec §3): the ship unit (§6).
type Epic struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Acceptance  string `json:"acceptance,omitempty"`
}

// Pick is one authored pick (spec §3): one bead -> one jj change. Ref is the
// stable, plan-local identity key — the durable plan<->warp join (§3/§7).
type Pick struct {
	Ref         string   `json:"ref"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Needs       []string `json:"needs,omitempty"`
	Files       []string `json:"files,omitempty"`
	Priority    *int     `json:"priority,omitempty"` // pointer: distinguishes unset from 0 (P0)
	Labels      []string `json:"labels,omitempty"`
}

// WarpPlan is the whole authored artifact (spec §3).
type WarpPlan struct {
	Epic  Epic   `json:"epic"`
	Picks []Pick `json:"picks"`
}

// Issue is one validation problem (an element of plan check's output, spec §5).
type Issue struct {
	Ref     string `json:"ref,omitempty"`
	Message string `json:"message"`
}

// EpicKey is the internal bd create --graph node key for the epic. It is not a
// valid pick ref (Validate rejects a colliding ref), so it never clashes.
const EpicKey = "@epic"

// Parse decodes warp-plan.json bytes. Unknown fields are tolerated for
// forward-compatibility; Validate enforces the required shape.
func Parse(b []byte) (WarpPlan, error) {
	var p WarpPlan
	if err := json.Unmarshal(b, &p); err != nil {
		return WarpPlan{}, fmt.Errorf("parse warp-plan json: %w", err)
	}
	return p, nil
}

// Load reads and parses warp-plan.json from disk.
func Load(path string) (WarpPlan, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return WarpPlan{}, fmt.Errorf("read warp-plan %s: %w", path, err)
	}
	return Parse(b)
}

// Validate checks the plan against the spec §5 contract and returns the issues
// found (empty => valid). It is pure and never mutates. Cycle detection is left
// to bd create --graph's own validation at emit time.
func Validate(p WarpPlan) []Issue {
	issues := []Issue{}
	if p.Epic.Title == "" {
		issues = append(issues, Issue{Message: "epic.title is required"})
	}
	if len(p.Picks) == 0 {
		issues = append(issues, Issue{Message: "at least one pick is required"})
	}
	seen := map[string]bool{}
	for _, pk := range p.Picks {
		switch {
		case pk.Ref == "":
			issues = append(issues, Issue{Message: "pick.ref is required"})
			continue
		case pk.Ref == EpicKey:
			issues = append(issues, Issue{Ref: pk.Ref, Message: fmt.Sprintf("pick.ref %q is reserved", EpicKey)})
		case seen[pk.Ref]:
			issues = append(issues, Issue{Ref: pk.Ref, Message: "duplicate pick.ref"})
		}
		seen[pk.Ref] = true
		if pk.Title == "" {
			issues = append(issues, Issue{Ref: pk.Ref, Message: "pick.title is required"})
		}
		if pk.Description == "" {
			issues = append(issues, Issue{Ref: pk.Ref, Message: "pick.description is required (the bead description is the plan)"})
		}
	}
	// needs must reference a known ref and not the pick itself (seen is complete
	// after the first loop).
	for _, pk := range p.Picks {
		for _, n := range pk.Needs {
			if n == pk.Ref {
				issues = append(issues, Issue{Ref: pk.Ref, Message: "pick.needs references itself"})
				continue
			}
			if n == EpicKey {
				issues = append(issues, Issue{Ref: pk.Ref, Message: fmt.Sprintf("pick.needs references reserved ref %q", EpicKey)})
				continue
			}
			if !seen[n] {
				issues = append(issues, Issue{Ref: pk.Ref, Message: fmt.Sprintf("pick.needs references unknown ref %q", n)})
			}
		}
	}
	return issues
}
