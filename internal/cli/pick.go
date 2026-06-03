// internal/cli/pick.go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Weft Contributors

package cli

import (
	"fmt"
	"strings"

	"github.com/seanb4t/weft/internal/exit"
	"github.com/seanb4t/weft/internal/run"
	"github.com/spf13/cobra"
)

func (a *App) newPickCmd() *cobra.Command {
	pick := &cobra.Command{Use: "pick", Short: "Bead-level pick lifecycle (spec §4.2)"}
	pick.AddCommand(a.newPickSealCmd(), a.newPickVerifyCmd(), a.newPickLandCmd(), a.newPickRedoCmd())
	return pick
}

func (a *App) newPickVerifyCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "verify <bead>",
		Short: "Run the configured verify gate; the verdict is data (spec §4.2)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			bead := args[0]
			gate := strings.TrimSpace(a.Config.Verify.Command)
			if gate == "" {
				return exit.Invocationf("no verify gate configured ([verify].command in .weft/config.toml)")
			}
			// The engine ran the gate fine, so this verb exits 0 regardless of the
			// gate's own exit — the pass/fail verdict is DATA (spec §3).
			res, err := a.Runner.Run("sh", "-c", gate)
			if err != nil {
				return exit.Hardf("verify gate could not run: %v", err)
			}
			pass := res.Code == 0
			data := map[string]any{"bead": bead, "pass": pass}
			verdict := "FAIL"
			if pass {
				verdict = "PASS"
			}
			return Emit(cmd, "pick.verify", data, fmt.Sprintf("verify %s: %s", bead, verdict))
		},
	}
}

func (a *App) newPickLandCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "land <bead>",
		Short: "Land a pick: assert its change is conflict-free, then bd close (spec §4.2)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			bead := args[0]
			change, err := changeOf(a.Runner, bead)
			if err != nil {
				return err
			}
			if change == "" {
				return exit.Invocationf("bead %s has no jj-change label (not sealed/integrated)", bead)
			}
			// Never land a conflicted change (seam 4 §6): the gate is concrete —
			// the change must not be in conflicts().
			res, err := run.JJ(a.Runner, "log", "-r", "conflicts() & "+change, "--no-graph", "-T", `change_id.short(12) ++ "\n"`)
			if err != nil {
				return exit.Hardf("jj conflicts check could not run: %v", err)
			}
			if res.Code != 0 {
				return exit.Hardf("jj conflicts check failed: %s", strings.TrimSpace(res.Stderr))
			}
			if strings.TrimSpace(res.Stdout) != "" {
				return exit.Invocationf("refusing to land %s: change %s is conflicted (resolve first)", bead, change)
			}
			if res, err := run.BD(a.Runner, "close", bead, "--suggest-next"); err != nil {
				return exit.Hardf("bd close could not run: %v", err)
			} else if res.Code != 0 {
				return exit.Hardf("bd close %s failed: %s", bead, strings.TrimSpace(res.Stderr))
			}
			data := map[string]any{"bead": bead, "change": change}
			return Emit(cmd, "pick.land", data, fmt.Sprintf("landed %s (change %s)", bead, change))
		},
	}
}

func (a *App) newPickRedoCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "redo <bead>",
		Short: "Recovery: abandon the pick's change (if any) and reopen the bead (spec §4.1)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			bead := args[0]
			change, err := changeOf(a.Runner, bead)
			if err != nil {
				return err
			}
			// A sealed pick has a change to abandon; an unsealed pick (crash before
			// seal) has no jj-change label, so changeOf returned "" and we skip
			// straight to the reopen below.
			if change != "" {
				// Abandon the sealed change and drop its now-dangling spine label.
				if res, err := run.JJ(a.Runner, "abandon", change); err != nil {
					return exit.Hardf("jj abandon could not run: %v", err)
				} else if res.Code != 0 {
					return exit.Hardf("jj abandon %s failed: %s", change, strings.TrimSpace(res.Stderr))
				}
				// Drop the now-dangling spine label (best-effort).
				_, _ = run.BD(a.Runner, "update", bead, "--remove-label", jjChangeLabelPrefix+change)
			}
			// Reopen to open so the next shed form re-picks it (in_progress → open).
			if res, err := run.BD(a.Runner, "update", bead, "--status", "open"); err != nil {
				return exit.Hardf("bd update could not run: %v", err)
			} else if res.Code != 0 {
				return exit.Hardf("bd update %s failed: %s", bead, strings.TrimSpace(res.Stderr))
			}
			data := map[string]any{"bead": bead}
			if change != "" {
				data["abandoned"] = change
			}
			return Emit(cmd, "pick.redo", data, fmt.Sprintf("redo %s (abandoned %q, reopened)", bead, change))
		},
	}
}

func (a *App) newPickSealCmd() *cobra.Command {
	var ctype string
	c := &cobra.Command{
		Use:   "seal <bead>",
		Short: "Seal the executor's work: jj commit + pin the jj-change spine label",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			bead := args[0]
			info, err := showBead(a.Runner, bead)
			if err != nil {
				return err
			}
			if existing := changeFromLabels(info.Labels); existing != "" {
				return exit.Invocationf("bead %s is already sealed (change %s); use 'weft pick redo %s' to re-seal", bead, existing, bead)
			}
			msg := fmt.Sprintf("%s(%s): %s", ctype, bead, info.Title)
			if res, err := run.JJ(a.Runner, "commit", "-m", msg); err != nil {
				return exit.Hardf("jj commit could not run: %v", err)
			} else if res.Code != 0 {
				return exit.Hardf("jj commit failed: %s", strings.TrimSpace(res.Stderr))
			}
			// The sealed change is now @- (jj commit describes @ and opens a new empty @).
			res, err := run.JJ(a.Runner, "log", "-r", "@-", "--no-graph", "-T", "change_id.short(12)")
			if err != nil {
				return exit.Hardf("read sealed change-id could not run: %v", err)
			}
			if res.Code != 0 {
				return exit.Hardf("read sealed change-id failed: %s", strings.TrimSpace(res.Stderr))
			}
			change := strings.TrimSpace(res.Stdout)
			if change == "" {
				return exit.Hardf("jj log -r @- returned an empty change-id for %s", bead)
			}
			if res, err := run.BD(a.Runner, "update", bead, "--add-label", jjChangeLabelPrefix+change); err != nil {
				return exit.Hardf("bd add-label could not run: %v", err)
			} else if res.Code != 0 {
				return exit.Hardf("bd add-label failed: %s", strings.TrimSpace(res.Stderr))
			}
			data := map[string]any{"bead": bead, "change": change}
			return Emit(cmd, "pick.seal", data, fmt.Sprintf("sealed %s as '%s' (change %s)", bead, msg, change))
		},
	}
	c.Flags().StringVar(&ctype, "type", "feat", "conventional-commit type for the message")
	return c
}
