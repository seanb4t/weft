// internal/cli/ws.go
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

func (a *App) newWsCmd() *cobra.Command {
	ws := &cobra.Command{Use: "ws", Short: "Workspace escape hatches (spec §4.3)"}
	ws.AddCommand(a.newWsListCmd())
	return ws
}

func (a *App) newWsListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List jj workspaces",
		RunE: func(cmd *cobra.Command, _ []string) error {
			res, err := run.JJ(a.Runner, "workspace", "list", "-T", `name ++ "\n"`)
			if err != nil {
				return exit.Hardf("jj workspace list could not run: %v", err)
			}
			if res.Code != 0 {
				return exit.Hardf("jj workspace list failed: %s", strings.TrimSpace(res.Stderr))
			}
			names := []string{} // non-nil so empty output serializes as [] not null
			for _, ln := range strings.Split(strings.TrimSpace(res.Stdout), "\n") {
				if ln = strings.TrimSpace(ln); ln != "" {
					names = append(names, ln)
				}
			}
			data := map[string]any{"workspaces": names}
			return Emit(cmd, "ws.list", data, fmt.Sprintf("workspaces: %s", strings.Join(names, " ")))
		},
	}
}
