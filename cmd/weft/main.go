// cmd/weft/main.go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Weft Contributors

package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/seanb4t/weft/internal/cli"
	"github.com/seanb4t/weft/internal/config"
	"github.com/seanb4t/weft/internal/exit"
	"github.com/seanb4t/weft/internal/run"
)

func main() {
	cfg, err := config.Load(filepath.Join(".weft", "config.toml"))
	if err != nil {
		fmt.Fprintln(os.Stderr, "weft: invalid .weft/config.toml:", err)
		os.Exit(exit.Code(exit.Hard(err)))
	}
	root := cli.NewRootCmd(cli.NewApp(run.Exec{}, cfg))
	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(exit.Code(err))
	}
}
