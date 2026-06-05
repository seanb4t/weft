// internal/cli/ws.go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Weft Contributors

package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/seanb4t/weft/internal/exit"
	"github.com/seanb4t/weft/internal/run"
	"github.com/seanb4t/weft/internal/workspace"
	"github.com/spf13/cobra"
)

func (a *App) newWsCmd() *cobra.Command {
	ws := &cobra.Command{Use: "ws", Short: "Workspace escape hatches (spec §4.3)"}
	ws.AddCommand(a.newWsListCmd(), a.newWsAddCmd(), a.newWsForgetCmd())
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
			names := splitTrimLines(res.Stdout)
			data := map[string]any{"workspaces": names}
			return Emit(cmd, "ws.list", data, fmt.Sprintf("workspaces: %s", strings.Join(names, " ")))
		},
	}
}

func (a *App) newWsAddCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "add <bead-id>",
		Short: "Create a jj workspace for a bead on trunk()",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			bead := args[0]
			root, err := jjRoot(a.Runner)
			if err != nil {
				return err
			}
			path := workspace.Path(root, a.Config.Workspace.Root, bead)
			name := workspace.Name(bead)
			res, err := run.JJ(a.Runner, "workspace", "add", path, "--name", name, "-r", "trunk()")
			if err != nil {
				return exit.Hardf("jj workspace add could not run: %v", err)
			}
			if res.Code != 0 {
				return exit.Hardf("jj workspace add failed: %s", strings.TrimSpace(res.Stderr))
			}
			data := map[string]any{"bead": bead, "workspace": name, "path": path}
			return Emit(cmd, "ws.add", data, fmt.Sprintf("workspace %s at %s", name, path))
		},
	}
}

func (a *App) newWsForgetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "forget <bead-id>",
		Short: "Forget a bead's jj workspace and remove its directory",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			bead := args[0]
			root, err := jjRoot(a.Runner)
			if err != nil {
				return err
			}
			name := workspace.Name(bead)
			res, err := run.JJ(a.Runner, "workspace", "forget", name)
			if err != nil {
				return exit.Hardf("jj workspace forget could not run: %v", err)
			}
			if res.Code != 0 {
				return exit.Hardf("jj workspace forget failed: %s", strings.TrimSpace(res.Stderr))
			}
			path := workspace.Path(root, a.Config.Workspace.Root, bead)
			if err := os.RemoveAll(path); err != nil {
				return exit.Hardf("rm workspace dir %s: %v", path, err)
			}
			data := map[string]any{"bead": bead, "workspace": name, "path": path}
			return Emit(cmd, "ws.forget", data, fmt.Sprintf("forgot workspace %s", name))
		},
	}
}

// jjRoot returns the repo root via `jj root`. Shared by ws/shed/reap verbs.
func jjRoot(r run.Runner) (string, error) {
	res, err := run.JJ(r, "root")
	if err != nil {
		return "", exit.Hardf("jj root could not run: %v", err)
	}
	if res.Code != 0 {
		return "", exit.Hardf("jj root failed: %s", strings.TrimSpace(res.Stderr))
	}
	return strings.TrimSpace(res.Stdout), nil
}
