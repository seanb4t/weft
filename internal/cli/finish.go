// internal/cli/finish.go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Weft Contributors

package cli

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/spf13/cobra"

	"github.com/seanb4t/weft/internal/exit"
	"github.com/seanb4t/weft/internal/run"
)

// epicIDPattern matches a bead id (the epic argument). Restricting to
// [a-zA-Z0-9._-] excludes every jj-revset metacharacter (space, '&', '|', ':',
// '@', '(', ')', '~') and the path separator '/', so the epic value is safe to
// interpolate into both the mergeStyle revset (<epic>@origin) and the GitHub API
// ref path (repos/{slug}/git/refs/heads/<epic>). Mirrors the revset-injection
// guard idiom on changeIDPattern (conflict.go); see spec §7.
var epicIDPattern = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)

// validateEpicID rejects an epic argument that could alter a revset or walk the
// GitHub API path, before any interpolation. Returns an invocation error
// (exit 1) so a bad argument never reaches a subprocess.
func validateEpicID(epic string) error {
	if !epicIDPattern.MatchString(epic) {
		return exit.Invocationf("invalid epic id %q — must match %s", epic, epicIDPattern)
	}
	return nil
}

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
			if err := validateEpicID(epic); err != nil {
				return err
			}
			picks, err := a.finishOpenPreflight(epic)
			if err != nil {
				return err
			}
			// PR title is "<epic-title> (<epic-id>)" (spec §4.2) — read the epic
			// title via bd show; the epic arg is the id.
			info, err := showBead(a.Runner, epic)
			if err != nil {
				return err
			}
			title := fmt.Sprintf("%s (%s)", info.Title, epic)
			body := assemblePRBody(epic, info.Title, picks)

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

			// Idempotency (§4.3): if an OPEN PR already exists for the branch,
			// re-push is done above; report the existing PR instead of opening a
			// second. A code==0 `gh pr view` with unparseable output is a hard
			// error — never fall through to pr create, or a schema/output drift
			// would silently open a duplicate PR.
			if res, err := run.GH(a.Runner, "pr", "view", epic, "--json", "url,state"); err == nil && res.Code == 0 {
				var existing struct {
					URL   string `json:"url"`
					State string `json:"state"`
				}
				if uerr := json.Unmarshal([]byte(res.Stdout), &existing); uerr != nil {
					return exit.Hardf("parse gh pr view json: %v", uerr)
				}
				if existing.URL != "" && existing.State == "OPEN" {
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

// Merge styles returned by mergeStyle and switched on in reconcile. Named
// constants keep the producer (mergeStyle) and consumer (reconcile switch) on a
// single source of truth, so an unhandled value is a compile-visible gap rather
// than a silent fall-through into the destructive abandon path.
const (
	mergeStyleMergeCommit    = "merge_commit"
	mergeStyleSquashOrRebase = "squash_or_rebase"
)

// prState returns the epic PR's state ("MERGED", "OPEN", "CLOSED"). The
// reconcile safety gate (spec §6.1) proceeds only on "MERGED" — never abandon
// unmerged work; jj alone cannot distinguish a squash-merge from a never-merged
// branch. Returning the raw state (not a bool) lets the caller name it in the
// refusal message.
func prState(r run.Runner, epic string) (string, error) {
	res, err := run.GH(r, "pr", "view", epic, "--json", "state,mergeCommit")
	if err != nil {
		return "", exit.Hardf("gh pr view could not run: %v", err)
	}
	if res.Code != 0 {
		return "", exit.Hardf("gh pr view %s failed: %s", epic, strings.TrimSpace(res.Stderr))
	}
	var v struct {
		State string `json:"state"`
	}
	if err := json.Unmarshal([]byte(res.Stdout), &v); err != nil {
		return "", exit.Hardf("parse gh pr view json: %v", err)
	}
	return v.State, nil
}

// mergeStyle returns mergeStyleMergeCommit if the epic's pushed tip
// (<epic>@origin) is an ancestor of main@origin (a true-merge, reconcilable via
// rebase --skip-emptied), or mergeStyleSquashOrRebase otherwise (content landed
// under a new commit id — needs jj new main + jj abandon). Spec §6.1 step 3.
// The epic value is validated by validateEpicID before it reaches this revset.
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
		return mergeStyleMergeCommit, nil
	}
	return mergeStyleSquashOrRebase, nil
}

