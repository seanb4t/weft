// internal/cli/stubs.go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Weft Contributors

package cli

import "github.com/spf13/cobra"

// TEMPORARY stubs so the root command compiles. Replaced by the real
// shed/ws verbs in Tasks 6–7 (this file is deleted then).

func (app *App) newShedCmd() *cobra.Command { return &cobra.Command{Use: "shed"} }

func (app *App) newWsCmd() *cobra.Command { return &cobra.Command{Use: "ws"} }
