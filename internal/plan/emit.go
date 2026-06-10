// internal/plan/emit.go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Weft Contributors

package plan

import (
	"bytes"
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

// foldAcceptance appends an "## Acceptance" section to a description when
// acceptance text is present (the graph node schema's acceptance field is
// unconfirmed — §8 posture — so it must never be a separate field).
func foldAcceptance(desc, acceptance string) string {
	if acceptance == "" {
		return desc
	}
	return strings.TrimRight(desc, "\n") + "\n\n## Acceptance\n" + acceptance
}

// GraphJSON builds the bd create --graph payload for a first emit (spec §6).
// The epic becomes an epic node; each pick a task node parented to it, carrying
// its weft-ref:<ref> identity label plus any authored labels.
// For roadmap plans (phases present), phase sub-epics replace pick task nodes,
// and inter-phase needs become blocks edges.
func GraphJSON(p WarpPlan, d Derivation) ([]byte, error) {
	nodes := []graphNode{{
		Key:         EpicKey,
		Title:       p.Epic.Title,
		Description: foldAcceptance(p.Epic.Description, p.Epic.Acceptance),
		Type:        "epic",
		Priority:    DefaultPriority,
	}}
	if len(p.Phases) > 0 {
		phases := sortedPhases(p.Phases)
		for _, ph := range phases {
			nodes = append(nodes, graphNode{
				Key:         ph.Ref,
				Title:       ph.Title,
				Description: foldAcceptance(ph.Description, ph.Acceptance),
				Type:        "epic",
				ParentKey:   EpicKey,
				Labels:      []string{RefLabelPrefix + ph.Ref},
				Priority:    DefaultPriority,
			})
		}
		edges := []graphEdge{}
		for _, ph := range phases {
			for _, n := range ph.Needs {
				edges = append(edges, graphEdge{FromKey: ph.Ref, ToKey: n, Type: EdgeType})
			}
		}
		return json.MarshalIndent(graphPlan{Nodes: nodes, Edges: edges}, "", "  ")
	}
	// Pick path — only reached when no phases are present (the roadmap branch above returns).
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

func sortedPhases(phases []Phase) []Phase {
	out := append([]Phase{}, phases...)
	sort.Slice(out, func(i, j int) bool { return out[i].Ref < out[j].Ref })
	return out
}

// RoadmapCounts returns the node/edge counts a roadmap plan's graph payload
// carries: 1 (project epic) + len(phases) nodes and the total authored needs edges.
// Used for the seam-9 preflight comparison. Phase edges come from authored
// needs inside GraphJSON — NOT from Derive — so callers must not use
// Derivation.Edges on the roadmap path (it is always empty there).
func RoadmapCounts(p WarpPlan) (nodes, edges int) {
	nodes = 1 + len(p.Phases)
	for _, ph := range p.Phases {
		edges += len(ph.Needs)
	}
	return nodes, edges
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

// ExistingBead is one live pick in an emitted warp, keyed by its weft-ref label.
type ExistingBead struct {
	ID     string
	Status string
}

// importRecord mirrors the subset of the bd export/import schema weft writes
// per pick (spec §7): upsert keyed by id (empty id => create). Status is set
// only for matched picks, to preserve their lifecycle state across a re-plan.
type importRecord struct {
	ID           string      `json:"id,omitempty"`
	Title        string      `json:"title"`
	Description  string      `json:"description,omitempty"`
	IssueType    string      `json:"issue_type"`
	Priority     int         `json:"priority"`
	Status       string      `json:"status,omitempty"`
	Parent       string      `json:"parent,omitempty"`
	Labels       []string    `json:"labels"`
	Dependencies []importDep `json:"dependencies,omitempty"`
}

type importDep struct {
	IssueID     string `json:"issue_id"`
	DependsOnID string `json:"depends_on_id"`
	Type        string `json:"type"`
}

// Replan is the computed re-plan delta against an existing warp (spec §7).
type Replan struct {
	jsonl         []byte         // bd import payload (one record per pick, newline-delimited)
	Created       []string       // refs with no existing bead (created by import, fields + parent only)
	Updated       []string       // refs matched to an existing bead (fields/labels/edges updated)
	DeferredEdges []Edge         // edges touching a not-yet-created pick (wired post-import — §8)
	Removed       []string       // refs present in the warp but absent from the plan (supersede is §8)
	Expect        []ReplanExpect // authored expectations for post-import read-back verification (never nil)
}

// JSONL returns a defensive copy of the bd import wire payload so callers
// cannot mutate the underlying bytes or cause the payload to drift from the
// Created/Updated/Removed/DeferredEdges delta slices.
func (r Replan) JSONL() []byte {
	out := make([]byte, len(r.jsonl))
	copy(out, r.jsonl)
	return out
}

// BuildReplan computes the bd import upsert payload and the deltas for a re-plan
// (spec §7). refToID maps known refs to existing beads (from weft-ref labels);
// epicID parents every record. Edges are expressed as dependencies only when
// BOTH endpoints already have ids; edges touching a newly created pick are
// reported as DeferredEdges (their bead-id does not exist until import runs).
func BuildReplan(p WarpPlan, d Derivation, epicID string, refToID map[string]ExistingBead) (Replan, error) {
	rp := Replan{Created: []string{}, Updated: []string{}, DeferredEdges: []Edge{}, Removed: []string{}, Expect: []ReplanExpect{}}

	// Group resolvable edges (both endpoints matched) by dependent ref.
	depsByRef := map[string][]importDep{}
	for _, e := range d.Edges {
		from, fok := refToID[e.From]
		to, tok := refToID[e.To]
		if fok && tok {
			depsByRef[e.From] = append(depsByRef[e.From], importDep{IssueID: from.ID, DependsOnID: to.ID, Type: EdgeType})
		} else {
			rp.DeferredEdges = append(rp.DeferredEdges, e)
		}
	}

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf) // one JSON object per line (JSONL)
	for _, pk := range sortedPicks(p.Picks) {
		bead, matched := refToID[pk.Ref]
		rec := importRecord{
			ID:           bead.ID, // "" when unmatched => create
			Title:        pk.Title,
			Description:  pk.Description,
			IssueType:    "task",
			Priority:     priorityOf(pk),
			Parent:       epicID,
			Labels:       labelsFor(pk),
			Dependencies: depsByRef[pk.Ref],
		}
		if matched {
			// Invariant: matched beads always carry a non-empty Status because
			// ExistingBead is populated from `bd list --json`, which always
			// includes a status field on every live bead. `bd import` preserves
			// whatever status is written here (never silently reopens closed
			// work). The `omitempty` on importRecord.Status therefore only drops
			// the field on the create path (unmatched ref, zero-value
			// ExistingBead), which is correct: new beads get bd's default status
			// on create rather than a stale one.
			rec.Status = bead.Status
			rp.Updated = append(rp.Updated, pk.Ref)
		} else {
			rp.Created = append(rp.Created, pk.Ref)
		}
		if err := enc.Encode(rec); err != nil {
			return Replan{}, err
		}
		rp.Expect = append(rp.Expect, ReplanExpect{
			Ref:      pk.Ref,
			Title:    rec.Title,
			Priority: rec.Priority,
			Labels:   rec.Labels,
			HasDesc:  strings.TrimSpace(rec.Description) != "",
		})
	}
	rp.jsonl = buf.Bytes()

	inPlan := map[string]bool{}
	for _, pk := range p.Picks {
		inPlan[pk.Ref] = true
	}
	for ref := range refToID {
		if !inPlan[ref] {
			rp.Removed = append(rp.Removed, ref)
		}
	}
	sort.Strings(rp.Removed)
	return rp, nil
}
