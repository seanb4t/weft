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
	pick.AddCommand(a.newPickSealCmd())
	return pick
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
