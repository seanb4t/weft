// cmd/weft/main.go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Weft Contributors

package main

import (
	"fmt"
	"os"

	"github.com/seanb4t/weft/internal/cli"
	"github.com/seanb4t/weft/internal/exit"
	"github.com/seanb4t/weft/internal/run"
)

func main() {
	root := cli.NewRootCmd(&cli.App{Runner: run.Exec{}})
	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(exit.Code(err))
	}
}
