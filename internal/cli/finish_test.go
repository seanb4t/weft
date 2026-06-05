// internal/cli/finish_test.go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Weft Contributors

package cli

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/seanb4t/weft/internal/exit"
	"github.com/seanb4t/weft/internal/run"
)

func TestClosedPicksReadsBeadTitleAndChange(t *testing.T) {
	r := &routeRunner{fn: func(name string, args []string) run.Result {
		j := strings.Join(append([]string{name}, args...), " ")
		if strings.Contains(j, "bd list --parent weft-e --status closed") {
			return run.Result{Stdout: `[{"id":"weft-e.1","title":"feat: A","labels":["jj-change:cha"]},{"id":"weft-e.2","title":"fix: B","labels":["jj-change:chb"]}]`, Code: 0}
		}
		return run.Result{Code: 0}
	}}
	picks, err := closedPicks(r, "weft-e")
	if err != nil {
		t.Fatalf("closedPicks: %v", err)
	}
	if len(picks) != 2 {
		t.Fatalf("want 2 picks, got %d", len(picks))
	}
	if picks[0] != (finishPick{Bead: "weft-e.1", Title: "feat: A", Change: "cha"}) {
		t.Errorf("picks[0] = %+v", picks[0])
	}
}

func TestClosedPicksEmptyEpicReturnsEmptySlice(t *testing.T) {
	r := &routeRunner{fn: func(string, []string) run.Result {
		return run.Result{Stdout: `[]`, Code: 0}
	}}
	picks, err := closedPicks(r, "weft-e")
	if err != nil {
		t.Fatalf("closedPicks: %v", err)
	}
	if picks == nil || len(picks) != 0 {
		t.Errorf("want non-nil empty slice, got %#v", picks)
	}
}

func TestAssemblePRBodyListsPicksWithChangeIDs(t *testing.T) {
	picks := []finishPick{
		{Bead: "weft-e.1", Title: "feat: A", Change: "cha"},
		{Bead: "weft-e.2", Title: "fix: B", Change: "chb"},
	}
	body := assemblePRBody("weft-e", "E title", picks)
	for _, want := range []string{
		"2 picks woven for weft-e",
		"`weft-e.1` feat: A (`cha`)",
		"`weft-e.2` fix: B (`chb`)",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("body missing %q\n---\n%s", want, body)
		}
	}
}

// finishPreflightRunner returns a routeRunner where EVERY finish-open preflight
// check passes and the epic has one closed pick. `over` overlays
// command-specific responses (matched FIRST) so a single test can fail exactly
// one check while all earlier checks still pass — otherwise a refusal test
// could trip an earlier check and pass for the wrong reason.
//
// Routing precision matters: the clean-tree probe is `jj --no-pager st`, whose
// joined form ENDS with " st". Do NOT route it on Contains(j,"st") — that also
// matches "git remote liST". Use HasSuffix(j," st").
func finishPreflightRunner(over func(j string) (run.Result, bool)) *routeRunner {
	return &routeRunner{fn: func(name string, args []string) run.Result {
		j := strings.Join(append([]string{name}, args...), " ")
		if over != nil {
			if res, ok := over(j); ok {
				return res
			}
		}
		switch {
		case strings.HasSuffix(j, " st"):
			return run.Result{Stdout: "The working copy has no changes.\n", Code: 0}
		case strings.Contains(j, "log -r trunk()..@"):
			return run.Result{Stdout: "cha\n", Code: 0}
		case strings.Contains(j, "git remote list"):
			return run.Result{Stdout: "origin https://github.com/o/r (git)\n", Code: 0}
		case strings.Contains(j, "auth status"):
			return run.Result{Code: 0}
		case strings.Contains(j, "bd list --parent weft-e --status closed"):
			return run.Result{Stdout: `[{"id":"weft-e.1","title":"feat: A","labels":["jj-change:cha"]}]`, Code: 0}
		case strings.Contains(j, "bd show weft-e"):
			return run.Result{Stdout: `[{"title":"Epic E","status":"open","labels":[]}]`, Code: 0}
		}
		return run.Result{Code: 0}
	}}
}

