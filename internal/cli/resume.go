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
			conflicts, err := conflictChanges(a.Runner)
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

// conflictChanges lists the change-ids of conflicted commits in the stack.
//
// NOTE (v1 limitation, tracked by weft-hjx.6): this uses a bare repo-wide
// conflicts() revset, which can over-report in a multi-workspace/multi-epic
// repo (it surfaces conflicts that may not belong to the resumed epic). Unlike
// pick land / shed integrate — whose verdicts GATE behavior and therefore scope
// as `conflicts() & <change>` — resume is read-only observability, so the cost
// is a display inaccuracy, not a wrong verdict. weft-hjx.6 tracks scoping this
// to the epic's stack (intersect with the epic beads' jj-change ids).
//
// F6 (schema note): resume emits conflicts as bare change-ids ([]string) for
// observability; the actionable [{bead,change}] form is shed integrate's (which
// can map each conflicted change to its owning bead via the wave stack, enabling
// the orchestrator to directly call `conflict open <bead>`).
func conflictChanges(r run.Runner) ([]string, error) {
	res, err := run.JJ(r, "log", "-r", "conflicts()", "--no-graph", "-T", `change_id.short(12) ++ "\n"`)
	if err != nil {
		return nil, exit.Hardf("jj log conflicts() could not run: %v", err)
	}
	if res.Code != 0 {
		return nil, exit.Hardf("jj log conflicts() failed: %s", strings.TrimSpace(res.Stderr))
	}
	out := []string{}
	for _, ln := range strings.Split(strings.TrimSpace(res.Stdout), "\n") {
		if ln = strings.TrimSpace(ln); ln != "" {
			out = append(out, ln)
		}
	}
	return out, nil
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
