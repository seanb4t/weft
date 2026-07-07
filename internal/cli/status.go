// internal/cli/status.go
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

// statusCounts tallies an epic's picks by raw bd status (weft-6xi step 2/5).
// Ready is deliberately NOT surfaced as a fifth count: doing so would need an
// extra `bd ready --parent <epic>` subprocess call per epic on the whole-warp
// path (O(epics) extra calls), and the four raw statuses already let a caller
// derive readiness from `bd ready` directly when needed. Keeping status to
// bd's own four statuses also keeps every count reproducible from a single
// `bd list` call, which is what makes the tally below pure and unit-testable.
type statusCounts struct {
	Closed     int `json:"closed"`
	InProgress int `json:"in_progress"`
	Blocked    int `json:"blocked"`
	Open       int `json:"open"`
}

// done is closed picks; remaining is everything not yet closed.
func (c statusCounts) done() int      { return c.Closed }
func (c statusCounts) remaining() int { return c.InProgress + c.Open + c.Blocked }

func (c *statusCounts) add(o statusCounts) {
	c.Closed += o.Closed
	c.InProgress += o.InProgress
	c.Blocked += o.Blocked
	c.Open += o.Open
}

// countByStatus is a pure, subprocess-free counting helper (the TDD core):
// given an epic's already-parsed picks, tally them by raw bd status. A status
// value outside open/in_progress/blocked/closed (bd's vocabulary may grow) is
// silently skipped — status is a live readout, not a validator, so an
// unrecognised status must not crash or skew the counts it does understand.
func countByStatus(children []warpChild) statusCounts {
	var c statusCounts
	for _, ch := range children {
		switch ch.Status {
		case "closed":
			c.Closed++
		case "in_progress":
			c.InProgress++
		case "blocked":
			c.Blocked++
		case "open":
			c.Open++
		}
	}
	return c
}

// epicStatus is one epic's entry in the status readout: id/title identify it,
// counts is the raw per-status tally, done/remaining are the derived
// closed-vs-not split (spec weft-6xi step 6).
type epicStatus struct {
	ID        string       `json:"id"`
	Title     string       `json:"title"`
	Counts    statusCounts `json:"counts"`
	Done      int          `json:"done"`
	Remaining int          `json:"remaining"`
}

func newEpicStatus(id, title string, c statusCounts) epicStatus {
	return epicStatus{ID: id, Title: title, Counts: c, Done: c.done(), Remaining: c.remaining()}
}

// aggregateStatus is the cross-epic (whole-warp) or single-epic (drill)
// summary attached at data.aggregate.
type aggregateStatus struct {
	Epics     int          `json:"epics"`
	Counts    statusCounts `json:"counts"`
	Done      int          `json:"done"`
	Remaining int          `json:"remaining"`
}

func newAggregate(epics int, c statusCounts) aggregateStatus {
	return aggregateStatus{Epics: epics, Counts: c, Done: c.done(), Remaining: c.remaining()}
}

func (a *App) newStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status [epic-id]",
		Short: "Whole-warp or per-epic pick-status readout, computed live from beads",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 1 {
				return a.statusEpic(cmd, args[0])
			}
			return a.statusOverview(cmd)
		},
	}
}

// statusOverview builds the whole-warp readout: every epic (`bd list --type
// epic --json`) with its picks (`bd list --parent <epic> --json`) counted by
// status, plus an aggregate across all epics. An empty warp (no epics) is not
// an error: it exits 0 with an explicit empty readout.
func (a *App) statusOverview(cmd *cobra.Command) error {
	res, err := run.BD(a.Runner, "list", "--type", "epic", "--json")
	if err != nil {
		return exit.Hardf("bd list --type epic could not run: %v", err)
	}
	if res.Code != 0 {
		return exit.Hardf("bd list --type epic failed: %s", strings.TrimSpace(res.Stderr))
	}
	var epics []warpChild
	if err := json.Unmarshal([]byte(res.Stdout), &epics); err != nil {
		return exit.Hardf("parse bd list --type epic json: %v", err)
	}

	results := make([]epicStatus, 0, len(epics))
	var agg statusCounts
	for _, e := range epics {
		children, err := epicChildren(a.Runner, e.ID)
		if err != nil {
			return err
		}
		c := countByStatus(children)
		agg.add(c)
		results = append(results, newEpicStatus(e.ID, e.Title, c))
	}

	data := map[string]any{
		"epics":     results,
		"aggregate": newAggregate(len(results), agg),
	}
	return Emit(cmd, "status", data, statusOverviewText(results, agg))
}

