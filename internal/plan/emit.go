// internal/plan/emit.go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Weft Contributors

package plan

import (
	"encoding/json"
	"sort"
	"strings"
)

// RefLabelPrefix stamps a pick's stable ref onto its bead as a label, carrying
// the ref->bead-id identity map in the warp itself (spec §3/§7). Re-plan reads
// it back to resolve refs to ids. No sidecar state; the plan file is never
// mutated post-emit.
const RefLabelPrefix = "weft-ref:"

// EdgeType is the bd dependency type weft emits for authored + derived edges.
const EdgeType = "blocks"

// DefaultPriority mirrors bd's default when a pick omits one.
const DefaultPriority = 2

// graphNode / graphEdge / graphPlan mirror the bd create --graph wire schema
// (grounded via --help/--dry-run: nodes keyed by "key", deps as top-level
// edges with from_key/to_key/type; UNKNOWN FIELDS ARE SILENTLY DROPPED — so
// these field names must match bd exactly).
type graphNode struct {
	Key         string   `json:"key"`
	Title       string   `json:"title"`
	Description string   `json:"description,omitempty"`
	Type        string   `json:"type,omitempty"`
	ParentKey   string   `json:"parent_key,omitempty"`
	Labels      []string `json:"labels,omitempty"`
	Priority    int      `json:"priority"`
}

type graphEdge struct {
	FromKey string `json:"from_key"`
	ToKey   string `json:"to_key"`
	Type    string `json:"type"`
}

type graphPlan struct {
	Nodes []graphNode `json:"nodes"`
	Edges []graphEdge `json:"edges"`
}

// GraphJSON builds the bd create --graph payload for a first emit (spec §6).
// The epic becomes an epic node; each pick a task node parented to it, carrying
// its weft-ref:<ref> identity label plus any authored labels.
func GraphJSON(p WarpPlan, d Derivation) ([]byte, error) {
	desc := p.Epic.Description
	if p.Epic.Acceptance != "" {
		// The graph node schema's acceptance field is unconfirmed (§8); fold it
		// into the description so it is never silently dropped.
		desc = strings.TrimRight(desc, "\n") + "\n\n## Acceptance\n" + p.Epic.Acceptance
	}
	nodes := []graphNode{{
		Key:         EpicKey,
		Title:       p.Epic.Title,
		Description: desc,
		Type:        "epic",
		Priority:    DefaultPriority,
	}}
	for _, pk := range sortedPicks(p.Picks) {
		nodes = append(nodes, graphNode{
			Key:         pk.Ref,
			Title:       pk.Title,
			Description: pk.Description,
			Type:        "task",
			ParentKey:   EpicKey,
			Labels:      labelsFor(pk),
			Priority:    priorityOf(pk),
		})
	}
	edges := []graphEdge{}
	for _, e := range d.Edges {
		edges = append(edges, graphEdge{FromKey: e.From, ToKey: e.To, Type: EdgeType})
	}
	return json.MarshalIndent(graphPlan{Nodes: nodes, Edges: edges}, "", "  ")
}

// labelsFor returns a pick's emitted labels: the weft-ref identity label first,
// then any authored labels (deduped, stable order).
func labelsFor(pk Pick) []string {
	out := []string{RefLabelPrefix + pk.Ref}
	seen := map[string]bool{out[0]: true}
	for _, l := range pk.Labels {
		if !seen[l] {
			out = append(out, l)
			seen[l] = true
		}
	}
	return out
}

func priorityOf(pk Pick) int {
	if pk.Priority != nil {
		return *pk.Priority
	}
	return DefaultPriority
}

func sortedPicks(picks []Pick) []Pick {
	out := append([]Pick{}, picks...)
	sort.Slice(out, func(i, j int) bool { return out[i].Ref < out[j].Ref })
	return out
}
