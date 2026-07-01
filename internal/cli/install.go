// internal/cli/install.go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Weft Contributors

package cli

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/seanb4t/weft/internal/install"
)

func (a *App) newInstallCmd() *cobra.Command {
	var scope, ref, local string
	var uninstall, dryRun bool
	c := &cobra.Command{
		Use:   "install",
		Short: "Install the weft Claude Code plugin (pinned to this binary's release)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			res, err := install.Install(a.Runner, install.Options{
				Version:   Version,
				Scope:     scope,
				Ref:       ref,
				Local:     local,
				Uninstall: uninstall,
				DryRun:    dryRun,
			})
			if err != nil {
				return err
			}
			// weft-2kc: surface the running binary so a fresh agent can put it
			// on PATH instead of hunting for a dist build. Best-effort — an
			// os.Executable error just omits the hint, never fails the install.
			binPath, _ := os.Executable()
			data := map[string]any{
				"plugin": res.Plugin, "marketplace": res.Marketplace,
				"source": res.Source, "ref": res.Ref, "scope": res.Scope,
				"uninstall": res.Uninstall, "registered": res.Registered,
				"installed": res.Installed, "commands": res.Commands,
				"binary":  binPath,
				"dry_run": dryRun,
			}
			text := "installed weft plugin (" + res.Scope + ")"
			next := "Restart Claude Code to load the weft plugin; try /weft:execute."
			switch {
			case dryRun:
				text = "[dry-run] would run:\n  " + joinLines(res.Commands)
				next = "" // dry-run installs nothing — no restart hint (Qodo PR #23)
			case uninstall:
				text = "uninstalled weft plugin (" + res.Scope + ")"
				next = "Restart Claude Code to unload the weft plugin."
			}
			// Real install only: append the on-PATH hint (dry-run installs
			// nothing; uninstall is removing).
			if !dryRun && !uninstall && binPath != "" {
				next += " The weft binary is at " + binPath +
					"; symlink it onto your PATH (e.g. `ln -s " + binPath +
					" ~/.local/bin/weft`) so agents can invoke 'weft' directly."
			}
			return EmitNext(cmd, "install", data, text, next)
		},
	}
	c.Flags().StringVar(&scope, "scope", "user", "install scope: user | project | local")
	c.Flags().StringVar(&ref, "ref", "", "override the git ref (branch/tag/sha) instead of the version tag")
	c.Flags().StringVar(&local, "local", "", "install from a local clone path (offline) instead of the git marketplace")
	c.Flags().BoolVar(&uninstall, "uninstall", false, "uninstall the weft plugin")
	c.Flags().BoolVar(&dryRun, "dry-run", false, "print the claude plugin commands without running them")
	return c
}