// statusEpic drills into one epic: its picks grouped by status. The epic-id
// argument is validated with the shared allowlist guard (finish.go) before it
// reaches a subprocess.
func (a *App) statusEpic(cmd *cobra.Command, epic string) error {
	if err := validateEpicID(epic); err != nil {
		return err
	}
	info, err := showBead(a.Runner, epic)
	if err != nil {
		return err
	}
	children, err := epicChildren(a.Runner, epic)
	if err != nil {
		return err
	}
	c := countByStatus(children)
	es := newEpicStatus(epic, info.Title, c)

	data := map[string]any{
		"epics":     []epicStatus{es},
		"aggregate": newAggregate(1, c),
	}
	return Emit(cmd, "status", data, statusDrillText(es, children))
}

// epicChildren reads all of an epic's direct children via `bd list --parent
// <epic> --json`, parsed into the warpChild shape (plan.go). Unlike warpScan
// (plan.go), this does NOT filter to children carrying a weft-ref: label —
// status must count every pick under the epic, labelled or not.
func epicChildren(r run.Runner, epic string) ([]warpChild, error) {
	res, err := run.BD(r, "list", "--parent", epic, "--json")
	if err != nil {
		return nil, exit.Hardf("bd list --parent %s could not run: %v", epic, err)
	}
	if res.Code != 0 {
		return nil, exit.Hardf("bd list --parent %s failed: %s", epic, strings.TrimSpace(res.Stderr))
	}
	var arr []warpChild
	if err := json.Unmarshal([]byte(res.Stdout), &arr); err != nil {
		return nil, exit.Hardf("parse bd list --parent %s json: %v", epic, err)
	}
	return arr, nil
}

// statusOverviewText renders the whole-warp human text: one line per epic then
// an aggregate line. An empty warp gets an explicit "no epics" line rather than
// a bare aggregate, so the empty case is never mistaken for missing output.
func statusOverviewText(epics []epicStatus, agg statusCounts) string {
	var b strings.Builder
	if len(epics) == 0 {
		b.WriteString("no epics — warp is empty")
	} else {
		for i, e := range epics {
			if i > 0 {
				b.WriteString("\n")
			}
			fmt.Fprintf(&b, "%s %s — closed %d, in_progress %d, blocked %d, open %d (done %d, remaining %d)",
				e.ID, e.Title, e.Counts.Closed, e.Counts.InProgress, e.Counts.Blocked, e.Counts.Open, e.Done, e.Remaining)
		}
	}
	fmt.Fprintf(&b, "\naggregate: %d epic(s) — closed %d, in_progress %d, blocked %d, open %d (done %d, remaining %d)",
		len(epics), agg.Closed, agg.InProgress, agg.Blocked, agg.Open, agg.done(), agg.remaining())
	return b.String()
}

// statusDrillText renders one epic's picks grouped by status (spec weft-j4c
// acceptance 2). Status groups render in a fixed order so output is
// deterministic across runs; a group with no members is omitted.
func statusDrillText(es epicStatus, children []warpChild) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s %s — closed %d, in_progress %d, blocked %d, open %d (done %d, remaining %d)",
		es.ID, es.Title, es.Counts.Closed, es.Counts.InProgress, es.Counts.Blocked, es.Counts.Open, es.Done, es.Remaining)
	for _, status := range []string{"closed", "in_progress", "blocked", "open"} {
		group := make([]warpChild, 0, len(children))
		for _, ch := range children {
			if ch.Status == status {
				group = append(group, ch)
			}
		}
		if len(group) == 0 {
			continue
		}
		fmt.Fprintf(&b, "\n  %s:", status)
		for _, ch := range group {
			fmt.Fprintf(&b, "\n    %s %s", ch.ID, ch.Title)
		}
	}
	return b.String()
}