func TestFinishOpenRefusesDirtyTree(t *testing.T) {
	r := finishPreflightRunner(func(j string) (run.Result, bool) {
		if strings.HasSuffix(j, " st") {
			return run.Result{Stdout: "Working copy changes:\nM internal/cli/finish.go\n", Code: 0}, true
		}
		return run.Result{}, false
	})
	_, err := newTestCmd(r, "finish", "open", "weft-e")
	if got := exit.Code(err); got != 1 {
		t.Fatalf("dirty working copy must be exit 1, got %d (err=%v)", got, err)
	}
	for _, c := range r.calls {
		if strings.Contains(strings.Join(c, " "), "git push") {
			t.Errorf("must not push with a dirty working copy: %v", c)
		}
	}
}

func TestFinishOpenRefusesEmptyStack(t *testing.T) {
	r := finishPreflightRunner(func(j string) (run.Result, bool) {
		if strings.Contains(j, "log -r trunk()..@") {
			return run.Result{Stdout: "", Code: 0}, true // nothing to ship
		}
		return run.Result{}, false
	})
	_, err := newTestCmd(r, "finish", "open", "weft-e", "--json")
	if got := exit.Code(err); got != 1 {
		t.Fatalf("empty stack must be exit 1, got %d (err=%v)", got, err)
	}
}

func TestFinishOpenRefusesNoRemote(t *testing.T) {
	r := finishPreflightRunner(func(j string) (run.Result, bool) {
		if strings.Contains(j, "git remote list") {
			return run.Result{Stdout: "", Code: 0}, true // no origin
		}
		return run.Result{}, false
	})
	_, err := newTestCmd(r, "finish", "open", "weft-e")
	if got := exit.Code(err); got != 1 {
		t.Fatalf("missing origin must be exit 1, got %d (err=%v)", got, err)
	}
}

func TestFinishOpenRefusesEmptyEpic(t *testing.T) {
	r := finishPreflightRunner(func(j string) (run.Result, bool) {
		if strings.Contains(j, "bd list --parent weft-e --status closed") {
			return run.Result{Stdout: `[]`, Code: 0}, true // nothing woven
		}
		return run.Result{}, false
	})
	_, err := newTestCmd(r, "finish", "open", "weft-e")
	if got := exit.Code(err); got != 1 {
		t.Fatalf("empty epic must be exit 1, got %d (err=%v)", got, err)
	}
	for _, c := range r.calls {
		if len(c) > 0 && (strings.Contains(strings.Join(c, " "), "git push") || strings.Contains(strings.Join(c, " "), "pr create")) {
			t.Errorf("must not push/create PR for an empty epic: %v", c)
		}
	}
}

// finishOpenPreflight error-branch coverage (weft-1vh). A subprocess that
// cannot START (Runner returns a non-nil error) and one that exits non-zero are
// BOTH engine hard failures (exit 2) for the jj steps routed through hardJJ;
// gh auth status is the exception — its non-zero exit is a user-fixable
// invocation error (exit 1). The tests below pin each of those mappings.

func TestFinishOpenPreflightJJStRunnerError(t *testing.T) {
	r := finishPreflightRunner(nil)
	r.errFn = func(name string, args []string) error {
		if name == "jj" && len(args) >= 2 && args[1] == "st" {
			return errors.New("jj binary vanished")
		}
		return nil
	}
	_, err := newTestCmd(r, "finish", "open", "weft-e")
	if got := exit.Code(err); got != 2 {
		t.Errorf("jj st runner error must be a hard failure (exit 2), got %d (err=%v)", got, err)
	}
}

func TestFinishOpenPreflightGHAuthRunnerError(t *testing.T) {
	r := finishPreflightRunner(nil)
	r.errFn = func(name string, args []string) error {
		if name == "gh" && len(args) >= 2 && args[0] == "auth" && args[1] == "status" {
			return errors.New("gh not installed")
		}
		return nil
	}
	_, err := newTestCmd(r, "finish", "open", "weft-e")
	if got := exit.Code(err); got != 2 {
		t.Errorf("gh auth status runner error must be a hard failure (exit 2), got %d (err=%v)", got, err)
	}
}

