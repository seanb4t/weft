// internal/cli/shed.go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Weft Contributors

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/seanb4t/weft/internal/exit"
	"github.com/seanb4t/weft/internal/run"
	"github.com/seanb4t/weft/internal/workspace"
	"github.com/spf13/cobra"
)

func (a *App) newShedCmd() *cobra.Command {
	shed := &cobra.Command{Use: "shed", Short: "Wave-level orchestration (spec §4.1)"}
	shed.AddCommand(a.newShedFormCmd(), a.newShedIsolateCmd(), a.newShedCleanupCmd())
	return shed
}

func (a *App) newShedFormCmd() *cobra.Command {
	var epic string
	var max int
	c := &cobra.Command{
		Use:   "form",
		Short: "Form a shed: the ready wave for an epic (bd ready ∩ epic, capped)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if epic == "" {
				return exit.Invocationf("--epic is required")
			}
			// Guard the cap explicitly: bd treats `--limit 0` as UNLIMITED, so
			// --max 0 (or negative) would silently invert the dial from "cap the
			// wave" to "no cap". Reject it as an invocation error.
			if max < 1 {
				return exit.Invocationf("--max must be >= 1 (got %d)", max)
			}
			res, err := run.BD(a.Runner, "ready", "--parent", epic, "--limit", strconv.Itoa(max), "--json")
			if err != nil {
				return exit.Hardf("bd ready could not run: %v", err)
			}
			if res.Code != 0 {
				return exit.Hardf("bd ready failed: %s", strings.TrimSpace(res.Stderr))
			}
			var issues []struct {
				ID string `json:"id"`
			}
			if err := json.Unmarshal([]byte(res.Stdout), &issues); err != nil {
				return exit.Hardf("parse bd ready json: %v", err)
			}
			wave := make([]string, 0, len(issues))
			for _, i := range issues {
				wave = append(wave, i.ID)
			}
			data := map[string]any{"epic": epic, "wave": wave}
			text := fmt.Sprintf("shed for %s: %s (%d picks)", epic, strings.Join(wave, " "), len(wave))
			return Emit(cmd, "shed.form", data, text)
		},
	}
	c.Flags().StringVar(&epic, "epic", "", "epic bead-id scoping the ready set (required)")
	// --max is the parallelism dial; its default comes from .weft/config.toml
	// [shed].max (falling back to config.DefaultShedMax). --max overrides it.
	c.Flags().IntVar(&max, "max", a.Config.ShedMax(), "max wave size (parallelism dial)")
	return c
}

func (a *App) newShedCleanupCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "cleanup <bead-id>...",
		Short: "Tear down a wave's workspaces (jj workspace forget + rm)",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := jjRoot(a.Runner)
			if err != nil {
				return err
			}
			wtRoot := workspace.Root(root, a.Config.Workspace.Root)
			cleaned := []string{}
			for _, bead := range args {
				name := workspace.Name(bead)
				path := workspace.Path(root, a.Config.Workspace.Root, bead)
				// Path-safety guard (spec §5): a bead-id carrying "/" or ".."
				// must not let os.RemoveAll escape the worktrees root.
				if !workspace.Contains(wtRoot, path) {
					return exit.Hardf("refusing to clean %q: resolves outside worktrees root %s", bead, wtRoot)
				}
				if res, err := run.JJ(a.Runner, "workspace", "forget", name); err != nil {
					return exit.Hardf("jj workspace forget could not run: %v", err)
				} else if res.Code != 0 {
					return exit.Hardf("jj workspace forget %s failed: %s", bead, strings.TrimSpace(res.Stderr))
				}
				if err := os.RemoveAll(path); err != nil {
					return exit.Hardf("rm workspace dir %s: %v", path, err)
				}
				cleaned = append(cleaned, bead)
			}
			data := map[string]any{"cleaned": cleaned}
			return Emit(cmd, "shed.cleanup", data, fmt.Sprintf("cleaned %d workspace(s)", len(cleaned)))
		},
	}
}

func (a *App) newShedIsolateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "isolate <bead-id>...",
		Short: "Isolate a wave: per bead set in_progress, then create its workspace on trunk()",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Trunk freshness (spec §7): fetch once per wave before isolating.
			if res, err := run.JJ(a.Runner, "git", "fetch"); err != nil {
				return exit.Hardf("jj git fetch could not run: %v", err)
			} else if res.Code != 0 {
				return exit.Hardf("jj git fetch failed: %s", strings.TrimSpace(res.Stderr))
			}
			root, err := jjRoot(a.Runner)
			if err != nil {
				return err
			}
			isolated := []string{}
			for _, bead := range args {
				// Status-first ordering invariant (spec §4): in_progress BEFORE
				// the workspace exists, so a crash never strands a reapable workspace.
				if res, err := run.BD(a.Runner, "update", bead, "--status", "in_progress"); err != nil {
					return exit.Hardf("bd update could not run: %v", err)
				} else if res.Code != 0 {
					return exit.Hardf("bd update %s failed: %s", bead, strings.TrimSpace(res.Stderr))
				}
				path := workspace.Path(root, a.Config.Workspace.Root, bead)
				name := workspace.Name(bead)
				// If add fails here, the bead is already in_progress with no
				// workspace. That is the deliberate status-first trade (spec §4):
				// recovery is `weft resume`, which surfaces an in_progress bead
				// that has no workspace and re-dispatches it — never a reaper
				// concern. The error names the bead so the strand is explicit.
				if res, err := run.JJ(a.Runner, "workspace", "add", path, "--name", name, "-r", "trunk()"); err != nil {
					return exit.Hardf("jj workspace add could not run: %v", err)
				} else if res.Code != 0 {
					return exit.Hardf("jj workspace add %s failed (bead left in_progress for resume): %s", bead, strings.TrimSpace(res.Stderr))
				}
				isolated = append(isolated, bead)
			}
			data := map[string]any{"wave": isolated}
			return Emit(cmd, "shed.isolate", data,
				fmt.Sprintf("isolated %d picks: %s", len(isolated), strings.Join(isolated, " ")))
		},
	}
}
