// internal/cli/pick_test.go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Weft Contributors

package cli

import (
	"strings"
	"testing"

	"github.com/seanb4t/weft/internal/run"
)

func TestPickSealCommitsAndLabels(t *testing.T) {
	r := &routeRunner{fn: func(name string, args []string) run.Result {
		j := strings.Join(append([]string{name}, args...), " ")
		switch {
		case strings.Contains(j, "bd show"):
			return run.Result{Stdout: `[{"title":"Add X","status":"in_progress","labels":[]}]`, Code: 0}
		case strings.Contains(j, "log -r @-"):
			return run.Result{Stdout: "ch4ng3id000\n", Code: 0}
		default: // jj commit, bd update --add-label
			return run.Result{Code: 0}
		}
	}}
	out, err := newTestCmd(r, "pick", "seal", "weft-hjx.1.1", "--json")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	var sawCommit, sawLabel bool
	for _, c := range r.calls {
		j := strings.Join(c, " ")
		if strings.Contains(j, `jj --no-pager commit -m feat(weft-hjx.1.1): Add X`) {
			sawCommit = true
		}
		if strings.Contains(j, "bd update weft-hjx.1.1 --add-label jj-change:ch4ng3id000") {
			sawLabel = true
		}
	}
	if !sawCommit {
		t.Errorf("expected conventional-commit jj commit; calls=%v", r.calls)
	}
	if !sawLabel {
		t.Errorf("expected jj-change label write; calls=%v", r.calls)
	}
	if !strings.Contains(out.String(), "ch4ng3id000") {
		t.Errorf("output missing change-id: %q", out.String())
	}
}
