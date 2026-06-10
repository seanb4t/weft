// internal/plan/verify.go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Weft Contributors

package plan

import (
	"fmt"
	"sort"
	"strings"
)

// ReplanExpect is the authored expectation for one pick after import: the
// fields BuildReplan wrote into the importRecord that bd must persist.
// Populated in BuildReplan's pick loop alongside Created/Updated.
type ReplanExpect struct {
	Ref      string // pick ref (matches the weft-ref:<ref> label)
	Title    string
	Priority int
	Labels   []string // authored labels from labelsFor(pk), includes weft-ref:<ref>
	HasDesc  bool     // true when the importRecord had a non-empty description
}

// ReadbackBead is one bead re-read after import, keyed for verification by
// its weft-ref label. The json tags match the `bd list --json` output keys so
// callers can unmarshal directly into []ReadbackBead.
type ReadbackBead struct {
	// ID is the bead id, for post-import edge wiring; ignored by VerifyReplan.
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Priority    int      `json:"priority"`
	Labels      []string `json:"labels"`
	Description string   `json:"description"`
}

// VerifyReplan diffs intended import expectations against the beads bd actually
// persisted (read back via bd list after import), returning human-readable
// discrepancies for any authored field that did not round-trip. Empty => clean.
// The returned slice is always non-nil (never JSON null).
func VerifyReplan(expect []ReplanExpect, actual map[string]ReadbackBead) []string {
	disc := []string{}

	// Sort by ref for deterministic output.
	sorted := make([]ReplanExpect, len(expect))
	copy(sorted, expect)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Ref < sorted[j].Ref })

	for _, ex := range sorted {
		ab, ok := actual[ex.Ref]
		if !ok {
			disc = append(disc, fmt.Sprintf("pick %s: not found after import (create/update did not persist)", ex.Ref))
			continue
		}
		if ab.Title != ex.Title {
			disc = append(disc, fmt.Sprintf("pick %s: title not persisted (sent %q, got %q)", ex.Ref, ex.Title, ab.Title))
		}
		if ab.Priority != ex.Priority {
			disc = append(disc, fmt.Sprintf("pick %s: priority not persisted (sent %d, got %d)", ex.Ref, ex.Priority, ab.Priority))
		}
		// Subset check: every authored label must be present in actual.Labels.
		actualSet := map[string]bool{}
		for _, l := range ab.Labels {
			actualSet[l] = true
		}
		for _, l := range ex.Labels {
			if !actualSet[l] {
				disc = append(disc, fmt.Sprintf("pick %s: label %q not persisted", ex.Ref, l))
			}
		}
		// Description presence check (not content — bd normalises whitespace/markdown).
		if ex.HasDesc && strings.TrimSpace(ab.Description) == "" {
			disc = append(disc, fmt.Sprintf("pick %s: description not persisted (dropped)", ex.Ref))
		}
	}
	return disc
}
