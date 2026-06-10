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
	"regexp"
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

// Phase is one roadmap phase (phased-emission spec §1): emitted as a phase
// sub-epic under the project epic, carrying its weft-ref:<ref> identity label.
type Phase struct {
	Ref         string   `json:"ref"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Acceptance  string   `json:"acceptance,omitempty"`
	Needs       []string `json:"needs,omitempty"`
}

// WarpPlan is the whole authored artifact (spec §3).
type WarpPlan struct {
	Epic   Epic    `json:"epic"`
	Picks  []Pick  `json:"picks"`
	Phases []Phase `json:"phases,omitempty"`
}

// Issue is one validation problem (an element of plan check's output, spec §5).
type Issue struct {
	Ref     string `json:"ref,omitempty"`
	Message string `json:"message"`
}

// EpicKey is the internal bd create --graph node key for the epic. It is not a
// valid pick ref (Validate rejects a colliding ref), so it never clashes.
const EpicKey = "@epic"

// refPattern enforces the §3 character-set contract: refs are stamped verbatim
// into weft-ref:<ref> labels and used as bd create --graph node keys, so colons
// (the label namespace separator), commas, whitespace, and control characters
// are disallowed to prevent ambiguous label round-trips.
var refPattern = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)

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
	if len(p.Phases) > 0 && len(p.Picks) > 0 {
		return append(issues, Issue{Message: "a plan carries phases or picks, not both"})
	}
	if len(p.Phases) > 0 {
		return append(issues, validatePhases(p.Phases)...)
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
			continue
		case seen[pk.Ref]:
			issues = append(issues, Issue{Ref: pk.Ref, Message: "duplicate pick.ref"})
		}
		seen[pk.Ref] = true
		if !refPattern.MatchString(pk.Ref) {
			issues = append(issues, Issue{Ref: pk.Ref, Message: fmt.Sprintf("pick.ref %q contains invalid characters (allowed: a-z A-Z 0-9 . _ -)", pk.Ref)})
		}
		if pk.Title == "" {
			issues = append(issues, Issue{Ref: pk.Ref, Message: "pick.title is required"})
		}
		if pk.Description == "" {
			issues = append(issues, Issue{Ref: pk.Ref, Message: "pick.description is required (the bead description is the plan)"})
		}
		if pk.Priority != nil && (*pk.Priority < 0 || *pk.Priority > 4) {
			issues = append(issues, Issue{Ref: pk.Ref, Message: "pick.priority must be between 0 and 4"})
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

// validatePhases mirrors the pick rules for the roadmap shape (phased-emission spec §1).
// Intentional delta: phases carry no priority or files fields, so the
// corresponding pick checks are absent here by design.
func validatePhases(phases []Phase) []Issue {
	issues := []Issue{}
	seen := map[string]bool{}
	for _, ph := range phases {
		switch {
		case ph.Ref == "":
			issues = append(issues, Issue{Message: "phase.ref is required"})
			continue
		case ph.Ref == EpicKey:
			issues = append(issues, Issue{Ref: ph.Ref, Message: fmt.Sprintf("phase.ref %q is reserved", EpicKey)})
			continue
		case seen[ph.Ref]:
			issues = append(issues, Issue{Ref: ph.Ref, Message: "duplicate phase.ref"})
		}
		seen[ph.Ref] = true
		if !refPattern.MatchString(ph.Ref) {
			issues = append(issues, Issue{Ref: ph.Ref, Message: fmt.Sprintf("phase.ref %q contains invalid characters (allowed: a-z A-Z 0-9 . _ -)", ph.Ref)})
		}
		if ph.Title == "" {
			issues = append(issues, Issue{Ref: ph.Ref, Message: "phase.title is required"})
		}
		if ph.Description == "" {
			issues = append(issues, Issue{Ref: ph.Ref, Message: "phase.description is required"})
		}
	}
	for _, ph := range phases {
		for _, n := range ph.Needs {
			switch {
			case n == ph.Ref:
				issues = append(issues, Issue{Ref: ph.Ref, Message: "phase.needs references itself"})
			case n == EpicKey:
				issues = append(issues, Issue{Ref: ph.Ref, Message: fmt.Sprintf("phase.needs references reserved ref %q", EpicKey)})
			case !seen[n]:
				issues = append(issues, Issue{Ref: ph.Ref, Message: fmt.Sprintf("phase.needs references unknown ref %q", n)})
			}
		}
	}
	return issues
}
