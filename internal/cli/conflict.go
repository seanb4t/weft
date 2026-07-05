// internal/cli/conflict.go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Weft Contributors

package cli

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
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

// resolveAttemptsLabelPrefix carries the crash-durable conflict-resolution
// attempt counter on a bead (spec I4). `conflict open` increments it BEFORE
// opening the workspace so a crash between open and finalize still counts the
// attempt; a healed `finalize` clears it. It follows the jj-change:<id> label
// precedent — bead labels are the only state that survives a mid-resolution crash.
const resolveAttemptsLabelPrefix = "resolve-attempts:"

// resolveAttemptsPattern is deliberately strict: only resolve-attempts:<digits>
// parses. A tampered or malformed value is treated as 0 and overwritten, so a
// bad label can never wedge resolution into permanent escalation or a no-count
// state (a tampered counter must not become a denial-of-resolution vector).
var resolveAttemptsPattern = regexp.MustCompile(`^resolve-attempts:([0-9]+)$`)

// resolveAttemptsFromLabels returns the effective attempt count and every
// resolve-attempts:* label present on the bead (both valid and tampered). The
// count is the MAX of all successfully-parsed counters (0 if none parse,
// including when every present label is tampered) — taking the max means no
// stray low or malformed label (e.g. a coexisting resolve-attempts:0 from a
// tamper, an out-of-band bd update, or a race between two concurrent
// `conflict open`) can pull the effective count below a real attempt count and
// suppress the cap. staleLabels lists ALL prefix-carrying labels so a caller can
// remove every one of them and collapse the bead to a single canonical counter
// on its next write — a tampered counter must never become a denial-of-resolution
// vector. An absent counter yields (0, nil).
func resolveAttemptsFromLabels(labels []string) (int, []string) {
	count := 0
	var staleLabels []string
	for _, l := range labels {
		if !strings.HasPrefix(l, resolveAttemptsLabelPrefix) {
			continue
		}
		staleLabels = append(staleLabels, l)
		m := resolveAttemptsPattern.FindStringSubmatch(l)
		if m == nil {
			continue // tampered/malformed: contributes 0, still removed
		}
		n, err := strconv.Atoi(m[1])
		if err != nil {
			continue // overflow or similar: contributes 0, still removed
		}
		if n > count {
			count = n
		}
	}
	return count, staleLabels
}

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
// rootChange (the healed change + any descendants the squash rebased). finalize
// uses this as the orchestrator's loop-termination gate, so it scopes by subtree
// (descendants) rather than resume's explicit epic-stack union — both avoid
// surfacing conflicts from unrelated epics.
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
			// One shared read: showBead carries both the jj-change spine and the
			// resolve-attempts counter, so open never issues a second `bd show`.
			info, err := showBead(a.Runner, bead)
			if err != nil {
				return err
			}
			change := changeFromLabels(info.Labels)
			if change == "" {
				return exit.Invocationf("bead %s has no jj-change label (not sealed)", bead)
			}
			attempts, staleLabels := resolveAttemptsFromLabels(info.Labels)
			maxAttempts, err := a.Config.MaxResolveAttempts()
			if err != nil {
				return err // invocation error: max_resolve_attempts < 1
			}
			// Oscillation guard (spec I4): at the cap, REFUSE to open another
			// resolution workspace. Flag the bead `human` and emit escalated:true
			// on exit 0 — escalation is an outcome, not an engine error (the same
			// shape as finalize's still-conflicted gate below). This fires before
			// the conflict check: a bead tried this many times terminates, period.
			if attempts >= maxAttempts {
				if res, err := run.BD(a.Runner, "update", bead, "--add-label", "human"); err != nil {
					return exit.Hardf("bd update could not run: %v", err)
				} else if res.Code != 0 {
					return exit.Hardf("bd update %s failed: %s", bead, strings.TrimSpace(res.Stderr))
				}
				data := map[string]any{"bead": bead, "change": change, "escalated": true,
					"attempts": attempts, "workspace": "", "path": ""}
				return Emit(cmd, "conflict.open", data, fmt.Sprintf(
					"escalated %s: %d resolve attempts exhausted (cap %d) — flagged `human`, change %s left for a person",
					bead, attempts, maxAttempts, change))
			}
			// Only open a resolution workspace for an actually-conflicted change.
			conflicted, err := changeConflicted(a.Runner, change)
			if err != nil {
				return err
			}
			if !conflicted {
				return exit.Invocationf("change %s for bead %s is not conflicted — nothing to resolve", change, bead)
			}
			// Increment the counter BEFORE opening the workspace: a crash between
			// here and finalize must still count the attempt (crash-durable — the
			// counter is a bead label, not process state). Drop EVERY prior
			// resolve-attempts:* label (valid, tampered, or a coexisting stray)
			// and add exactly one canonical counter, so the bead is collapsed to a
			// single counter no matter how many labels it carried.
			newCount := attempts + 1
			updateArgs := []string{"update", bead}
			for _, l := range staleLabels {
				updateArgs = append(updateArgs, "--remove-label", l)
			}
			updateArgs = append(updateArgs, "--add-label", fmt.Sprintf("%s%d", resolveAttemptsLabelPrefix, newCount))
			if res, err := run.BD(a.Runner, updateArgs...); err != nil {
				return exit.Hardf("bd update could not run: %v", err)
			} else if res.Code != 0 {
				return exit.Hardf("bd update %s failed: %s", bead, strings.TrimSpace(res.Stderr))
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
			// seam 9: both escalated and attempts ride the normal path too.
			data := map[string]any{"bead": bead, "change": change, "escalated": false,
				"attempts": newCount, "workspace": name, "path": path}
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
			// One shared read: the jj-change spine plus the resolve-attempts
			// counter to clear on a heal.
			info, err := showBead(a.Runner, bead)
			if err != nil {
				return err
			}
			change := changeFromLabels(info.Labels)
			if change == "" {
				return exit.Invocationf("bead %s has no jj-change label (not sealed)", bead)
			}
			_, staleLabels := resolveAttemptsFromLabels(info.Labels)
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
			// Healed: clear EVERY resolve-attempts:* label so a later, unrelated
			// conflict on this bead starts fresh (spec I4 — a heal resets the
			// oscillation guard) and no stray counter survives. Hard-fail like
			// every other bd write here; a best-effort clear could leave a stale
			// counter that false-escalates the next conflict. The escalated gate
			// above returns earlier and deliberately does NOT clear. Gate on the
			// change actually healing (not merely the resolution workspace's `@`
			// being conflict-free): scopedConflictChanges can still report the
			// change's subtree conflicted after the squash, and clearing the
			// counter then would reset the I4 oscillation guard on a change that
			// never fully healed.
			if !remainingSet[change] && len(staleLabels) > 0 {
				updateArgs := []string{"update", bead}
				for _, l := range staleLabels {
					updateArgs = append(updateArgs, "--remove-label", l)
				}
				if res, err := run.BD(a.Runner, updateArgs...); err != nil {
					return exit.Hardf("bd update could not run: %v", err)
				} else if res.Code != 0 {
					return exit.Hardf("bd update %s failed: %s", bead, strings.TrimSpace(res.Stderr))
				}
			}
			// F5: emit escalated:false on the success path so both paths carry the key.
			data := map[string]any{"bead": bead, "change": change, "escalated": false, "healed": healed, "remaining_conflicts": remaining}
			text := fmt.Sprintf("finalized %s (change %s): %d healed, %d conflict(s) remaining",
				bead, change, len(healed), len(remaining))
			return Emit(cmd, "conflict.finalize", data, text)
		},
	}
}
