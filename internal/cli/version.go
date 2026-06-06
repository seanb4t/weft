// internal/cli/version.go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Weft Contributors

package cli

import (
	"runtime/debug"
	"strings"

	"github.com/spf13/cobra"
)

// Version is the engine version. Release builds set it via
//   -ldflags "-X github.com/seanb4t/weft/internal/cli.Version=<X.Y.Z>"
// (GoReleaser injects the tag without its leading "v"). When unset — a
// `go install …@vX.Y.Z` or a local `go build` — it is derived from the module
// build info. The result is a clean "X.Y.Z" for any released build and the
// "0.0.0-dev" sentinel otherwise, so internal/install.semverPattern correctly
// refuses to pin a release tag for dev builds.
var Version string

func init() { Version = resolveVersion(Version, debug.ReadBuildInfo) }

// resolveVersion picks the version: an explicit ldflags value wins (leading "v"
// stripped); else the module version from build info (skipping the "(devel)"
// placeholder); else the dev sentinel.
func resolveVersion(ldflagsVal string, readBuildInfo func() (*debug.BuildInfo, bool)) string {
	if ldflagsVal != "" {
		return strings.TrimPrefix(ldflagsVal, "v")
	}
	if bi, ok := readBuildInfo(); ok && bi.Main.Version != "" && bi.Main.Version != "(devel)" {
		return strings.TrimPrefix(bi.Main.Version, "v")
	}
	return "0.0.0-dev"
}

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the weft version",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return Emit(cmd, "version", map[string]string{"version": Version}, "weft "+Version)
		},
	}
}
