// internal/workspace/workspace.go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Weft Contributors

// Package workspace derives jj workspace identity and on-disk paths for a bead
// (spec §3). The jj workspace name is a sanitized bead-id; the inverse mapping
// is the reaper's join key (spec §5).
package workspace

import (
	"path/filepath"
	"strings"
)

// resolveSuffix marks a conflict-resolution workspace (seam 4 §4.1). Bead-ids
// never end in "-resolve", so it is an unambiguous kind discriminant.
const resolveSuffix = "-resolve"

// Kind classifies a workspace by purpose: an executor workspace runs a pick,
// a resolve workspace hosts a conflict resolution (seam 4 §4.1).
type Kind string

const (
	KindExecutor Kind = "executor"
	KindResolve  Kind = "resolve"
)

// Sanitize maps a bead-id to a jj-safe workspace name (dots → "__"). It is
// bijective over the bead-id domain — ids look like weft-<hash>.<n>... and never
// contain "__", so Desanitize inverts it for any id Sanitize produced. The
// mapping is NOT bijective for arbitrary strings (an input already containing
// "__" would round-trip to a different value).
func Sanitize(beadID string) string { return strings.ReplaceAll(beadID, ".", "__") }

// Desanitize is the inverse of Sanitize over the bead-id domain (see Sanitize).
func Desanitize(name string) string { return strings.ReplaceAll(name, "__", ".") }

// Name returns the executor-workspace name for a bead.
func Name(beadID string) string { return Sanitize(beadID) }

// Resolve classifies a workspace name and returns its owning bead-id and Kind.
func Resolve(name string) (beadID string, kind Kind) {
	if strings.HasSuffix(name, resolveSuffix) {
		return Desanitize(strings.TrimSuffix(name, resolveSuffix)), KindResolve
	}
	return Desanitize(name), KindExecutor
}

// Root returns the sibling worktrees directory for the repo at jjRoot. A
// non-empty cfgRoot overrides the default ../<repo>_worktrees; a relative
// cfgRoot resolves against jjRoot.
func Root(jjRoot, cfgRoot string) string {
	if cfgRoot != "" {
		if filepath.IsAbs(cfgRoot) {
			return filepath.Clean(cfgRoot)
		}
		return filepath.Clean(filepath.Join(jjRoot, cfgRoot))
	}
	return filepath.Join(filepath.Dir(jjRoot), filepath.Base(jjRoot)+"_worktrees")
}

// Path returns the absolute workspace directory for a bead.
func Path(jjRoot, cfgRoot, beadID string) string {
	return filepath.Join(Root(jjRoot, cfgRoot), Name(beadID))
}

// ResolveName returns the resolution-workspace name for a bead (seam 4 §4.1):
// the executor name plus the -resolve suffix that marks the second kind. Resolve
// inverts it back to the owning bead-id + KindResolve.
func ResolveName(beadID string) string { return Name(beadID) + resolveSuffix }

// ResolvePath returns the absolute resolution-workspace directory for a bead —
// the same worktrees root as Path, with the -resolve leaf.
func ResolvePath(jjRoot, cfgRoot, beadID string) string {
	return filepath.Join(Root(jjRoot, cfgRoot), ResolveName(beadID))
}

// Contains reports whether child resolves to a path at or inside parent. The
// destructive verbs (ws forget, shed cleanup, weft reap) guard their
// os.RemoveAll target with it so a bead-id or jj workspace name carrying "/" or
// ".." cannot escape the worktrees root and delete unrelated directories.
func Contains(parent, child string) bool {
	rel, err := filepath.Rel(filepath.Clean(parent), filepath.Clean(child))
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
