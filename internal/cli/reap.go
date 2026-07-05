// internal/cli/reap.go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Weft Contributors

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/seanb4t/weft/internal/exit"
	"github.com/seanb4t/weft/internal/liveness"
	"github.com/seanb4t/weft/internal/run"
	"github.com/seanb4t/weft/internal/workspace"
	"github.com/spf13/cobra"
)

func (a *App) newReapCmd() *cobra.Command {
	var epic string
	var dryRun bool
	c := &cobra.Command{
		Use:   "reap",
		Short: "Reconcile jj workspaces against bead state; reap orphans (spec §5)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			threshold, err := a.Config.LivenessThreshold()
			if err != nil {
				return exit.Invocationf("[liveness] threshold: %v", err)
			}
			root, err := jjRoot(a.Runner)
			if err != nil {
				return err
			}
			res, err := run.JJ(a.Runner, "workspace", "list", "-T", `name ++ "\n"`)
			if err != nil {
				return exit.Hardf("jj workspace list could not run: %v", err)
			}
			if res.Code != 0 {
				return exit.Hardf("jj workspace list failed: %s", strings.TrimSpace(res.Stderr))
			}
			wtRoot := workspace.Root(root, a.Config.Workspace.Root)
			// Envelope keys are always []-initialized (never null) — seam 9
			// discipline. reaped/wouldReap carry bead-ids; foreign carries the raw
			// workspace names of skipped non-weft workspaces (doctor reports them).
			reaped := []string{}
			wouldReap := []string{}
			foreign := []string{}
			for _, name := range splitTrimLines(res.Stdout) {
				if name == "default" {
					continue // never reap the orchestrator's own workspace
				}
				bead, _ := workspace.Resolve(name) // kind-aware: strips -resolve, desanitizes
				// --epic scope: bead-ids are hierarchical, so a descendant of the
				// epic has it as a dotted prefix (e.g. weft-hjx.1.3 under weft-hjx.1).
				if !scopedToEpic(epic, bead) {
					continue
				}
				status, err := beadStatus(a.Runner, bead)
				if err != nil {
					return err
				}
				// dir is the sole declaration (moved above the path-safety guard so
				// it feeds the foreign and liveness checks).
				dir := filepath.Join(wtRoot, name)
				if status == "" && !dirExists(dir) {
					// Foreign: resolves to no bead AND lives outside the worktrees
					// root (weft creates every workspace it owns under wtRoot).
					// Forgetting it would break whoever owns it (e.g. a Claude Code
					// worktree-agent-* session). Doctor reports these; reap skips.
					// A beadless workspace whose dir IS under wtRoot is a genuine
					// weft orphan (its bead was deleted) and still falls through to
					// reap below.
					foreign = append(foreign, name)
					continue
				}
				// executor_live decision table (spec §5/§10, invariant I3): an
				// in_progress bead is kept only while its executor is LIVE; an
				// in_progress bead gone quiet past the threshold is a CRASHED
				// executor and falls through to reap. Everything else (closed /
				// open / beadless-with-dir) is an orphan; forget never loses sealed
				// work (spec §2).
				if status == "in_progress" {
					last, err := liveness.LastActivity(a.Runner, name, dir)
					if err != nil {
						return exit.Hardf("liveness probe for %s: %v", name, err)
					}
					if liveness.Live(last, time.Now(), threshold) {
						continue // busy — the seam 3 §5 authoritative guard
					}
					// crashed: in_progress but dead past threshold → fall through to reap
				}
				// Path-safety guard (spec §5): never RemoveAll outside the
				// worktrees root. A jj workspace name carrying "/" or ".." would
				// otherwise let the join escape wtRoot and delete unrelated dirs.
				safe, err := workspace.ContainsResolved(wtRoot, dir)
				if err != nil {
					return exit.Hardf("refusing to reap %q: cannot resolve path for containment check: %v", name, err)
				}
				if !safe {
					return exit.Hardf("refusing to reap %q: resolves outside worktrees root %s", name, wtRoot)
				}
				if dryRun {
					wouldReap = append(wouldReap, bead) // report only, mutate nothing
					continue
				}
				if res, err := run.JJ(a.Runner, "workspace", "forget", name); err != nil {
					return exit.Hardf("jj workspace forget could not run: %v", err)
				} else if res.Code != 0 {
					return exit.Hardf("jj workspace forget %s failed: %s", name, strings.TrimSpace(res.Stderr))
				}
				if err := os.RemoveAll(dir); err != nil {
					return exit.Hardf("rm workspace dir %s: %v", dir, err)
				}
				reaped = append(reaped, bead) // emit the bead-id, matching shed.isolate/cleanup
			}
			data := map[string]any{"reaped": reaped, "would_reap": wouldReap, "foreign": foreign, "dry_run": dryRun}
			summary := fmt.Sprintf("reaped %d orphan workspace(s)", len(reaped))
			if dryRun {
				summary = fmt.Sprintf("dry-run: would reap %d orphan workspace(s)", len(wouldReap))
			}
			return Emit(cmd, "reap", data, summary)
		},
	}
	c.Flags().StringVar(&epic, "epic", "", "scope reconciliation to descendants of this epic")
	c.Flags().BoolVar(&dryRun, "dry-run", false, "report workspaces that would be reaped without mutating anything")
	return c
}

// beadStatus returns the bead's status, or "" when the bead genuinely does not
// exist (its workspace is then an orphan the caller may reap).
//
// reap is destructive (forget + RemoveAll), so beadStatus is fail-safe: it
// returns "" ONLY when bd definitively reports the bead missing — either a
// zero-exit empty result, or a non-zero exit whose error payload says "no
// issue(s) found". Any OTHER non-zero exit (Dolt unreachable, bd crash) or a
// zero-exit-but-unparseable body is an infrastructure anomaly, NOT a missing
// bead, and hard-fails rather than letting a transient glitch reap live work.
func beadStatus(r run.Runner, bead string) (string, error) {
	res, err := run.BD(r, "show", bead, "--json")
	if err != nil {
		return "", exit.Hardf("bd show could not run: %v", err)
	}
	if res.Code != 0 {
		if beadNotFound(res.Stdout) {
			return "", nil // bead genuinely gone → its workspace is an orphan
		}
		return "", exit.Hardf("bd show %s failed (treating as infrastructure error, not an orphan): %s",
			bead, strings.TrimSpace(firstNonEmpty(res.Stderr, res.Stdout)))
	}
	var arr []struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal([]byte(res.Stdout), &arr); err != nil {
		return "", exit.Hardf("bd show returned malformed JSON for %s: %v", bead, err)
	}
	if len(arr) == 0 {
		return "", nil // bd found nothing → its workspace is an orphan
	}
	return arr[0].Status, nil
}

// beadNotFound reports whether a non-zero `bd show --json` body is the
// recognized "bead does not exist" error (as opposed to an infrastructure
// failure). bd emits {"error": "no issues found ..."} for a missing id.
func beadNotFound(stdout string) bool {
	var e struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal([]byte(stdout), &e); err != nil {
		return false // unrecognized / non-JSON body → not a definitive not-found
	}
	return strings.Contains(e.Error, "no issue")
}

// firstNonEmpty returns the first non-blank string, used to surface whichever of
// stderr/stdout carries the error detail.
func firstNonEmpty(a, b string) string {
	if strings.TrimSpace(a) != "" {
		return a
	}
	return b
}
