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

	"github.com/seanb4t/weft/internal/exit"
	"github.com/seanb4t/weft/internal/run"
	"github.com/seanb4t/weft/internal/workspace"
	"github.com/spf13/cobra"
)

func (a *App) newReapCmd() *cobra.Command {
	var epic string
	c := &cobra.Command{
		Use:   "reap",
		Short: "Reconcile jj workspaces against bead state; reap orphans (spec §5)",
		RunE: func(cmd *cobra.Command, _ []string) error {
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
			reaped := []string{}
			for _, name := range splitTrimLines(res.Stdout) {
				if name == "default" {
					continue // never reap the orchestrator's own workspace
				}
				bead, _ := workspace.Resolve(name) // kind-aware: strips -resolve, desanitizes
				// --epic scope: bead-ids are hierarchical, so a descendant of the
				// epic has it as a dotted prefix (e.g. weft-hjx.1.3 under weft-hjx.1).
				if epic != "" && bead != epic && !strings.HasPrefix(bead, epic+".") {
					continue
				}
				status, err := beadStatus(a.Runner, bead)
				if err != nil {
					return err
				}
				// v1: the executor_live guard (spec §5/§10) is deferred. Any
				// in_progress bead is kept — including a CRASHED executor whose
				// bead is still in_progress (it is over-retained, not reaped,
				// until the liveness guard lands). Everything else (closed / open
				// / genuinely-missing) is an orphan and reaped; forget never loses
				// sealed work (spec §2).
				if status == "in_progress" {
					continue
				}
				// Path-safety guard (spec §5): never RemoveAll outside the
				// worktrees root. A jj workspace name carrying "/" or ".." would
				// otherwise let the join escape wtRoot and delete unrelated dirs.
				dir := filepath.Join(wtRoot, name)
				safe, err := workspace.ContainsResolved(wtRoot, dir)
				if err != nil {
					return exit.Hardf("refusing to reap %q: cannot resolve path for containment check: %v", name, err)
				}
				if !safe {
					return exit.Hardf("refusing to reap %q: resolves outside worktrees root %s", name, wtRoot)
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
			data := map[string]any{"reaped": reaped}
			return Emit(cmd, "reap", data, fmt.Sprintf("reaped %d orphan workspace(s)", len(reaped)))
		},
	}
	c.Flags().StringVar(&epic, "epic", "", "scope reconciliation to descendants of this epic")
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