// An installed-but-unauthenticated gh (non-zero exit, not a runner error) is a
// user-fixable invocation error (exit 1) — distinct from the could-not-run
// hard-fail above.
func TestFinishOpenRefusesUnauthenticatedGH(t *testing.T) {
	r := finishPreflightRunner(func(j string) (run.Result, bool) {
		if strings.Contains(j, "auth status") {
			return run.Result{Code: 1, Stderr: "not logged in"}, true
		}
		return run.Result{}, false
	})
	_, err := newTestCmd(r, "finish", "open", "weft-e")
	if got := exit.Code(err); got != 1 {
		t.Errorf("unauthenticated gh must be an invocation error (exit 1), got %d (err=%v)", got, err)
	}
}

// hardJJ maps a NON-ZERO exit (distinct from a runner error) to a hard failure
// too. Cover the two preflight steps beyond jj st that route through it — jj log
// and jj git remote list — so a broken jj (e.g. corrupt repo) can't fall through
// the guard untested (weft-7tj.7).
func TestFinishOpenPreflightJJLogNonZeroExit(t *testing.T) {
	r := finishPreflightRunner(func(j string) (run.Result, bool) {
		if strings.Contains(j, "log -r trunk()..@") {
			return run.Result{Code: 1, Stderr: "jj log exploded"}, true
		}
		return run.Result{}, false
	})
	_, err := newTestCmd(r, "finish", "open", "weft-e")
	if got := exit.Code(err); got != 2 {
		t.Errorf("jj log non-zero exit must be a hard failure (exit 2), got %d (err=%v)", got, err)
	}
}

func TestFinishOpenPreflightRemoteListNonZeroExit(t *testing.T) {
	r := finishPreflightRunner(func(j string) (run.Result, bool) {
		if strings.Contains(j, "git remote list") {
			return run.Result{Code: 1, Stderr: "remote list exploded"}, true
		}
		return run.Result{}, false
	})
	_, err := newTestCmd(r, "finish", "open", "weft-e")
	if got := exit.Code(err); got != 2 {
		t.Errorf("jj git remote list non-zero exit must be a hard failure (exit 2), got %d (err=%v)", got, err)
	}
}

func TestFinishOpenDryRunMutatesNothing(t *testing.T) {
	r := finishPreflightRunner(nil)
	out, err := newTestCmd(r, "finish", "open", "weft-e", "--dry-run", "--json")
	if err != nil {
		t.Fatalf("dry-run: %v", err)
	}
	for _, c := range r.calls {
		j := strings.Join(c, " ")
		if strings.Contains(j, "git push") || strings.Contains(j, "pr create") || strings.Contains(j, "bookmark set") {
			t.Errorf("dry-run must not mutate; saw %v", c)
		}
	}
	if !strings.Contains(out.String(), `"dry_run": true`) {
		t.Errorf("dry-run envelope missing dry_run:true: %q", out.String())
	}
}

