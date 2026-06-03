// internal/cli/plan.go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Weft Contributors

package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/seanb4t/weft/internal/exit"
	"github.com/seanb4t/weft/internal/plan"
	"github.com/seanb4t/weft/internal/run"
	"github.com/spf13/cobra"
)

func (a *App) newPlanCmd() *cobra.Command {
	p := &cobra.Command{Use: "plan", Short: "Planning -> warp emission (spec seam 2)"}
	p.AddCommand(a.newPlanCheckCmd(), a.newPlanEmitCmd())
	return p
}

func (a *App) newPlanEmitCmd() *cobra.Command {
	var dryRun bool
	var epic string
	c := &cobra.Command{
		Use:   "emit <file>",
		Short: "Emit the warp from warp-plan.json (derive edges, preview, create/upsert)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			wp, err := plan.Load(args[0])
			if err != nil {
				return exit.Invocationf("%v", err)
			}
			if issues := plan.Validate(wp); len(issues) > 0 {
				return exit.Invocationf("warp-plan is invalid (%d issue(s)); run 'weft plan check' first", len(issues))
			}
			d := plan.Derive(wp.Picks, a.Config.PlanStructural(), a.Config.PlanOverlapMax())
			if epic != "" {
				return a.planReplan(cmd, wp, d, epic, dryRun) // Task 8
			}
			return a.planFirstEmit(cmd, wp, d, dryRun)
		},
	}
	c.Flags().BoolVar(&dryRun, "dry-run", false, "preview the warp without mutating beads")
	c.Flags().StringVar(&epic, "epic", "", "existing epic id to re-plan against (bd import upsert)")
	return c
}

// planFirstEmit creates a brand-new warp via bd create --graph (spec §6).
func (a *App) planFirstEmit(cmd *cobra.Command, wp plan.WarpPlan, d plan.Derivation, dryRun bool) error {
	graph, err := plan.GraphJSON(wp, d)
	if err != nil {
		return exit.Hardf("build graph payload: %v", err)
	}
	if dryRun {
		data := map[string]any{
			"dry_run": true, "mode": "create", "epic": wp.Epic.Title,
			"picks": len(wp.Picks), "edges": d.Edges, "tolerated": d.Tolerated,
		}
		return Emit(cmd, "plan.emit", data, planPreviewText("create", wp, d))
	}
	// bd create --graph takes a file path (no stdin), so stage the payload.
	path, cleanup, err := writeTempPayload("weft-warp-*.json", graph)
	if err != nil {
		return err
	}
	defer cleanup()
	res, err := run.BD(a.Runner, "create", "--graph", path)
	if err != nil {
		return exit.Hardf("bd create --graph could not run: %v", err)
	}
	if res.Code != 0 {
		return exit.Hardf("bd create --graph failed: %s", strings.TrimSpace(res.Stderr))
	}
	data := map[string]any{
		"mode": "create", "created": len(wp.Picks), "edges": d.Edges,
		"tolerated": d.Tolerated, "bd_output": strings.TrimSpace(res.Stdout),
	}
	text := fmt.Sprintf("emitted warp: %d pick(s), %d edge(s), %d tolerated overlap(s)\n%s",
		len(wp.Picks), len(d.Edges), len(d.Tolerated), strings.TrimSpace(res.Stdout))
	return Emit(cmd, "plan.emit", data, text)
}

// writeTempPayload stages a payload weft must hand to bd as a file path.
func writeTempPayload(pattern string, payload []byte) (string, func(), error) {
	f, err := os.CreateTemp("", pattern)
	if err != nil {
		return "", func() {}, exit.Hardf("temp payload file: %v", err)
	}
	if _, err := f.Write(payload); err != nil {
		f.Close()
		os.Remove(f.Name())
		return "", func() {}, exit.Hardf("write payload: %v", err)
	}
	f.Close()
	return f.Name(), func() { os.Remove(f.Name()) }, nil
}

// planPreviewText renders the dry-run human gate (spec §5): edges + the
// warn+tolerate overlaps the human is approving.
func planPreviewText(mode string, wp plan.WarpPlan, d plan.Derivation) string {
	var b strings.Builder
	fmt.Fprintf(&b, "DRY RUN (%s) — epic %q, %d pick(s), %d edge(s)\n", mode, wp.Epic.Title, len(wp.Picks), len(d.Edges))
	for _, e := range d.Edges {
		fmt.Fprintf(&b, "  edge: %s depends on %s\n", e.From, e.To)
	}
	if len(d.Tolerated) > 0 {
		fmt.Fprintf(&b, "  %d tolerated overlap(s) (same shed; conflict resolved via seam 4):\n", len(d.Tolerated))
		for _, o := range d.Tolerated {
			fmt.Fprintf(&b, "    %s ~ %s share %v\n", o.A, o.B, o.Shared)
		}
	}
	b.WriteString("  (no mutation — re-run without --dry-run to emit)")
	return b.String()
}

// planReplan is a TEMPORARY STUB — Task 8 replaces this with bd import upsert.
func (a *App) planReplan(cmd *cobra.Command, wp plan.WarpPlan, d plan.Derivation, epic string, dryRun bool) error {
	return exit.Invocationf("re-plan (--epic) not yet implemented")
}

func (a *App) newPlanCheckCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "check <file>",
		Short: "Validate warp-plan.json; validity is data (exit 0, no mutation)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			wp, err := plan.Load(args[0])
			if err != nil {
				return exit.Invocationf("%v", err)
			}
			issues := plan.Validate(wp)
			data := map[string]any{"valid": len(issues) == 0, "issues": issues}
			text := fmt.Sprintf("valid: %d pick(s), no issues", len(wp.Picks))
			if len(issues) > 0 {
				text = fmt.Sprintf("INVALID: %d issue(s)", len(issues))
				for _, is := range issues {
					if is.Ref != "" {
						text += fmt.Sprintf("\n  - [%s] %s", is.Ref, is.Message)
					} else {
						text += fmt.Sprintf("\n  - %s", is.Message)
					}
				}
			}
			return Emit(cmd, "plan.check", data, text)
		},
	}
}
