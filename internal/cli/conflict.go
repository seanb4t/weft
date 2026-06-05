// internal/cli/conflict.go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Weft Contributors

package cli

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/seanb4t/weft/internal/exit"
	"github.com/seanb4t/weft/internal/run"
	"github.com/seanb4t/weft/internal/workspace"
	"github.com/spf13/cobra"
)

// changeIDPattern matches a bare jj change-id. jj renders change-ids in a
// lowercase alphabet; restricting to [a-z0-9] excludes every revset
// metacharacter (& | : . ( ) ~ and whitespace), so a tampered jj-change
// label cannot alter revset evaluation when interpolated.
var changeIDPattern = regexp.MustCompile(`^[a-z0-9]+$`)

// workspaceRevPattern matches a "<workspace-name>@" working-copy reference,
// where the name is a Sanitize()d bead-id ([a-z0-9_-]); it likewise excludes
// revset metacharacters apart from the trailing '@' addressing operator.
var workspaceRevPattern = regexp.MustCompile(`^[a-z0-9_-]+@$`)

func (a *App) newConflictCmd() *cobra.Command {
	c := &cobra.Command{Use: "conflict", Short: "Conflict-resolution choreography (spec seam 4)"}
	c.AddCommand(a.newConflictOpenCmd(), a.newConflictFinalizeCmd())
	return c
}

// changeConflicted reports whether a revision is in jj's conflicts() set. The
// revision may be a change-id or a <workspace-name>@ working-copy reference.
func changeConflicted(r run.Runner, rev string) (bool, error) {
	if !changeIDPattern.MatchString(rev) && !workspaceRevPattern.MatchString(rev) {
		return false, exit.Hardf("refusing to interpolate unsafe revision %q into a revset", rev)
	}
	res, err := run.JJ(r, "log", "-r", "conflicts() & "+rev, "--no-graph", "-T", `change_id.short(12) ++ "\n"`)
	if err != nil {
		return false, exit.Hardf("jj conflicts check could not run: %v", err)
	}
	if res.Code != 0 {
		return false, exit.Hardf("jj conflicts check failed: %s", strings.TrimSpace(res.Stderr))
	}
	return strings.TrimSpace(res.Stdout) != "", nil
}

// scopedConflictChanges lists conflicted change-ids within the subtree rooted at
// rootChange (the healed change + any descendants the squash rebased). Unlike
// resume's repo-wide conflictChanges, finalize uses this as the orchestrator's
// loop-termination gate, so it must not surface conflicts from unrelated epics.
func scopedConflictChanges(r run.Runner, rootChange string) ([]string, error) {
	if !changeIDPattern.MatchString(rootChange) {
		return nil, exit.Hardf("refusing to interpolate unsafe revision %q into a revset", rootChange)
	}
	res, err := run.JJ(r, "log", "-r", "conflicts() & descendants("+rootChange+")", "--no-graph", "-T", `change_id.short(12) ++ "\n"`)
	if err != nil {
		return nil, exit.Hardf("jj scoped conflicts check could not run: %v", err)
	}
	if res.Code != 0 {
		return nil, exit.Hardf("jj scoped conflicts check failed: %s", strings.TrimSpace(res.Stderr))
	}
	return splitTrimLines(res.Stdout), nil
}

func (a *App) newConflictOpenCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "open <bead>",
		Short: "Open a resolution workspace on a conflicted pick + emit the resolver brief",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			bead := args[0]
			change, err := changeOf(a.Runner, bead)
			if err != nil {
				return err
			}
			if change == "" {
				return exit.Invocationf("bead %s has no jj-change label (not sealed)", bead)
			}
			// Only open a resolution workspace for an actually-conflicted change.
			conflicted, err := changeConflicted(a.Runner, change)
			if err != nil {
				return err
			}
			if !conflicted {
				return exit.Invocationf("change %s for bead %s is not conflicted — nothing to resolve", change, bead)
			}
			root, err := jjRoot(a.Runner)
			if err != nil {
				return err
			}
			name := workspace.ResolveName(bead)
			path := workspace.ResolvePath(root, a.Config.Workspace.Root, bead)
			// Resolution workspace: -r <change> makes its @ a CHILD of the conflicted
			// change, so the conflict materializes there for the resolver to edit
			// (spec §3 "jj new L in a resolution workspace").
			if res, err := run.JJ(a.Runner, "workspace", "add", path, "--name", name, "-r", change); err != nil {
				return exit.Hardf("jj workspace add could not run: %v", err)
			} else if res.Code != 0 {
				return exit.Hardf("jj workspace add %s failed: %s", name, strings.TrimSpace(res.Stderr))
			}
			// Pin diff marker style — the only built-in style that represents 3+-sided
			// conflicts natively (§5). Repo-scoped; per-workspace pinning is a §8 refinement.
			if res, err := run.JJ(a.Runner, "config", "set", "--repo", "ui.conflict-marker-style", "diff"); err != nil {
				return exit.Hardf("jj config set could not run: %v", err)
			} else if res.Code != 0 {
				return exit.Hardf("jj config set failed: %s", strings.TrimSpace(res.Stderr))
			}
			data := map[string]any{"bead": bead, "change": change, "workspace": name, "path": path}
			// F1: spec §5 prohibits `jj resolve` in agent context; use `jj st` instead.
			text := fmt.Sprintf(
				"opened resolution workspace %s for %s (change %s) at %s\n"+
					"resolver: edit the conflict markers in that workspace (run `jj st` to list the conflicted paths), remove them, then `weft conflict finalize %s`",
				name, bead, change, path, bead)
			return Emit(cmd, "conflict.open", data, text)
		},
	}
}