// deleteRemoteBranch removes the epic's remote branch via the GitHub API.
// gh pr merge --delete-branch is unreliable (PR #18 evidence), so reconcile
// deletes the ref directly. Best-effort and idempotent: an already-absent
// branch (or unresolvable slug) yields false, not an error. Returns whether a
// branch was actually deleted (spec §6.1 step 4). The epic value is validated
// by validateEpicID before it reaches the ref path.
func deleteRemoteBranch(r run.Runner, epic string) bool {
	res, err := run.GH(r, "repo", "view", "--json", "nameWithOwner")
	if err != nil || res.Code != 0 {
		return false
	}
	var v struct {
		NameWithOwner string `json:"nameWithOwner"`
	}
	if json.Unmarshal([]byte(res.Stdout), &v) != nil || v.NameWithOwner == "" {
		return false
	}
	ref := fmt.Sprintf("repos/%s/git/refs/heads/%s", v.NameWithOwner, epic)
	res, err = run.GH(r, "api", "-X", "DELETE", ref)
	return err == nil && res.Code == 0
}

func (a *App) newFinishReconcileCmd() *cobra.Command {
	var dryRun bool
	c := &cobra.Command{
		Use:   "reconcile <epic>",
		Short: "Reconcile local jj state after the epic's PR merges",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			epic := args[0]
			if err := validateEpicID(epic); err != nil {
				return err
			}
			state, err := prState(a.Runner, epic)
			if err != nil {
				return err
			}
			if state != "MERGED" {
				return exit.Invocationf("PR for %s is not merged (%s) — refusing to reconcile", epic, state)
			}
			// jj git fetch updates remote-tracking refs (a local mutation), so it
			// runs only in the real path — dry-run must mutate nothing (spec §6.1).
			// Dry-run's mergeStyle therefore previews against current refs.
			if !dryRun {
				if res, err := run.JJ(a.Runner, "git", "fetch"); err != nil {
					return exit.Hardf("jj git fetch could not run: %v", err)
				} else if res.Code != 0 {
					return exit.Hardf("jj git fetch failed: %s", strings.TrimSpace(res.Stderr))
				}
			}
			style, err := mergeStyle(a.Runner, epic)
			if err != nil {
				return err
			}
			abandoned := []string{}
			if dryRun {
				data := map[string]any{
					"epic": epic, "merged": true, "merge_style": style,
					"abandoned": abandoned, "bookmark_deleted": false,
					"remote_branch_deleted": false, "dry_run": true,
				}
				return Emit(cmd, "finish.reconcile", data,
					fmt.Sprintf("[dry-run] %s merged (%s) — would reconcile", epic, style))
			}
			switch style {
			case mergeStyleMergeCommit:
				if res, err := run.JJ(a.Runner, "rebase", "-b", "@", "-o", "main", "--skip-emptied"); err != nil {
					return exit.Hardf("jj rebase could not run: %v", err)
				} else if res.Code != 0 {
					return exit.Hardf("jj rebase failed: %s", strings.TrimSpace(res.Stderr))
				}
			case mergeStyleSquashOrRebase:
				if res, err := run.JJ(a.Runner, "new", "main"); err != nil {
					return exit.Hardf("jj new main could not run: %v", err)
				} else if res.Code != 0 {
					return exit.Hardf("jj new main failed: %s", strings.TrimSpace(res.Stderr))
				}
				rootRes, err := run.JJ(a.Runner, "log", "-r", "roots(trunk()..@)", "--no-graph", "-T", `change_id.short(12) ++ "\n"`)
				if err != nil {
					return exit.Hardf("jj log roots could not run: %v", err)
				}
				if rootRes.Code != 0 {
					return exit.Hardf("jj log roots failed: %s", strings.TrimSpace(rootRes.Stderr))
				}
				for _, root := range splitTrimLines(rootRes.Stdout) {
					if res, err := run.JJ(a.Runner, "abandon", root+"::"); err != nil {
						return exit.Hardf("jj abandon could not run: %v", err)
					} else if res.Code != 0 {
						return exit.Hardf("jj abandon %s:: failed: %s", root, strings.TrimSpace(res.Stderr))
					}
					abandoned = append(abandoned, root)
				}
			default:
				return exit.Hardf("unknown merge style %q", style)
			}
			// Drop the local bookmark (idempotent backstop; the squash abandon may
			// already have removed it — a "no such bookmark" is expected). The
			// envelope reports the actual outcome rather than assuming success.
			delRes, delErr := run.JJ(a.Runner, "bookmark", "delete", epic)
			bookmarkDeleted := delErr == nil && delRes.Code == 0
			// Delete the remote branch if the merge left it behind (§6.1 step 4).
			remoteDeleted := deleteRemoteBranch(a.Runner, epic)
			data := map[string]any{
				"epic": epic, "merged": true, "merge_style": style,
				"abandoned": abandoned, "bookmark_deleted": bookmarkDeleted,
				"remote_branch_deleted": remoteDeleted, "dry_run": false,
			}
			return Emit(cmd, "finish.reconcile", data,
				fmt.Sprintf("reconciled %s (%s): %d abandoned", epic, style, len(abandoned)))
		},
	}
	c.Flags().BoolVar(&dryRun, "dry-run", false, "detect the merge style and emit the plan without mutating")
	return c
}
