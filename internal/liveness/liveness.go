// internal/liveness/liveness.go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Weft Contributors

// Package liveness answers one question from existing state only: when was
// this workspace last worked? No PID files, no heartbeats — the engine never
// spawns agents, so it trusts only signals it can observe (spec: decision 3).
package liveness

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/seanb4t/weft/internal/run"
)

// wsNamePattern matches a Sanitize()d workspace name ([a-z0-9_-]); it excludes
// every revset metacharacter, so a name cannot alter revset evaluation when
// interpolated as <name>@ (same rationale as cli's workspaceRevPattern).
var wsNamePattern = regexp.MustCompile(`^[a-z0-9_-]+$`)

// tsLayout parses the explicit strftime format requested in the template
// below — locale-independent, unlike jj's default timestamp rendering.
const tsLayout = "2006-01-02T15:04:05-0700"

// LastActivity returns the most recent evidence of executor work in a
// workspace: the committer timestamp of the workspace's working-copy commit
// (jj refreshes it on every per-workspace snapshot, i.e. every jj command run
// there) joined with the newest file mtime under the workspace directory
// (guards the edited-files-but-ran-no-jj-command window). The .jj directory
// is excluded from the walk — jj's own bookkeeping is not executor activity.
// A missing/unwalkable directory contributes nothing (the jj signal stands);
// a failing jj call is an infrastructure anomaly and errors — callers decide
// policy (reap hard-fails; doctor may degrade).
func LastActivity(r run.Runner, wsName, wsDir string) (time.Time, error) {
	if !wsNamePattern.MatchString(wsName) {
		return time.Time{}, fmt.Errorf("refusing to interpolate unsafe workspace name %q into a revset", wsName)
	}
	res, err := run.JJ(r, "log", "--no-graph", "-r", wsName+"@",
		"-T", `committer.timestamp().format("%Y-%m-%dT%H:%M:%S%z") ++ "\n"`)
	if err != nil {
		return time.Time{}, fmt.Errorf("jj log %s@ could not run: %w", wsName, err)
	}
	if res.Code != 0 {
		return time.Time{}, fmt.Errorf("jj log %s@ failed: %s", wsName, strings.TrimSpace(res.Stderr))
	}
	last, err := time.Parse(tsLayout, strings.TrimSpace(res.Stdout))
	if err != nil {
		return time.Time{}, fmt.Errorf("parse jj timestamp %q: %w", strings.TrimSpace(res.Stdout), err)
	}
	// Join with the newest file mtime under the workspace dir (best-effort
	// walk). Directory mtimes are excluded: creating or removing any child —
	// including jj's own bookkeeping under .jj — bumps the parent directory's
	// mtime, so only regular files are trustworthy evidence of executor edits.
	_ = filepath.WalkDir(wsDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // unreadable entries contribute nothing
		}
		if d.IsDir() {
			if d.Name() == ".jj" {
				return filepath.SkipDir
			}
			return nil
		}
		if info, ierr := d.Info(); ierr == nil && info.ModTime().After(last) {
			last = info.ModTime()
		}
		return nil
	})
	return last, nil
}

// Live reports whether activity at t is within threshold of now.
func Live(t, now time.Time, threshold time.Duration) bool {
	return now.Sub(t) <= threshold
}
