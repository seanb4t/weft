// internal/cli/lines.go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Weft Contributors

package cli

import "strings"

// splitTrimLines splits s on newlines, trims whitespace from each line, and
// returns non-empty lines. The result is always a non-nil slice ([]string{})
// so that JSON marshalling produces [] rather than null for an empty result.
func splitTrimLines(s string) []string {
	out := []string{}
	for _, ln := range strings.Split(strings.TrimSpace(s), "\n") {
		if ln = strings.TrimSpace(ln); ln != "" {
			out = append(out, ln)
		}
	}
	return out
}

// joinLines renders a command list as newline-indented text for the next hint.
func joinLines(xs []string) string { return strings.Join(xs, "\n  ") }