func (a *App) newConflictFinalizeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "finalize <bead>",
		Short: "Fold the resolver's edits into the conflicted change, heal descendants, reap (or escalate)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			bead := args[0]
			change, err := changeOf(a.Runner, bead)
			if err != nil {
				return err
			}
			if change == "" {
				return exit.Invocationf("bead %s has no jj-change label (not sealed)", bead)
			}
			root, err := jjRoot(a.Runner)
			if err != nil {
				return err
			}
			name := workspace.ResolveName(bead)
			path := workspace.ResolvePath(root, a.Config.Workspace.Root, bead)
			wsRev := name + "@" // jj addresses a workspace's working copy as <name>@

			// F3: finalize requires a prior `conflict open` — the resolution workspace
			// must exist on disk. Without this, changeConflicted(wsRev) fails with a
			// cryptic jj error on an unknown <name>@ reference.
			if _, statErr := os.Stat(path); os.IsNotExist(statErr) {
				return exit.Invocationf("no resolution workspace for %s at %s — run `weft conflict open %s` first", bead, path, bead)
			}

			// Escalation gate (§6): the resolver must have removed the markers. If
			// the resolution workspace's @ is still conflicted, do NOT squash —
			// flag the bead with the `human` label and leave the change conflicted.
			// (`bd human` only lists/responds/dismisses; the flag IS the label.)
			stillConflicted, err := changeConflicted(a.Runner, wsRev)
			if err != nil {
				return err
			}
			if stillConflicted {
				if res, err := run.BD(a.Runner, "update", bead, "--add-label", "human"); err != nil {
					return exit.Hardf("bd update could not run: %v", err)
				} else if res.Code != 0 {
					return exit.Hardf("bd update %s failed: %s", bead, strings.TrimSpace(res.Stderr))
				}
				data := map[string]any{
					"bead": bead, "change": change, "escalated": true,
					"healed": []string{}, "remaining_conflicts": []string{change},
				}
				return Emit(cmd, "conflict.finalize", data,
					fmt.Sprintf("escalated %s: resolution still conflicted — flagged `human`, change %s left for a person", bead, change))
			}

			// The resolution must be non-empty (the resolver actually edited the
			// markers) before we fold it in.
			res, err := run.JJ(a.Runner, "diff", "--git", "-r", wsRev)
			if err != nil {
				return exit.Hardf("jj diff could not run: %v", err)
			}
			if res.Code != 0 {
				return exit.Hardf("jj diff failed: %s", strings.TrimSpace(res.Stderr))
			}
			if strings.TrimSpace(res.Stdout) == "" {
				return exit.Invocationf("resolution workspace %s has no changes — resolver did not edit the markers", name)
			}

			// Fold the resolution into the conflicted change; jj auto-rebases and
			// conflict-simplifies descendants, so one resolution heals the stack (§2.1).
			if res, err := run.JJ(a.Runner, "squash", "--from", wsRev, "--into", change); err != nil {
				return exit.Hardf("jj squash could not run: %v", err)
			} else if res.Code != 0 {
				return exit.Hardf("jj squash failed: %s", strings.TrimSpace(res.Stderr))
			}

			// Reap the resolution workspace (seam 3 mechanics + path-safety guard).
			// Defense-in-depth: ResolvePath always yields an in-root path, but guard
			// against any future refactor that might produce an out-of-root path.
			wtRoot := workspace.Root(root, a.Config.Workspace.Root)
			safe, err := workspace.ContainsResolved(wtRoot, path)
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
			if err := os.RemoveAll(path); err != nil {
				return exit.Hardf("rm resolution workspace %s: %v", path, err)
			}

			// F2: Re-query conflicts() scoped to the change's subtree (not repo-wide).
			// finalize uses remaining_conflicts as the orchestrator's loop-termination
			// gate, so the repo-wide scope is a bug — use scopedConflictChanges instead.
			remaining, err := scopedConflictChanges(a.Runner, change)
			if err != nil {
				return err
			}
			remainingSet := map[string]bool{}
			for _, c := range remaining {
				remainingSet[c] = true
			}
			healed := []string{}
			if !remainingSet[change] {
				healed = append(healed, change)
			}
			// F5: emit escalated:false on the success path so both paths carry the key.
			data := map[string]any{"bead": bead, "change": change, "escalated": false, "healed": healed, "remaining_conflicts": remaining}
			text := fmt.Sprintf("finalized %s (change %s): %d healed, %d conflict(s) remaining",
				bead, change, len(healed), len(remaining))
			return Emit(cmd, "conflict.finalize", data, text)
		},
	}
}
