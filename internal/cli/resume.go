// internal/cli/resume.go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Weft Contributors

package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/seanb4t/weft/internal/exit"
	"github.com/seanb4t/weft/internal/run"
	"github.com/spf13/cobra"
)

func (a *App) newResumeCmd() *cobra.Command {
	var epic string
	c := &cobra.Command{
		Use:   "resume",
		Short: "Read-only projection of durable epic state (spec §4.5)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if epic == "" {
				return exit.Invocationf("--epic is required")
			}
			// Output key aliases: the bd status values are mapped to more descriptive
			// output keys — "closed" → "landed", "in_progress" → "in_flight".
			// These aliases are intentional: "landed" matches the spec §4.5 vocabulary
			// and "in_flight" avoids the internal bd status name leaking into the
			// public command surface. The mapping is load-bearing for prompt consumers.
			landed, err := beadIDsByStatus(a.Runner, epic, "closed")
			if err != nil {
				return err
			}
			inflight, err := beadIDsByStatus(a.Runner, epic, "in_progress")
			if err != nil {
				return err
			}
			blocked, err := beadIDsByStatus(a.Runner, epic, "blocked")
			if err != nil {
				return err
			}
			ready, err := readyIDs(a.Runner, epic)
			if err != nil {
				return err
			}
			conflicts, err := conflictChanges(a.Runner, epic)
			if err != nil {
				return err
			}
			data := map[string]any{
				"epic": epic, "landed": landed, "in_flight": inflight,
				"blocked": blocked, "ready": ready, "conflicts": conflicts,
			}
			text := fmt.Sprintf("epic %s — landed %d, in-flight %d, ready %d, blocked %d, conflicts %d",
				epic, len(landed), len(inflight), len(ready), len(blocked), len(conflicts))
			return Emit(cmd, "resume", data, text)
		},
	}
	c.Flags().StringVar(&epic, "epic", "", "epic to project (required)")
	return c
}

// beadIDsByStatus lists ids of the epic's children in a given status.
func beadIDsByStatus(r run.Runner, epic, status string) ([]string, error) {
	res, err := run.BD(r, "list", "--parent", epic, "--status", status, "--json")
	if err != nil {
		return nil, exit.Hardf("bd list could not run: %v", err)
	}
	if res.Code != 0 {
		return nil, exit.Hardf("bd list failed: %s", strings.TrimSpace(res.Stderr))
	}
	return idsFromJSON(res.Stdout)
}

// readyIDs lists ids of the epic's ready (unblocked) children.
func readyIDs(r run.Runner, epic string) ([]string, error) {
	res, err := run.BD(r, "ready", "--parent", epic, "--json")
	if err != nil {
		return nil, exit.Hardf("bd ready could not run: %v", err)
	}
	if res.Code != 0 {
		return nil, exit.Hardf("bd ready failed: %s", strings.TrimSpace(res.Stderr))
	}
	return idsFromJSON(res.Stdout)
}

// conflictChanges lists the change-ids of conflicted commits in this epic's
// stack. It derives the epic's sealed beads' jj-change ids (those carrying a
// jj-change:<id> label) and intersects: conflicts() & (ch1 | ch2 | ...).
//
// Empty-changes short-circuit: when no bead is sealed yet there is nothing in
// this epic that can be conflicted, so the function returns [] immediately
// without building an invalid `conflicts() & ()` revset.
//
// Injection guard: each change-id is validated against changeIDPattern before
// interpolation into the revset string (same rationale as conflict.go's
// changeConflicted — a tampered jj-change label could otherwise alter revset
// evaluation). changeIDPattern is defined in conflict.go (same package).
//
// F6 (schema note): resume emits conflicts as bare change-ids ([]string) for
// observability; the actionable [{bead,change}] form is shed integrate's (which
// can map each conflicted change to its owning bead via the wave stack, enabling
// the orchestrator to directly call `conflict open <bead>`).
func conflictChanges(r run.Runner, epic string) ([]string, error) {
	changes, err := epicChanges(r, epic)
	if err != nil {
		return nil, err
	}
	if len(changes) == 0 {
		// No sealed beads → nothing in this epic can be conflicted. Return empty
		// WITHOUT building an invalid `conflicts() & ()` revset.
		return []string{}, nil
	}
	// Allowlist-validate each change-id before interpolation (revset-injection
	// guard; same rationale as conflict.go changeConflicted — a tampered
	// jj-change label could otherwise alter revset evaluation). changeIDPattern
	// is defined in conflict.go (same package).
	for _, ch := range changes {
		if !changeIDPattern.MatchString(ch) {
			return nil, exit.Hardf("refusing to interpolate unsafe change-id %q into a revset", ch)
		}
	}
	revset := "conflicts() & (" + strings.Join(changes, " | ") + ")"
	res, err := run.JJ(r, "log", "-r", revset, "--no-graph", "-T", `change_id.short(12) ++ "\n"`)
	if err != nil {
		return nil, exit.Hardf("jj log scoped conflicts() %q could not run: %v", revset, err)
	}
	if res.Code != 0 {
		return nil, exit.Hardf("jj log scoped conflicts() %q failed: %s", revset, strings.TrimSpace(res.Stderr))
	}
	return splitTrimLines(res.Stdout), nil
}

// epicChanges returns the jj-change ids of the epic's sealed beads (those
// carrying a jj-change:<id> label), used to scope conflicts() to this epic's
// stack rather than the whole repo. Returns a non-nil empty slice when no
// bead is sealed yet.
func epicChanges(r run.Runner, epic string) ([]string, error) {
	res, err := run.BD(r, "list", "--parent", epic, "--json")
	if err != nil {
		return nil, exit.Hardf("bd list could not run: %v", err)
	}
	if res.Code != 0 {
		return nil, exit.Hardf("bd list failed: %s", strings.TrimSpace(res.Stderr))
	}
	var arr []struct {
		Labels []string `json:"labels"`
	}
	if err := json.Unmarshal([]byte(res.Stdout), &arr); err != nil {
		return nil, exit.Hardf("parse bd json: %v", err)
	}
	changes := []string{}
	for _, b := range arr {
		if ch := changeFromLabels(b.Labels); ch != "" {
			changes = append(changes, ch)
		}
	}
	return changes, nil
}

// idsFromJSON parses a bd issue-array JSON and returns the ids.
func idsFromJSON(s string) ([]string, error) {
	var arr []struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal([]byte(s), &arr); err != nil {
		return nil, exit.Hardf("parse bd json: %v", err)
	}
	ids := []string{}
	for _, i := range arr {
		ids = append(ids, i.ID)
	}
	return ids, nil
}
