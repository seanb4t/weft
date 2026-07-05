// internal/cli/doctor.go
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

// finding is one doctor diagnosis. reason is machine-readable and
// category-scoped (spec Component 2): stray → stale-activity|landed-unclosed;
// orphan → bead-not-in-progress|bead-missing; lost → workspace-missing;
// conflicted → change-conflicted; unreconciled → pr-merged-local-remains;
// foreign → no-bead. evidence is human-oriented; suggest names the recovery
// verb — doctor only proposes, it never mutates (invariant I1, ADR weft-qc0).
type finding struct {
	Category  string `json:"category"`
	Reason    string `json:"reason"`
	Bead      string `json:"bead,omitempty"`
	Workspace string `json:"workspace,omitempty"`
	Change    string `json:"change,omitempty"`
	Evidence  string `json:"evidence"`
	Suggest   string `json:"suggest"`
}

func (a *App) newDoctorCmd() *cobra.Command {
	var epic string
	c := &cobra.Command{
		Use:   "doctor",
		Short: "Whole-warp health: join beads × workspaces × changes × PRs; report, never mutate (spec I1)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			threshold, err := a.Config.LivenessThreshold()
			if err != nil {
				return exit.Invocationf("[liveness] threshold: %v", err)
			}
			root, err := jjRoot(a.Runner)
			if err != nil {
				return err
			}
			wtRoot := workspace.Root(root, a.Config.Workspace.Root)

			// Envelope keys are always []-initialized (never null) — seam 9
			// discipline. findings accumulates across all three passes; warnings
			// carries only the best-effort gh degradations (Pass 3).
			findings := []finding{}
			warnings := []string{}

			// Pass 1 — workspace-side: the reap join (reap.go:39-61), REPORT-ONLY.
			// Two disjoint sets feed Pass 2: seenWorkspace records every in-scope
			// non-default workspace's bead (so a bead with a workspace can never be
			// concluded "lost"); seenStray records ONLY the beads Pass 1 already
			// flagged stray/stale-activity (so Pass 2 does not double-report them).
			// A bead with a live/fresh workspace is in seenWorkspace but NOT
			// seenStray, so Pass 2 still inspects its change for landed/conflicted.
			res, err := run.JJ(a.Runner, "workspace", "list", "-T", `name ++ "\n"`)
			if err != nil {
				return exit.Hardf("jj workspace list could not run: %v", err)
			}
			if res.Code != 0 {
				return exit.Hardf("jj workspace list failed: %s", strings.TrimSpace(res.Stderr))
			}
			seenWorkspace := map[string]bool{}
			seenStray := map[string]bool{}
			now := time.Now()
			for _, name := range splitTrimLines(res.Stdout) {
				if name == "default" {
					continue // the orchestrator's own workspace is never a finding
				}
				bead, _ := workspace.Resolve(name) // kind-aware, exactly reap's join key
				if !scopedToEpic(epic, bead) {
					continue
				}
				seenWorkspace[bead] = true
				status, err := beadStatus(a.Runner, bead)
				if err != nil {
					return err // bd is local infrastructure — hard-fail (exit 2)
				}
				dir := filepath.Join(wtRoot, name)
				switch {
				case status == "":
					// A missing bead is an orphan ONLY if weft owns its directory;
					// a missing bead with no dir under wtRoot is a foreign workspace
					// (e.g. a Claude Code worktree-agent-*) — reap must never touch
					// it, so doctor flags it distinctly (spec Component 3).
					if dirExists(dir) {
						findings = append(findings, finding{
							Category: "orphan", Reason: "bead-missing",
							Bead: bead, Workspace: name,
							Evidence: fmt.Sprintf("workspace %s present under %s but bead %s does not exist", name, wtRoot, bead),
							Suggest:  "weft reap",
						})
					} else {
						findings = append(findings, finding{
							Category: "foreign", Reason: "no-bead",
							Workspace: name,
							Evidence:  fmt.Sprintf("workspace %s resolves to no bead and has no directory under %s", name, wtRoot),
							Suggest:   "manual sweep",
						})
					}
				case status != "in_progress":
					findings = append(findings, finding{
						Category: "orphan", Reason: "bead-not-in-progress",
						Bead: bead, Workspace: name,
						Evidence: fmt.Sprintf("bead %s is %s but workspace %s remains", bead, status, name),
						Suggest:  "weft reap",
					})
				default: // in_progress — a stale workspace is a probable crashed executor
					last, err := liveness.LastActivity(a.Runner, name, dir)
					if err != nil {
						// jj is local infrastructure, not best-effort — a liveness
						// error hard-fails rather than masking a dead workspace.
						return exit.Hardf("liveness for workspace %s: %v", name, err)
					}
					if !liveness.Live(last, now, threshold) {
						seenStray[bead] = true // dedup: Pass 2 must not re-report it
						findings = append(findings, finding{
							Category: "stray", Reason: "stale-activity",
							Bead: bead, Workspace: name,
							Evidence: fmt.Sprintf("bead %s in_progress; last activity %s exceeds threshold %s", bead, last.Format(time.RFC3339), threshold),
							Suggest:  "weft reap",
						})
					}
				}
			}

			// Pass 2 — bead-side: every in_progress bead globally (bd list, no
			// --parent). The landed/conflicted checks run for EVERY bead Pass 1
			// did not already flag stray (seenStray) — a live workspace does not
			// exempt a bead whose sealed change is already in trunk()/conflicts(),
			// which is exactly the state doctor exists to catch. Only the terminal
			// "lost" conclusion is gated on workspace absence (seenWorkspace): a
			// bead that has a workspace cannot be lost. A sealed change
			// (jj-change:<id> label) is the join key.
			inprog, err := inProgressBeads(a.Runner)
			if err != nil {
				return err
			}
			for _, b := range inprog {
				if !scopedToEpic(epic, b.ID) {
					continue
				}
				if seenStray[b.ID] {
					continue // Pass 1 already reported this bead as a stray
				}
				change := changeFromLabels(b.Labels)
				if change != "" {
					landed, err := changeInTrunk(a.Runner, change)
					if err != nil {
						return err
					}
					if landed {
						findings = append(findings, finding{
							Category: "stray", Reason: "landed-unclosed",
							Bead: b.ID, Change: change,
							Evidence: fmt.Sprintf("change %s for bead %s is in trunk() but the bead is still in_progress", change, b.ID),
							Suggest:  fmt.Sprintf("bd close %s", b.ID),
						})
						continue
					}
					conflicted, err := changeConflicted(a.Runner, change)
					if err != nil {
						return err
					}
					if conflicted {
						findings = append(findings, finding{
							Category: "conflicted", Reason: "change-conflicted",
							Bead: b.ID, Change: change,
							Evidence: fmt.Sprintf("change %s for bead %s is in conflicts()", change, b.ID),
							Suggest:  fmt.Sprintf("weft conflict open %s", b.ID),
						})
						continue
					}
				}
				if seenWorkspace[b.ID] {
					continue // has a workspace and a benign change → Pass 1 owns it
				}
				// in_progress, no workspace, change (if any) neither landed nor
				// conflicted → the woven work is unreachable.
				findings = append(findings, finding{
					Category: "lost", Reason: "workspace-missing",
					Bead: b.ID, Change: change,
					Evidence: fmt.Sprintf("bead %s is in_progress but has no workspace under %s", b.ID, wtRoot),
					Suggest:  fmt.Sprintf("weft pick redo %s", b.ID),
				})
			}

			// Pass 3 — epic-side (best-effort gh): an open epic whose local
			// bookmark still exists while its PR is MERGED needs reconciliation.
			// EVERY gh error degrades to a warning and never aborts the join —
			// the deleteRemoteBranch posture (finish.go:403). Doctor is fully
			// useful offline.
			epics, err := openEpics(a.Runner)
			if err != nil {
				return err
			}
			for _, ep := range epics {
				if !scopedToEpic(epic, ep) {
					continue
				}
				if err := validateEpicID(ep); err != nil {
					warnings = append(warnings, fmt.Sprintf("skipping epic %q: %v", ep, err))
					continue
				}
				present, err := bookmarkPresent(a.Runner, ep)
				if err != nil {
					return err // jj is local infrastructure — hard-fail
				}
				if !present {
					continue // nothing local remains → nothing to reconcile
				}
				merged, warn := epicPRMerged(a.Runner, ep)
				if warn != "" {
					warnings = append(warnings, warn)
				}
				if merged {
					findings = append(findings, finding{
						Category: "unreconciled", Reason: "pr-merged-local-remains",
						Bead:     ep,
						Evidence: fmt.Sprintf("epic %s PR is MERGED but its local bookmark still exists", ep),
						Suggest:  fmt.Sprintf("weft finish reconcile %s", ep),
					})
				}
			}

			data := map[string]any{"findings": findings, "warnings": warnings}
			return Emit(cmd, "doctor", data, doctorSummary(findings, warnings))
		},
	}
	c.Flags().StringVar(&epic, "epic", "", "scope the join to descendants of this epic")
	return c
}

