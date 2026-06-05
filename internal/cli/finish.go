// internal/cli/finish.go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Weft Contributors

package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/seanb4t/weft/internal/exit"
	"github.com/seanb4t/weft/internal/run"
)

// finishPick is one closed pick: its bead id, conventional-commit subject
// (bead title), and woven jj change-id. The PR body is assembled from these
// (spec §5, design.md §5.1 audit — no SUMMARY.md).
type finishPick struct {
	Bead   string `json:"bead"`
	Title  string `json:"title"`
	Change string `json:"change"`
}

// closedPicks reads the epic's closed children in ONE bd list call, yielding
// bead id + title + jj-change label together. (beadIDsByStatus returns only
// ids; epicChanges returns only change-ids — neither alone carries the title,
// so finish open needs this combined reader; see spec §5.) Returns a non-nil
// empty slice for an epic with no closed children.
func closedPicks(r run.Runner, epic string) ([]finishPick, error) {
	res, err := run.BD(r, "list", "--parent", epic, "--status", "closed", "--json")
	if err != nil {
		return nil, exit.Hardf("bd list could not run: %v", err)
	}
	if res.Code != 0 {
		return nil, exit.Hardf("bd list failed: %s", strings.TrimSpace(res.Stderr))
	}
	var arr []struct {
		ID     string   `json:"id"`
		Title  string   `json:"title"`
		Labels []string `json:"labels"`
	}
	if err := json.Unmarshal([]byte(res.Stdout), &arr); err != nil {
		return nil, exit.Hardf("parse bd json: %v", err)
	}
	picks := make([]finishPick, 0, len(arr))
	for _, b := range arr {
		picks = append(picks, finishPick{Bead: b.ID, Title: b.Title, Change: changeFromLabels(b.Labels)})
	}
	return picks, nil
}

// assemblePRBody renders the PR body from the epic's closed picks (spec §5):
// a one-line summary, one bullet per pick tying its subject to its change-id,
// and the generated-by trailer. Deterministic — no LLM call.
func assemblePRBody(epic, title string, picks []finishPick) string {
	var b strings.Builder
	fmt.Fprintf(&b, "## Summary\n\n%d picks woven for %s — %s.\n\n## Picks\n\n", len(picks), epic, title)
	for _, p := range picks {
		fmt.Fprintf(&b, "- `%s` %s (`%s`)\n", p.Bead, p.Title, p.Change)
	}
	b.WriteString("\n🤖 Generated with [Claude Code](https://claude.com/claude-code)\n")
	return b.String()
}

func (a *App) newFinishCmd() *cobra.Command {
	finish := &cobra.Command{Use: "finish", Short: "Ship an epic: open a PR, then reconcile after merge (spec §6 / seam 6)"}
	finish.AddCommand(a.newFinishOpenCmd(), a.newFinishReconcileCmd())
	return finish
}

// finishOpenPreflight enforces the spec §4.1 step-1 / §5 guards. Returns the
// closed picks (so the caller need not re-read) on success.
func (a *App) finishOpenPreflight(epic string) ([]finishPick, error) {
	// 1. Working tree clean: in jj, the clean state is an EMPTY @ on top of the
	// described picks (post jj commit / jj new). jj prints this exact line.
	if res, err := run.JJ(a.Runner, "st"); err != nil {
		return nil, exit.Hardf("jj st could not run: %v", err)
	} else if res.Code != 0 {
		return nil, exit.Hardf("jj st failed: %s", strings.TrimSpace(res.Stderr))
	} else if !strings.Contains(res.Stdout, "no changes") {
		return nil, exit.Invocationf("working copy is not clean — commit your picks (jj commit) before finishing")
	}
	// 2. Stack non-empty: there is something between trunk() and @ to ship.
	res, err := run.JJ(a.Runner, "log", "-r", "trunk()..@", "--no-graph", "-T", `change_id.short(12) ++ "\n"`)
	if err != nil {
		return nil, exit.Hardf("jj log could not run: %v", err)
	}
	if res.Code != 0 {
		return nil, exit.Hardf("jj log failed: %s", strings.TrimSpace(res.Stderr))
	}
	if strings.TrimSpace(res.Stdout) == "" {
		return nil, exit.Invocationf("nothing to ship for %s — no changes between trunk() and @", epic)
	}
	// 3. origin remote configured.
	if res, err := run.JJ(a.Runner, "git", "remote", "list"); err != nil {
		return nil, exit.Hardf("jj git remote list could not run: %v", err)
	} else if res.Code != 0 {
		return nil, exit.Hardf("jj git remote list failed: %s", strings.TrimSpace(res.Stderr))
	} else if !strings.Contains(res.Stdout, "origin") {
		return nil, exit.Invocationf("no 'origin' remote configured — cannot push %s", epic)
	}
	// 4. gh authenticated.
	if res, err := run.GH(a.Runner, "auth", "status"); err != nil {
		return nil, exit.Hardf("gh auth status could not run (is gh installed?): %v", err)
	} else if res.Code != 0 {
		return nil, exit.Invocationf("gh is not authenticated — run `gh auth login`")
	}
	// 5. Empty-epic guard (§5): refuse rather than open an empty PR.
	picks, err := closedPicks(a.Runner, epic)
	if err != nil {
		return nil, err
	}
	if len(picks) == 0 {
		return nil, exit.Invocationf("nothing woven to ship for %s — no closed beads", epic)
	}
	return picks, nil
}

