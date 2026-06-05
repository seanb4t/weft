// internal/workspace/workspace_test.go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Weft Contributors

package workspace

import (
	"os"
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

// TestContainsResolved verifies the symlink-aware containment helper used by all
// destructive remove sites. Four cases:
//  1. in-root real dir → true, nil.
//  2. symlink inside root whose target is OUTSIDE root → false, nil (the escape).
//  3. non-existent child under root → true, nil (lexical fallback; RemoveAll is no-op).
//  4. non-existent parent → non-nil error (cannot resolve parent).
func TestContainsResolved(t *testing.T) {
	// Case 1: real in-root directory → true, nil.
	root := t.TempDir()
	child := filepath.Join(root, "ws1")
	if err := os.MkdirAll(child, 0o755); err != nil {
		t.Fatalf("mkdir child: %v", err)
	}
	got, err := ContainsResolved(root, child)
	if err != nil {
		t.Fatalf("case 1 (real in-root): unexpected error: %v", err)
	}
	if !got {
		t.Errorf("case 1 (real in-root): want true, got false")
	}

	// Case 2: symlink INSIDE root whose target is OUTSIDE root → false, nil.
	outside := t.TempDir() // completely separate temp dir
	link := filepath.Join(root, "evil")
	if err := os.Symlink(outside, link); err != nil {
		t.Fatalf("symlink: %v", err)
	}
	got, err = ContainsResolved(root, link)
	if err != nil {
		t.Fatalf("case 2 (symlink escape): unexpected error: %v", err)
	}
	if got {
		t.Errorf("case 2 (symlink escape): want false (symlink points outside root), got true")
	}

	// Case 3: non-existent child under root → true, nil (lexical fallback).
	nonExist := filepath.Join(root, "does-not-exist")
	got, err = ContainsResolved(root, nonExist)
	if err != nil {
		t.Fatalf("case 3 (non-existent child): unexpected error: %v", err)
	}
	if !got {
		t.Errorf("case 3 (non-existent child): want true (lexical fallback), got false")
	}

	// Case 4: non-existent parent → non-nil error.
	_, err = ContainsResolved("/no/such/parent", filepath.Join("/no/such/parent", "child"))
	if err == nil {
		t.Errorf("case 4 (non-existent parent): want non-nil error, got nil")
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
