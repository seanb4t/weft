// internal/workspace/workspace.go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Weft Contributors

// Package workspace derives jj workspace identity and on-disk paths for a bead
// (spec §3). The jj workspace name is a sanitized bead-id; the inverse mapping
// is the reaper's join key (spec §5).
package workspace

import (
	"errors"
	"io/fs"
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

// ContainsResolved reports whether child, after resolving symlinks, is at or
// inside parent (also symlink-resolved). Destructive verbs (ws forget, shed
// cleanup, reap, conflict finalize) MUST use this instead of the lexical
// Contains before os.RemoveAll, so a symlink planted at the expected path
// cannot redirect the delete outside the worktrees root. If child does not
// exist there is nothing to follow or delete, so it falls back to a lexical
// check against the resolved parent (the subsequent RemoveAll is a no-op). Any
// other resolution error is returned and callers MUST treat it as "refuse".
func ContainsResolved(parent, child string) (bool, error) {
	realParent, err := filepath.EvalSymlinks(parent)
	if err != nil {
		return false, err
	}
	realChild, err := filepath.EvalSymlinks(child)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			// Child does not exist — nothing will be followed or deleted.
			// Resolve as much as we can: evaluate the parent directory and
			// re-attach the base name so the lexical check uses real paths.
			resolvedDir, dirErr := filepath.EvalSymlinks(filepath.Dir(filepath.Clean(child)))
			if dirErr != nil {
				// Parent dir also doesn't exist; fall back to clean lexical child.
				return Contains(realParent, filepath.Clean(child)), nil
			}
			return Contains(realParent, filepath.Join(resolvedDir, filepath.Base(child))), nil
		}
		return false, err
	}
	return Contains(realParent, realChild), nil
}
