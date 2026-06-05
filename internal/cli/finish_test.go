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
