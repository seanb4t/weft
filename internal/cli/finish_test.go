// internal/cli/finish_test.go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Weft Contributors

package cli

import (
	"encoding/json"
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

func TestFinishReconcileDryRunMutatesNothing(t *testing.T) {
	r := mergedReconcileRunner(false, nil)
	out, err := newTestCmd(r, "finish", "reconcile", "weft-e", "--dry-run", "--json")
	if err != nil {
		t.Fatalf("dry-run: %v", err)
	}
	for _, c := range r.calls {
		j := strings.Join(c, " ")
		if strings.Contains(j, "new main") || strings.Contains(j, "abandon") || strings.Contains(j, "rebase") || strings.Contains(j, "bookmark delete") || strings.Contains(j, "api -X DELETE") {
			t.Errorf("dry-run must not mutate; saw %v", c)
		}
	}
	if !strings.Contains(out.String(), `"dry_run": true`) {
		t.Errorf("dry-run envelope missing dry_run:true: %q", out.String())
	}
}