func (a *App) newFinishOpenCmd() *cobra.Command {
	var dryRun, draft bool
	c := &cobra.Command{
		Use:   "open <epic>",
		Short: "Push the epic's stack and open a GitHub PR (body from closed beads)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			epic := args[0]
			picks, err := a.finishOpenPreflight(epic)
			if err != nil {
				return err
			}
			title := fmt.Sprintf("%s (%s)", epic, epic) // title enrichment deferred to weft-hjx.9.7
			body := assemblePRBody(epic, epic, picks)

			if dryRun {
				data := map[string]any{
					"epic": epic, "bookmark": epic, "pushed": false,
					"pr_url": "", "pr_exists": false, "picks": picks, "dry_run": true,
				}
				return Emit(cmd, "finish.open", data,
					fmt.Sprintf("[dry-run] would push %s and open PR:\n%s", epic, body))
			}

			// Set the bookmark at the working-copy tip and push.
			if res, err := run.JJ(a.Runner, "bookmark", "set", epic, "-r", "@"); err != nil {
				return exit.Hardf("jj bookmark set could not run: %v", err)
			} else if res.Code != 0 {
				return exit.Hardf("jj bookmark set %s failed: %s", epic, strings.TrimSpace(res.Stderr))
			}
			if res, err := run.JJ(a.Runner, "git", "push", "-b", epic); err != nil {
				return exit.Hardf("jj git push could not run: %v", err)
			} else if res.Code != 0 {
				return exit.Hardf("jj git push -b %s failed: %s", epic, strings.TrimSpace(res.Stderr))
			}

			// Idempotency (§4.3): if a PR already exists for the branch, re-push is
			// done above; report the existing PR instead of opening a second.
			if res, err := run.GH(a.Runner, "pr", "view", epic, "--json", "url"); err == nil && res.Code == 0 {
				var existing struct {
					URL string `json:"url"`
				}
				if json.Unmarshal([]byte(res.Stdout), &existing) == nil && existing.URL != "" {
					data := map[string]any{
						"epic": epic, "bookmark": epic, "pushed": true,
						"pr_url": existing.URL, "pr_exists": true, "picks": picks, "dry_run": false,
					}
					return Emit(cmd, "finish.open", data,
						fmt.Sprintf("re-pushed %s; PR already open: %s", epic, existing.URL))
				}
			}

			// Assemble the body to a temp file (shell-arg limits; same idiom as
			// plan emit's bd create --graph payload).
			path, cleanup, err := writeTempPayload("weft-pr-body-*.md", []byte(body))
			if err != nil {
				return err
			}
			defer cleanup()

			ghArgs := []string{"pr", "create", "--title", title, "--body-file", path, "--base", "main"}
			if draft {
				ghArgs = append(ghArgs, "--draft")
			}
			res, err := run.GH(a.Runner, ghArgs...)
			if err != nil {
				return exit.Hardf("gh pr create could not run: %v", err)
			}
			if res.Code != 0 {
				return exit.Hardf("gh pr create failed: %s", strings.TrimSpace(res.Stderr))
			}
			prURL := strings.TrimSpace(res.Stdout)
			data := map[string]any{
				"epic": epic, "bookmark": epic, "pushed": true,
				"pr_url": prURL, "pr_exists": false, "picks": picks, "dry_run": false,
			}
			return Emit(cmd, "finish.open", data, fmt.Sprintf("opened PR for %s: %s", epic, prURL))
		},
	}
	c.Flags().BoolVar(&dryRun, "dry-run", false, "emit the push plan + PR body + gh command without mutating")
	c.Flags().BoolVar(&draft, "draft", false, "open the PR as a draft")
	return c
}

// prMerged reports whether the epic's PR is in the MERGED state (spec §6.1
// safety gate — never abandon unmerged work; jj alone cannot distinguish a
// squash-merge from a never-merged branch).
func prMerged(r run.Runner, epic string) (bool, error) {
	res, err := run.GH(r, "pr", "view", epic, "--json", "state,mergeCommit")
	if err != nil {
		return false, exit.Hardf("gh pr view could not run: %v", err)
	}
	if res.Code != 0 {
		return false, exit.Hardf("gh pr view %s failed: %s", epic, strings.TrimSpace(res.Stderr))
	}
	var v struct {
		State string `json:"state"`
	}
	if err := json.Unmarshal([]byte(res.Stdout), &v); err != nil {
		return false, exit.Hardf("parse gh pr view json: %v", err)
	}
	return v.State == "MERGED", nil
}

// mergeStyle returns "merge_commit" if the epic's pushed tip (<epic>@origin) is
// an ancestor of main@origin (a true-merge, reconcilable via rebase
// --skip-emptied), or "squash_or_rebase" otherwise (content landed under a new
// commit id — needs jj new main + jj abandon). Spec §6.1.3.
func mergeStyle(r run.Runner, epic string) (string, error) {
	revset := epic + "@origin & ::main@origin"
	res, err := run.JJ(r, "log", "-r", revset, "--no-graph", "-T", "commit_id")
	if err != nil {
		return "", exit.Hardf("jj log (merge-style detect) could not run: %v", err)
	}
	if res.Code != 0 {
		return "", exit.Hardf("jj log (merge-style detect) failed: %s", strings.TrimSpace(res.Stderr))
	}
	if strings.TrimSpace(res.Stdout) != "" {
		return "merge_commit", nil
	}
	return "squash_or_rebase", nil
}

func (a *App) newFinishReconcileCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "reconcile <epic>",
		Short: "Reconcile local jj state after the epic's PR merges",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			epic := args[0]
			merged, err := prMerged(a.Runner, epic)
			if err != nil {
				return err
			}
			if !merged {
				return exit.Invocationf("PR for %s is not merged — refusing to reconcile", epic)
			}
			return exit.Hardf("finish reconcile execution not yet implemented") // Task 6
		},
	}
	return c
}
