// internal/cli/shed.go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Weft Contributors

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/seanb4t/weft/internal/exit"
	"github.com/seanb4t/weft/internal/run"
	"github.com/seanb4t/weft/internal/workspace"
	"github.com/spf13/cobra"
)

func (a *App) newShedCmd() *cobra.Command {
	shed := &cobra.Command{Use: "shed", Short: "Wave-level orchestration (spec §4.1)"}
	shed.AddCommand(a.newShedFormCmd(), a.newShedIsolateCmd(), a.newShedCleanupCmd(), a.newShedIntegrateCmd())
	return shed
}

func (a *App) newShedFormCmd() *cobra.Command {
	var epic string
	var max int
	c := &cobra.Command{
		Use:   "form",
		Short: "Form a shed: the ready wave for an epic (bd ready ∩ epic, capped)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if epic == "" {
				return exit.Invocationf("--epic is required")
			}
			// Guard the cap explicitly: bd treats `--limit 0` as UNLIMITED, so
			// --max 0 (or negative) would silently invert the dial from "cap the
			// wave" to "no cap". Reject it as an invocation error.
			if max < 1 {
				return exit.Invocationf("--max must be >= 1 (got %d)", max)
			}
			res, err := run.BD(a.Runner, "ready", "--parent", epic, "--limit", strconv.Itoa(max), "--json")
			if err != nil {
				return exit.Hardf("bd ready could not run: %v", err)
			}
			if res.Code != 0 {
				return exit.Hardf("bd ready failed: %s", strings.TrimSpace(res.Stderr))
			}
			var issues []struct {
				ID string `json:"id"`
			}
			if err := json.Unmarshal([]byte(res.Stdout), &issues); err != nil {
				return exit.Hardf("parse bd ready json: %v", err)
			}
			wave := make([]string, 0, len(issues))
			for _, i := range issues {
				wave = append(wave, i.ID)
			}
			data := map[string]any{"epic": epic, "wave": wave}
			text := fmt.Sprintf("shed for %s: %s (%d picks)", epic, strings.Join(wave, " "), len(wave))
			return Emit(cmd, "shed.form", data, text)
		},
	}
	c.Flags().StringVar(&epic, "epic", "", "epic bead-id scoping the ready set (required)")
	// --max is the parallelism dial; its default comes from .weft/config.toml
	// [shed].max (falling back to config.DefaultShedMax). --max overrides it.
	c.Flags().IntVar(&max, "max", a.Config.ShedMax(), "max wave size (parallelism dial)")
	return c
}

func (a *App) newShedCleanupCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "cleanup <bead-id>...",
		Short: "Tear down a wave's workspaces (jj workspace forget + rm)",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := jjRoot(a.Runner)
			if err != nil {
				return err
			}
			wtRoot := workspace.Root(root, a.Config.Workspace.Root)
			cleaned := []string{}
			for _, bead := range args {
				name := workspace.Name(bead)
				path := workspace.Path(root, a.Config.Workspace.Root, bead)
				// Path-safety guard (spec §5): a bead-id carrying "/" or ".."
				// must not let os.RemoveAll escape the worktrees root.
				safe, err := workspace.ContainsResolved(wtRoot, path)
				if err != nil {
					return exit.Hardf("refusing to clean %q: cannot resolve path for containment check: %v", bead, err)
				}
				if !safe {
					return exit.Hardf("refusing to clean %q: resolves outside worktrees root %s", bead, wtRoot)
				}
				if res, err := run.JJ(a.Runner, "workspace", "forget", name); err != nil {
					return exit.Hardf("jj workspace forget could not run: %v", err)
				} else if res.Code != 0 {
					return exit.Hardf("jj workspace forget %s failed: %s", bead, strings.TrimSpace(res.Stderr))
				}
				if err := os.RemoveAll(path); err != nil {
					return exit.Hardf("rm workspace dir %s: %v", path, err)
				}
				cleaned = append(cleaned, bead)
			}
			data := map[string]any{"cleaned": cleaned}
			return Emit(cmd, "shed.cleanup", data, fmt.Sprintf("cleaned %d workspace(s)", len(cleaned)))
		},
	}
}

func (a *App) newShedIsolateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "isolate <bead-id>...",
		Short: "Isolate a wave: per bead set in_progress, then create its workspace on trunk()",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Trunk freshness (spec §7): fetch once per wave before isolating.
			if res, err := run.JJ(a.Runner, "git", "fetch"); err != nil {
				return exit.Hardf("jj git fetch could not run: %v", err)
			} else if res.Code != 0 {
				return exit.Hardf("jj git fetch failed: %s", strings.TrimSpace(res.Stderr))
			}
			root, err := jjRoot(a.Runner)
			if err != nil {
				return err
			}
			isolated := []string{}
			for _, bead := range args {
				// Status-first ordering invariant (spec §4): in_progress BEFORE
				// the workspace exists, so a crash never strands a reapable workspace.
				if res, err := run.BD(a.Runner, "update", bead, "--status", "in_progress"); err != nil {
					return exit.Hardf("bd update could not run: %v", err)
				} else if res.Code != 0 {
					return exit.Hardf("bd update %s failed: %s", bead, strings.TrimSpace(res.Stderr))
				}
				path := workspace.Path(root, a.Config.Workspace.Root, bead)
				name := workspace.Name(bead)
				// If add fails here, the bead is already in_progress with no
				// workspace. That is the deliberate status-first trade (spec §4):
				// recovery is `weft resume`, which surfaces an in_progress bead
				// that has no workspace and re-dispatches it — never a reaper
				// concern. The error names the bead so the strand is explicit.
				if res, err := run.JJ(a.Runner, "workspace", "add", path, "--name", name, "-r", "trunk()"); err != nil {
					return exit.Hardf("jj workspace add could not run: %v", err)
				} else if res.Code != 0 {
					return exit.Hardf("jj workspace add %s failed (bead left in_progress for resume): %s", bead, strings.TrimSpace(res.Stderr))
				}
				isolated = append(isolated, bead)
			}
			data := map[string]any{"wave": isolated}
			return Emit(cmd, "shed.isolate", data,
				fmt.Sprintf("isolated %d picks: %s", len(isolated), strings.Join(isolated, " ")))
		},
	}
}

