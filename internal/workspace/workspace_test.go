// internal/workspace/workspace_test.go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Weft Contributors

package workspace

import (
	"path/filepath"
	"testing"
)

func TestSanitizeRoundTrip(t *testing.T) {
	for _, id := range []string{"weft-hjx", "weft-hjx.1", "weft-hjx.1.3"} {
		if got := Desanitize(Sanitize(id)); got != id {
			t.Errorf("round trip %q -> %q -> %q", id, Sanitize(id), got)
		}
	}
	if Sanitize("weft-hjx.1") != "weft-hjx__1" {
		t.Errorf("Sanitize dots wrong: %q", Sanitize("weft-hjx.1"))
	}
}

func TestSanitizeNotBijectiveOutsideBeadIDDomain(t *testing.T) {
	// Documents the domain limitation: an input already containing "__" does
	// not round-trip. Real bead-ids never contain "__", so this is safe in
	// practice, but the doc must not over-claim unconditional bijectivity.
	if got := Desanitize(Sanitize("a__b")); got == "a__b" {
		t.Errorf("expected non-round-trip for %q, got %q", "a__b", got)
	}
}

func TestResolveKind(t *testing.T) {
	bead, kind := Resolve("weft-hjx__1")
	if bead != "weft-hjx.1" || kind != KindExecutor {
		t.Errorf("executor: got (%q,%q)", bead, kind)
	}
	bead, kind = Resolve("weft-hjx__1-resolve")
	if bead != "weft-hjx.1" || kind != KindResolve {
		t.Errorf("resolve: got (%q,%q)", bead, kind)
	}
}

func TestContains(t *testing.T) {
	root := "/a/b/weft_worktrees"
	// Inside → true.
	if !Contains(root, root+"/weft-hjx__1") {
		t.Errorf("path inside root must be contained")
	}
	if !Contains(root, root) {
		t.Errorf("root itself must be contained")
	}
	// Escapes via ".." → false.
	if Contains(root, root+"/../../etc") {
		t.Errorf("path escaping root must NOT be contained")
	}
	if Contains(root, "/a/b/other") {
		t.Errorf("sibling path must NOT be contained")
	}
}

func TestRootAndPath(t *testing.T) {
	// Default: sibling of the repo dir.
	if got := Root("/a/b/weft", ""); got != "/a/b/weft_worktrees" {
		t.Errorf("default Root = %q", got)
	}
	// Relative override resolves against the repo root.
	if got := Root("/a/b/weft", "../wt"); got != "/a/b/wt" {
		t.Errorf("relative Root = %q", got)
	}
	// Absolute override used as-is.
	if got := Root("/a/b/weft", "/tmp/wt"); got != "/tmp/wt" {
		t.Errorf("absolute Root = %q", got)
	}
	if got := Path("/a/b/weft", "", "weft-hjx.1"); got != "/a/b/weft_worktrees/weft-hjx__1" {
		t.Errorf("Path = %q", got)
	}
}

func TestResolveNameAndPath(t *testing.T) {
	name := ResolveName("weft-hjx.4.2")
	if name != "weft-hjx__4__2-resolve" {
		t.Fatalf("ResolveName = %q, want weft-hjx__4__2-resolve", name)
	}
	// Round-trips through the existing kind-aware Resolve.
	bead, kind := Resolve(name)
	if bead != "weft-hjx.4.2" || kind != KindResolve {
		t.Fatalf("Resolve(%q) = %q,%v; want weft-hjx.4.2,resolve", name, bead, kind)
	}
	p := ResolvePath("/repo", "", "weft-hjx.4.2")
	if filepath.Base(p) != name {
		t.Fatalf("ResolvePath base = %q, want %q", filepath.Base(p), name)
	}
	// Same worktrees root as an executor workspace, different leaf.
	if filepath.Dir(p) != filepath.Dir(Path("/repo", "", "weft-hjx.4.2")) {
		t.Fatalf("ResolvePath root = %q, want same as Path root", filepath.Dir(p))
	}
}

// TestContainsRejectsEscape (F7): Contains must reject paths that escape the
// worktrees root via "../" traversal. This is defense-in-depth for the
// os.RemoveAll guards in finalize and shed cleanup (ResolvePath always yields
// an in-root path, but the guard catches any future refactor that might not).
func TestContainsRejectsEscape(t *testing.T) {
	if Contains("/repo/wt", "/repo/wt/../evil") {
		t.Errorf("Contains must reject path escaping root via ../")
	}
	if !Contains("/repo/wt", "/repo/wt/ok") {
		t.Errorf("Contains must accept path inside root")
	}
}