// scopedToEpic reports whether id falls under the --epic filter: an unset epic
// matches everything; otherwise id must equal the epic or be a dotted
// descendant (the hierarchical-prefix rule from reap.go:46).
func scopedToEpic(epic, id string) bool {
	return epic == "" || id == epic || strings.HasPrefix(id, epic+".")
}

// dirExists reports whether path is an existing directory. Used to separate an
// orphan (weft-owned workspace dir present) from a foreign workspace (no dir).
func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// inProgressBead is the subset of `bd list --status in_progress --json` the
// bead-side join needs: the id and its labels (the jj-change:<id> join key).
type inProgressBead struct {
	ID     string   `json:"id"`
	Labels []string `json:"labels"`
}

// inProgressBeads lists every in_progress bead globally (no --parent, so it
// crosses epics). bd is local infrastructure — any failure hard-fails (exit 2).
func inProgressBeads(r run.Runner) ([]inProgressBead, error) {
	res, err := run.BD(r, "list", "--status", "in_progress", "--json")
	if err != nil {
		return nil, exit.Hardf("bd list could not run: %v", err)
	}
	if res.Code != 0 {
		return nil, exit.Hardf("bd list failed: %s", strings.TrimSpace(res.Stderr))
	}
	var arr []inProgressBead
	if err := json.Unmarshal([]byte(res.Stdout), &arr); err != nil {
		return nil, exit.Hardf("parse bd list json: %v", err)
	}
	return arr, nil
}

