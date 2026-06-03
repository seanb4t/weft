// internal/cli/plan.go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Weft Contributors

package cli

import (
	"fmt"

	"github.com/seanb4t/weft/internal/exit"
	"github.com/seanb4t/weft/internal/plan"
	"github.com/spf13/cobra"
)

func (a *App) newPlanCmd() *cobra.Command {
	p := &cobra.Command{Use: "plan", Short: "Planning -> warp emission (spec seam 2)"}
	p.AddCommand(a.newPlanCheckCmd())
	return p
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
