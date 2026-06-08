// internal/weave/agents_test.go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Weft Contributors

//go:build integration

package weave_test

import (
	"os"
	"path/filepath"
	"testing"
)

// pickFiles returns the files a ref's executor writes into its workspace.
// p2a/p2b collide on collide_h.txt; p4a/p4b collide on collide_e.txt
// (add/add, different content → first-class conflict at integrate).
func pickFiles(ref string) map[string]string {
	switch ref {
	case "p1":
		return map[string]string{"p1.txt": "p1\n"}
	case "p2a":
		return map[string]string{"collide_h.txt": "heal-A\n"}
	case "p2b":
		return map[string]string{"collide_h.txt": "heal-B\n"}
	case "p3":
		return map[string]string{"p3.txt": "p3\n"}
	case "p4a":
		return map[string]string{"collide_e.txt": "esc-A\n"}
	case "p4b":
		return map[string]string{"collide_e.txt": "esc-B\n"}
	}
	return nil
}

// scriptedExecutor writes the ref's content into the bead's workspace. When
// failVerify is true it also drops the .weft-verify-fail marker so the verify
// gate (`test ! -f .weft-verify-fail`) reports data.pass:false.
func (r *scratchRepo) scriptedExecutor(t *testing.T, ws, ref string, failVerify bool) {
	t.Helper()
	for name, content := range pickFiles(ref) {
		if err := os.WriteFile(filepath.Join(ws, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	marker := filepath.Join(ws, ".weft-verify-fail")
	if failVerify {
		if err := os.WriteFile(marker, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	} else {
		_ = os.Remove(marker) // ensure absent on the passing (re)try
	}
}

// scriptedResolver acts in the resolution workspace opened by `conflict open`.
// heal=true: rewrite the conflicted file with a clean merged body (markers
// removed) so finalize squashes+heals. heal=false: leave the workspace
// untouched so finalize sees a still-conflicted @ and escalates (human label).
func (r *scratchRepo) scriptedResolver(t *testing.T, resolveDir, conflictedFile string, heal bool) {
	t.Helper()
	if !heal {
		return // leave markers → escalation path
	}
	if err := os.WriteFile(filepath.Join(resolveDir, conflictedFile), []byte("resolved\n"), 0o600); err != nil {
		t.Fatal(err)
	}
}