// changeInTrunk reports whether a sealed change has already landed in trunk().
// The change-id is validated against changeIDPattern before interpolation
// (the resume.go:119 revset-injection guard).
func changeInTrunk(r run.Runner, change string) (bool, error) {
	if !changeIDPattern.MatchString(change) {
		return false, exit.Hardf("refusing to interpolate unsafe change-id %q into a revset", change)
	}
	res, err := run.JJ(r, "log", "-r", change+" & ::trunk()", "--no-graph", "-T", `change_id.short(12) ++ "\n"`)
	if err != nil {
		return false, exit.Hardf("jj log (change-in-trunk) could not run: %v", err)
	}
	if res.Code != 0 {
		return false, exit.Hardf("jj log (change-in-trunk) failed: %s", strings.TrimSpace(res.Stderr))
	}
	return strings.TrimSpace(res.Stdout) != "", nil
}

// openEpics lists the ids of every open epic from `bd epic status --json`. bd
// is local infrastructure — any failure hard-fails (exit 2).
func openEpics(r run.Runner) ([]string, error) {
	res, err := run.BD(r, "epic", "status", "--json")
	if err != nil {
		return nil, exit.Hardf("bd epic status could not run: %v", err)
	}
	if res.Code != 0 {
		return nil, exit.Hardf("bd epic status failed: %s", strings.TrimSpace(res.Stderr))
	}
	var arr []struct {
		Epic struct {
			ID     string `json:"id"`
			Status string `json:"status"`
		} `json:"epic"`
	}
	if err := json.Unmarshal([]byte(res.Stdout), &arr); err != nil {
		return nil, exit.Hardf("parse bd epic status json: %v", err)
	}
	epics := []string{}
	for _, e := range arr {
		if e.Epic.Status == "open" {
			epics = append(epics, e.Epic.ID)
		}
	}
	return epics, nil
}

// bookmarkPresent reports whether a local bookmark named epic exists. jj prints
// the matching bookmark on stdout (empty when absent, with a stderr warning);
// a missing bookmark is exit 0, so any non-zero exit is a jj infrastructure
// failure and hard-fails. The epic value is validated by validateEpicID before
// it reaches this argument.
func bookmarkPresent(r run.Runner, epic string) (bool, error) {
	res, err := run.JJ(r, "bookmark", "list", epic)
	if err != nil {
		return false, exit.Hardf("jj bookmark list could not run: %v", err)
	}
	if res.Code != 0 {
		return false, exit.Hardf("jj bookmark list failed: %s", strings.TrimSpace(res.Stderr))
	}
	return strings.TrimSpace(res.Stdout) != "", nil
}

// epicPRMerged reports whether the epic's PR is MERGED, best-effort: every gh
// failure mode (could-not-run, non-zero exit, unparseable output) degrades to a
// non-empty warning and (false, warning) rather than aborting — the
// deleteRemoteBranch posture (finish.go:403). Returns (true, "") only on a
// definitively MERGED PR, (false, "") on any other known state.
func epicPRMerged(r run.Runner, epic string) (merged bool, warning string) {
	res, err := run.GH(r, "pr", "view", epic, "--json", "state")
	if err != nil {
		return false, fmt.Sprintf("gh pr view %s could not run (PR state unknown): %v", epic, err)
	}
	if res.Code != 0 {
		return false, fmt.Sprintf("gh pr view %s failed (PR state unknown): %s", epic, strings.TrimSpace(res.Stderr))
	}
	var v struct {
		State string `json:"state"`
	}
	if err := json.Unmarshal([]byte(res.Stdout), &v); err != nil {
		return false, fmt.Sprintf("parse gh pr view json for %s (PR state unknown): %v", epic, err)
	}
	return v.State == "MERGED", ""
}

// doctorSummary renders the human text: a header count then one line per
// finding — "<category>(<reason>) <bead|workspace>: <evidence> → <suggest>" —
// with warnings appended. "warp healthy — no findings" when there are none.
func doctorSummary(findings []finding, warnings []string) string {
	var b strings.Builder
	if len(findings) == 0 {
		b.WriteString("warp healthy — no findings")
	} else {
		fmt.Fprintf(&b, "%d finding(s)", len(findings))
		for _, f := range findings {
			subject := f.Bead
			if subject == "" {
				subject = f.Workspace
			}
			fmt.Fprintf(&b, "\n  %s(%s) %s: %s → %s", f.Category, f.Reason, subject, f.Evidence, f.Suggest)
		}
	}
	for _, w := range warnings {
		fmt.Fprintf(&b, "\n  warning: %s", w)
	}
	return b.String()
}