func TestFinishOpenPushesAndCreatesPR(t *testing.T) {
	r := finishPreflightRunner(func(j string) (run.Result, bool) {
		if strings.Contains(j, "pr view weft-e") {
			return run.Result{Code: 1, Stderr: "no pull requests found"}, true // no existing PR
		}
		if strings.Contains(j, "pr create") {
			return run.Result{Stdout: "https://github.com/o/r/pull/42\n", Code: 0}, true
		}
		return run.Result{}, false
	})
	out, err := newTestCmd(r, "finish", "open", "weft-e", "--json")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	var sawSet, sawPush, sawCreate bool
	for _, c := range r.calls {
		j := strings.Join(c, " ")
		sawSet = sawSet || strings.Contains(j, "bookmark set weft-e")
		sawPush = sawPush || strings.Contains(j, "git push -b weft-e")
		sawCreate = sawCreate || strings.Contains(j, "pr create")
	}
	if !sawSet || !sawPush || !sawCreate {
		t.Fatalf("expected bookmark set + push + pr create; set=%v push=%v create=%v calls=%v", sawSet, sawPush, sawCreate, r.calls)
	}
	var env struct {
		Data struct {
			PRURL string       `json:"pr_url"`
			Picks []finishPick `json:"picks"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(out.String()), &env); err != nil {
		t.Fatalf("decode envelope: %v; out=%q", err, out.String())
	}
	if env.Data.PRURL != "https://github.com/o/r/pull/42" {
		t.Errorf("pr_url = %q", env.Data.PRURL)
	}
	if len(env.Data.Picks) != 1 || env.Data.Picks[0].Change != "cha" {
		t.Errorf("picks = %+v", env.Data.Picks)
	}
}

func TestFinishOpenTitleFromEpicTitle(t *testing.T) {
	r := finishPreflightRunner(func(j string) (run.Result, bool) {
		if strings.Contains(j, "pr view weft-e") {
			return run.Result{Code: 1, Stderr: "no pull requests found"}, true // no existing PR
		}
		if strings.Contains(j, "pr create") {
			return run.Result{Stdout: "https://github.com/o/r/pull/9\n", Code: 0}, true
		}
		return run.Result{}, false
	})
	if _, err := newTestCmd(r, "finish", "open", "weft-e", "--json"); err != nil {
		t.Fatalf("open: %v", err)
	}
	var title string
	for _, c := range r.calls {
		for i, a := range c {
			if a == "--title" && i+1 < len(c) {
				title = c[i+1]
			}
		}
	}
	if title != "Epic E (weft-e)" {
		t.Errorf("PR title = %q, want %q (epic-title (epic-id), spec §4.2)", title, "Epic E (weft-e)")
	}
}

func TestFinishOpenIdempotentWhenPRExists(t *testing.T) {
	r := finishPreflightRunner(func(j string) (run.Result, bool) {
		if strings.Contains(j, "pr view weft-e") {
			return run.Result{Stdout: `{"url":"https://github.com/o/r/pull/7","state":"OPEN"}`, Code: 0}, true
		}
		return run.Result{}, false
	})
	out, err := newTestCmd(r, "finish", "open", "weft-e", "--json")
	if err != nil {
		t.Fatalf("open (existing PR): %v", err)
	}
	for _, c := range r.calls {
		if strings.Contains(strings.Join(c, " "), "pr create") {
			t.Errorf("must NOT create a second PR when one exists: %v", c)
		}
	}
	if !strings.Contains(out.String(), `"pr_exists": true`) || !strings.Contains(out.String(), "pull/7") {
		t.Errorf("expected pr_exists:true + existing url: %q", out.String())
	}
}

func TestFinishOpenDryRunPicksSerializeAsArray(t *testing.T) {
	// One closed pick → picks is a populated []; assert it's a JSON array, not null.
	r := finishPreflightRunner(nil)
	out, _ := newTestCmd(r, "finish", "open", "weft-e", "--dry-run", "--json")
	if !strings.Contains(out.String(), `"picks": [`) {
		t.Errorf("picks must serialize as a JSON array: %q", out.String())
	}
}

func TestFinishReconcileRefusesUnmergedPR(t *testing.T) {
	r := &routeRunner{fn: func(name string, args []string) run.Result {
		j := strings.Join(append([]string{name}, args...), " ")
		if strings.Contains(j, "pr view weft-e") {
			return run.Result{Stdout: `{"state":"OPEN","mergeCommit":null}`, Code: 0}
		}
		return run.Result{Code: 0}
	}}
	_, err := newTestCmd(r, "finish", "reconcile", "weft-e")
	if got := exit.Code(err); got != 1 {
		t.Fatalf("unmerged PR must be exit 1, got %d (err=%v)", got, err)
	}
	if err == nil || !strings.Contains(err.Error(), "(OPEN)") {
		t.Errorf("refusal must name the PR state per spec §6.1: %v", err)
	}
	for _, c := range r.calls {
		if strings.Contains(strings.Join(c, " "), "abandon") || strings.Contains(strings.Join(c, " "), "rebase") {
			t.Errorf("must NOT touch jj topology when the PR is not merged: %v", c)
		}
	}
}

func TestMergeStyleDetectsTrueMergeVsSquash(t *testing.T) {
	// Ancestor present → merge_commit.
	rMerge := &routeRunner{fn: func(name string, args []string) run.Result {
		j := strings.Join(append([]string{name}, args...), " ")
		if strings.Contains(j, "weft-e@origin & ::main@origin") {
			return run.Result{Stdout: "deadbeef\n", Code: 0}
		}
		return run.Result{Code: 0}
	}}
	if got, err := mergeStyle(rMerge, "weft-e"); err != nil || got != "merge_commit" {
		t.Errorf("ancestor present → merge_commit; got %q err=%v", got, err)
	}
	// Ancestor absent → squash_or_rebase.
	rSquash := &routeRunner{fn: func(name string, args []string) run.Result {
		j := strings.Join(append([]string{name}, args...), " ")
		if strings.Contains(j, "weft-e@origin & ::main@origin") {
			return run.Result{Stdout: "", Code: 0}
		}
		return run.Result{Code: 0}
	}}
	if got, err := mergeStyle(rSquash, "weft-e"); err != nil || got != "squash_or_rebase" {
		t.Errorf("ancestor absent → squash_or_rebase; got %q err=%v", got, err)
	}
}

// mergedReconcileRunner: PR merged; merge-style is controlled by `ancestor`.
func mergedReconcileRunner(ancestor bool, extra func(j string) (run.Result, bool)) *routeRunner {
	return &routeRunner{fn: func(name string, args []string) run.Result {
		j := strings.Join(append([]string{name}, args...), " ")
		if extra != nil {
			if res, ok := extra(j); ok {
				return res
			}
		}
		switch {
		case strings.Contains(j, "pr view weft-e"):
			return run.Result{Stdout: `{"state":"MERGED","mergeCommit":{"oid":"abc"}}`, Code: 0}
		case strings.Contains(j, "weft-e@origin & ::main@origin"):
			if ancestor {
				return run.Result{Stdout: "deadbeef\n", Code: 0}
			}
			return run.Result{Stdout: "", Code: 0}
		case strings.Contains(j, "roots(trunk()..@)"):
			return run.Result{Stdout: "rootchg\n", Code: 0}
		}
		return run.Result{Code: 0}
	}}
}

func TestFinishReconcileSquashUsesNewAndAbandon(t *testing.T) {
	r := mergedReconcileRunner(false, nil)
	out, err := newTestCmd(r, "finish", "reconcile", "weft-e", "--json")
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	var sawNew, sawAbandon, sawRebase bool
	for _, c := range r.calls {
		j := strings.Join(c, " ")
		sawNew = sawNew || strings.Contains(j, "new main")
		sawAbandon = sawAbandon || strings.Contains(j, "abandon")
		sawRebase = sawRebase || strings.Contains(j, "rebase")
	}
	if !sawNew || !sawAbandon {
		t.Errorf("squash path must use jj new main + abandon; new=%v abandon=%v", sawNew, sawAbandon)
	}
	if sawRebase {
		t.Errorf("squash path must NOT rebase: %v", r.calls)
	}
	if !strings.Contains(out.String(), `"merge_style": "squash_or_rebase"`) {
		t.Errorf("envelope merge_style wrong: %q", out.String())
	}
}

func TestFinishReconcileTrueMergeUsesRebase(t *testing.T) {
	r := mergedReconcileRunner(true, nil)
	out, err := newTestCmd(r, "finish", "reconcile", "weft-e", "--json")
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	var sawRebase, sawAbandon bool
	for _, c := range r.calls {
		j := strings.Join(c, " ")
		sawRebase = sawRebase || strings.Contains(j, "rebase -b @ -o main")
		sawAbandon = sawAbandon || strings.Contains(j, "abandon")
	}
	if !sawRebase {
		t.Errorf("true-merge path must rebase --skip-emptied: %v", r.calls)
	}
	if sawAbandon {
		t.Errorf("true-merge path must NOT abandon: %v", r.calls)
	}
	if !strings.Contains(out.String(), `"merge_style": "merge_commit"`) {
		t.Errorf("envelope merge_style wrong: %q", out.String())
	}
	if !strings.Contains(out.String(), `"abandoned": []`) {
		t.Errorf("true-merge path must serialize abandoned as an empty array (spec §10): %q", out.String())
	}
}

func TestFinishReconcileDeletesStaleRemoteBranch(t *testing.T) {
	r := mergedReconcileRunner(false, func(j string) (run.Result, bool) {
		if strings.Contains(j, "repo view") {
			return run.Result{Stdout: `{"nameWithOwner":"o/r"}`, Code: 0}, true
		}
		if strings.Contains(j, "api -X DELETE repos/o/r/git/refs/heads/weft-e") {
			return run.Result{Code: 0}, true
		}
		return run.Result{}, false
	})
	out, err := newTestCmd(r, "finish", "reconcile", "weft-e", "--json")
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	var sawDelete bool
	for _, c := range r.calls {
		if strings.Contains(strings.Join(c, " "), "api -X DELETE repos/o/r/git/refs/heads/weft-e") {
			sawDelete = true
		}
	}
	if !sawDelete {
		t.Errorf("expected gh api DELETE of the remote ref; calls=%v", r.calls)
	}
	if !strings.Contains(out.String(), `"remote_branch_deleted": true`) {
		t.Errorf("envelope must report remote_branch_deleted:true: %q", out.String())
	}
}

// A non-404 delete failure (auth/5xx/429) is spec-sanctioned best-effort — it
// must NOT fail the verb, but it MUST be surfaced rather than collapsed into the
// same silent false as a 404 (weft-3jg / weft-9db.12).
func TestFinishReconcileSurfacesNon404DeleteFailure(t *testing.T) {
	r := mergedReconcileRunner(false, func(j string) (run.Result, bool) {
		if strings.Contains(j, "repo view") {
			return run.Result{Stdout: `{"nameWithOwner":"o/r"}`, Code: 0}, true
		}
		if strings.Contains(j, "api -X DELETE repos/o/r/git/refs/heads/weft-e") {
			return run.Result{Code: 1, Stderr: "gh: Must have admin rights to Repository. (HTTP 403)"}, true
		}
		return run.Result{}, false
	})
	out, err := newTestCmd(r, "finish", "reconcile", "weft-e", "--json")
	if err != nil {
		t.Fatalf("reconcile: %v", err) // best-effort: a 403 must not abort reconcile
	}
	var env struct {
		Data struct {
			RemoteBranchDeleted bool   `json:"remote_branch_deleted"`
			RemoteBranchWarning string `json:"remote_branch_warning"`
		} `json:"data"`
	}
	if e := json.Unmarshal(out.Bytes(), &env); e != nil {
		t.Fatalf("unmarshal envelope: %v\n%s", e, out.String())
	}
	if env.Data.RemoteBranchDeleted {
		t.Errorf("a 403 delete did not remove the branch; remote_branch_deleted must be false")
	}
	if !strings.Contains(env.Data.RemoteBranchWarning, "403") {
		t.Errorf("non-404 failure must surface a warning carrying the gh stderr; got %q", env.Data.RemoteBranchWarning)
	}
}

// A 404 (already-gone) is the expected best-effort path — silent: deleted=false,
// no warning (spec §6.1 step 4).
func TestFinishReconcileSilentOn404DeleteFailure(t *testing.T) {
	r := mergedReconcileRunner(false, func(j string) (run.Result, bool) {
		if strings.Contains(j, "repo view") {
			return run.Result{Stdout: `{"nameWithOwner":"o/r"}`, Code: 0}, true
		}
		if strings.Contains(j, "api -X DELETE repos/o/r/git/refs/heads/weft-e") {
			return run.Result{Code: 1, Stderr: "gh: Not Found (HTTP 404)"}, true
		}
		return run.Result{}, false
	})
	out, err := newTestCmd(r, "finish", "reconcile", "weft-e", "--json")
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	var env struct {
		Data struct {
			RemoteBranchDeleted bool   `json:"remote_branch_deleted"`
			RemoteBranchWarning string `json:"remote_branch_warning"`
		} `json:"data"`
	}
	if e := json.Unmarshal(out.Bytes(), &env); e != nil {
		t.Fatalf("unmarshal envelope: %v\n%s", e, out.String())
	}
	if env.Data.RemoteBranchDeleted {
		t.Errorf("404 means already-gone: remote_branch_deleted must be false")
	}
	if env.Data.RemoteBranchWarning != "" {
		t.Errorf("404 is spec-sanctioned silent best-effort; warning must be empty, got %q", env.Data.RemoteBranchWarning)
	}
}

func TestFinishReconcileDryRunMutatesNothing(t *testing.T) {
	r := mergedReconcileRunner(false, nil)
	out, err := newTestCmd(r, "finish", "reconcile", "weft-e", "--dry-run", "--json")
	if err != nil {
		t.Fatalf("dry-run: %v", err)
	}
	for _, c := range r.calls {
		j := strings.Join(c, " ")
		if strings.Contains(j, "new main") || strings.Contains(j, "abandon") || strings.Contains(j, "rebase") || strings.Contains(j, "bookmark delete") || strings.Contains(j, "api -X DELETE") || strings.Contains(j, "git fetch") {
			t.Errorf("dry-run must not mutate; saw %v", c)
		}
	}
	var env struct {
		Data struct {
			DryRun              bool   `json:"dry_run"`
			RemoteBranchWarning string `json:"remote_branch_warning"`
		} `json:"data"`
	}
	if e := json.Unmarshal(out.Bytes(), &env); e != nil {
		t.Fatalf("unmarshal envelope: %v\n%s", e, out.String())
	}
	if !env.Data.DryRun {
		t.Errorf("dry-run envelope missing dry_run:true: %q", out.String())
	}
	// Shape parity: the warning key is present-but-empty in dry-run, never absent.
	if env.Data.RemoteBranchWarning != "" {
		t.Errorf("dry-run deletes nothing; remote_branch_warning must be empty, got %q", env.Data.RemoteBranchWarning)
	}
}

// --- input validation (epic id allowlist; revset/path injection guard) ---

func TestFinishRejectsInjectionEpicID(t *testing.T) {
	bad := []string{"weft-e & all()", "weft-e@origin", "a/b", "weft e", "weft-e:x", "..", "-rf", ".hidden"}
	for _, epic := range bad {
		for _, verb := range []string{"open", "reconcile"} {
			r := &routeRunner{fn: func(string, []string) run.Result { return run.Result{Code: 0} }}
			_, err := newTestCmd(r, "finish", verb, epic)
			if got := exit.Code(err); got != 1 {
				t.Errorf("%s %q: want exit 1 (invalid epic), got %d (err=%v)", verb, epic, got, err)
			}
			if len(r.calls) != 0 {
				t.Errorf("%s %q: no subprocess may run before epic validation; saw %v", verb, epic, r.calls)
			}
		}
	}
}

// --- PR body uses the epic TITLE, not the id (regression: assemblePRBody arg) ---

func TestFinishOpenBodyUsesEpicTitle(t *testing.T) {
	r := finishPreflightRunner(nil)
	// Non-JSON dry-run emits the assembled body in the human text.
	out, err := newTestCmd(r, "finish", "open", "weft-e", "--dry-run")
	if err != nil {
		t.Fatalf("dry-run: %v", err)
	}
	if !strings.Contains(out.String(), "woven for weft-e — Epic E") {
		t.Errorf("PR body summary must use the epic title, not the id: %q", out.String())
	}
}

// --- closedPicks error surfacing (runner error + non-zero exit) ---

func TestClosedPicksSurfacesRunnerError(t *testing.T) {
	if _, err := closedPicks(errRunner{}, "weft-e"); exit.Code(err) != 2 {
		t.Errorf("runner error must be hard (exit 2), got %v", err)
	}
}

func TestClosedPicksSurfacesNonZeroExit(t *testing.T) {
	r := &routeRunner{fn: func(string, []string) run.Result { return run.Result{Code: 1, Stderr: "nope"} }}
	if _, err := closedPicks(r, "weft-e"); exit.Code(err) != 2 {
		t.Errorf("non-zero bd exit must be hard (exit 2), got %v", err)
	}
}

// --- prState surfaces a malformed gh pr view payload ---

func TestPrStateSurfacesParseFailure(t *testing.T) {
	r := &routeRunner{fn: func(string, []string) run.Result { return run.Result{Stdout: "not json", Code: 0} }}
	if _, err := prState(r, "weft-e"); exit.Code(err) != 2 {
		t.Errorf("malformed gh pr view must be hard (exit 2), got %v", err)
	}
}

// --- mergeStyle surfaces a non-zero jj log exit ---

func TestMergeStyleSurfacesNonZeroExit(t *testing.T) {
	r := &routeRunner{fn: func(string, []string) run.Result { return run.Result{Code: 1, Stderr: "bad revset"} }}
	if _, err := mergeStyle(r, "weft-e"); exit.Code(err) != 2 {
		t.Errorf("non-zero jj log exit must be hard (exit 2), got %v", err)
	}
}

// --- deleteRemoteBranch best-effort idempotency (already-absent / unresolvable) ---

func TestDeleteRemoteBranchAbsentReturnsFalse(t *testing.T) {
	r := &routeRunner{fn: func(name string, args []string) run.Result {
		j := strings.Join(append([]string{name}, args...), " ")
		if strings.Contains(j, "repo view") {
			return run.Result{Stdout: `{"nameWithOwner":"o/r"}`, Code: 0}
		}
		if strings.Contains(j, "api -X DELETE") {
			return run.Result{Code: 1, Stderr: "HTTP 404: Not Found"} // already gone
		}
		return run.Result{Code: 0}
	}}
	deleted, warn := deleteRemoteBranch(r, "weft-e")
	if deleted {
		t.Error("an already-absent remote branch (404) must yield false, not a deletion")
	}
	if warn != "" {
		t.Errorf("a 404 is the silent best-effort case; warning must be empty, got %q", warn)
	}
}

func TestDeleteRemoteBranchUnresolvableSlugSurfacesWarning(t *testing.T) {
	r := &routeRunner{fn: func(string, []string) run.Result { return run.Result{Code: 1, Stderr: "no auth"} }} // repo view fails
	deleted, warn := deleteRemoteBranch(r, "weft-e")
	if deleted {
		t.Error("an unresolvable repo slug must yield false")
	}
	if warn == "" {
		t.Error("an unresolvable slug is a non-404 failure and must surface a warning, not stay silent")
	}
}

// isHTTPNotFound must key on the numeric "HTTP 404" marker only — a bare
// "not found" substring would mis-classify non-404 failures (a 403 permission
// denial, a 422 reference error, a DNS lookup failure) as the silent 404 path
// and swallow the warning weft-3jg exists to surface (weft-7tj.8).
func TestIsHTTPNotFoundRequiresNumericMarker(t *testing.T) {
	cases := []struct {
		stderr string
		want   bool
	}{
		{"gh: Not Found (HTTP 404)", true},
		{"HTTP 404: Not Found", true},
		{"gh: Must have admin rights to Repository. (HTTP 403)", false},
		{"gh: Resource not found (HTTP 403)", false}, // the false-positive a bare "not found" match would wrongly silence
		{"gh: Reference not found (HTTP 422)", false},
		{"dial tcp: lookup api.github.com: no such host", false},
		{"", false},
	}
	for _, c := range cases {
		if got := isHTTPNotFound(c.stderr); got != c.want {
			t.Errorf("isHTTPNotFound(%q) = %v, want %v", c.stderr, got, c.want)
		}
	}
}
