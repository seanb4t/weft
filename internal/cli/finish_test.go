// internal/cli/finish_test.go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Weft Contributors

package cli

import (
	"strings"
	"testing"

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
