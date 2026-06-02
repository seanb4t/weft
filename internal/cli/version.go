// internal/cli/version.go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Weft Contributors

package cli

import "github.com/spf13/cobra"

// Version is the engine version (overridable via -ldflags at build time later).
const Version = "0.0.0-dev"

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the weft version",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return Emit(cmd, "version", map[string]string{"version": Version}, "weft "+Version)
		},
	}
}