func (a *App) newShedIntegrateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "integrate <bead-id>...",
		Short: "Rebase the wave's sealed changes into a dep-ordered linear stack",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Wave members are mutually independent (spec §4.1), so the dep graph
			// imposes no intra-wave order; the deterministic tiebreaker is bead-id
			// lexicographic.
			beads := append([]string{}, args...)
			sort.Strings(beads)

			// Resolve each pick's sealed change-id (the spine).
			changes := make([]string, 0, len(beads))
			for _, b := range beads {
				ch, err := changeOf(a.Runner, b)
				if err != nil {
					return err
				}
				if ch == "" {
					return exit.Invocationf("bead %s has no jj-change label (not sealed)", b)
				}
				changes = append(changes, ch)
			}

			// Allowlist-validate every change-id before it is interpolated into a
			// jj revset. Both the per-member `rebase -s <ch>` below and the scoped
			// conflicts() revset further down take change-ids straight from the
			// bead's jj-change:<id> label; a tampered label could otherwise inject
			// revset metacharacters and silently alter evaluation (no OS-shell
			// injection — exec is arg-sliced). Same guard the sibling
			// revset-builders apply (conflict.go changeConflicted /
			// scopedConflictChanges, resume.go conflictChanges); changeIDPattern is
			// defined in conflict.go (same package).
			for _, ch := range changes {
				if !changeIDPattern.MatchString(ch) {
					return exit.Hardf("refusing to interpolate unsafe change-id %q into a revset", ch)
				}
			}

			// Rebase into a linear stack: trunk() <- beads[0] <- beads[1] <- ...
			//
			// NOTE: --skip-emptied is intentionally omitted here, diverging from
			// spec §4.1's verb table. --skip-emptied abandons a member that rebases
			// to empty, which would leave prev=<ch> pointing at a now-nonexistent
			// change and break the next rebase -o <ch>. jj change-ids are stable
			// across rebase, so without the flag every member survives and the
			// linear cursor stays valid; an empty member surfaces downstream rather
			// than being silently dropped. See ADR weft-hjx.7 (decision bead).
			prev := "trunk()"
			stack := make([]map[string]string, 0, len(beads))
			for i, ch := range changes {
				if res, err := run.JJ(a.Runner, "rebase", "-s", ch, "-o", prev); err != nil {
					return exit.Hardf("jj rebase could not run: %v", err)
				} else if res.Code != 0 {
					return exit.Hardf("jj rebase %s failed: %s", ch, strings.TrimSpace(res.Stderr))
				}
				prev = ch
				stack = append(stack, map[string]string{"bead": beads[i], "change": ch})
			}

			// First-class conflicts are surfaced as data; resolution is seam 4.
			// The revset is stack-scoped: only report conflicts that belong to this
			// wave's members, not any pre-existing conflicts elsewhere in the repo.
			scopedRevset := "conflicts() & (" + strings.Join(changes, " | ") + ")"
			res, err := run.JJ(a.Runner, "log", "-r", scopedRevset, "--no-graph", "-T", `change_id.short(12) ++ "\n"`)
			if err != nil {
				return exit.Hardf("jj log conflicts() could not run: %v", err)
			}
			if res.Code != 0 {
				return exit.Hardf("jj log conflicts() failed: %s", strings.TrimSpace(res.Stderr))
			}
			// Map each conflicted change-id back to its owning bead via the
			// stack we just built, so the orchestrator can `conflict open <bead>`
			// (seam 4 §3). paths/lowest_ancestor enrichment is deferred (§8).
			// F6: conflicts[] uses [{bead,change}] — the actionable orchestrator-input
			// form (each entry is directly consumable by `conflict open <bead>`),
			// distinct from resume's observability []string form (see conflictChanges).
			changeToBead := map[string]string{}
			for _, e := range stack {
				changeToBead[e["change"]] = e["bead"]
			}
			conflicts := []map[string]string{}
			for _, ln := range splitTrimLines(res.Stdout) {
				// F4: guard against a conflicted change-id not in the integration stack.
				// A missing key would silently produce bead:"" (misleading for orchestrators).
				b, ok := changeToBead[ln]
				if !ok {
					return exit.Hardf("conflicted change %s is not in the integration stack — cannot map it to a bead", ln)
				}
				conflicts = append(conflicts, map[string]string{"bead": b, "change": ln})
			}

			// NOTE (seam-4 envelope deferred): conflicts[] is emitted inside data{} here,
			// not as a top-level envelope field. The decision on whether conflicts belongs
			// at the top-level envelope (parity with the resume note in conflictChanges)
			// is tracked as a deferred seam-4 envelope decision (weft-hjx.6).
			data := map[string]any{"stack": stack, "conflicts": conflicts}
			// changes[i] == stack[i]["change"] by construction; reuse directly.
			text := fmt.Sprintf("integrated %d picks: %s", len(stack), strings.Join(changes, " -> "))
			if len(conflicts) > 0 {
				ids := make([]string, 0, len(conflicts))
				for _, c := range conflicts {
					ids = append(ids, c["change"])
				}
				text += fmt.Sprintf("  [%d conflicted: %s]", len(conflicts), strings.Join(ids, " "))
			}
			return Emit(cmd, "shed.integrate", data, text)
		},
	}
}
