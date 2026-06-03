// internal/cli/root.go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Weft Contributors

// Package cli wires the weft verb surface (spec §4) onto cobra. Verbs reach
// bd/jj through App.Runner so they are unit-testable with a fake.
package cli

import (
	"github.com/seanb4t/weft/internal/config"
	"github.com/seanb4t/weft/internal/exit"
	"github.com/seanb4t/weft/internal/run"
	"github.com/spf13/cobra"
)

// App holds the engine's injectable dependencies.
type App struct {
	Runner run.Runner
	Config config.Config
}

// NewRootCmd builds the weft root command and its verb tree.
func NewRootCmd(app *App) *cobra.Command {
	root := &cobra.Command{
		Use:           "weft",
		Short:         "Weft — spec-driven AI dev orchestration on beads + jj",
		SilenceUsage:  true, // don't dump usage on every RunE error
		SilenceErrors: true, // main prints errors and sets the exit code
	}
	// Output contract (spec §3): default text, --json envelope, --pick field.
	root.PersistentFlags().Bool("json", false, "emit the uniform JSON envelope")
	root.PersistentFlags().String("pick", "", "extract one field by path (e.g. data.wave[0])")
	// Map cobra flag-parse errors to exit code 1 (invocation error).
	root.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		return exit.Invocation(err)
	})

	root.AddCommand(newVersionCmd())
	root.AddCommand(app.newShedCmd())
	root.AddCommand(app.newWsCmd())
	root.AddCommand(app.newReapCmd())
	root.AddCommand(app.newPickCmd())
	return root
}
