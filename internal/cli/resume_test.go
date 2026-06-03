// internal/cli/resume_test.go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Weft Contributors

package cli

import (
	"strings"
	"testing"

	"github.com/seanb4t/weft/internal/run"
)

func TestResumeProjectsState(t *testing.T) {
	r := &routeRunner{fn: func(name string, args []string) run.Result {
		j := strings.Join(append([]string{name}, args...), " ")
		switch {
		case strings.Contains(j, "list --parent weft-hjx.1 --status closed"):
			return run.Result{Stdout: `[{"id":"weft-hjx.1.1"}]`, Code: 0}
		case strings.Contains(j, "list --parent weft-hjx.1 --status in_progress"):
			return run.Result{Stdout: `[{"id":"weft-hjx.1.5"}]`, Code: 0}
		case strings.Contains(j, "list --parent weft-hjx.1 --status blocked"):
			return run.Result{Stdout: `[]`, Code: 0}
		case strings.Contains(j, "ready --parent weft-hjx.1"):
			return run.Result{Stdout: `[{"id":"weft-hjx.1.6"}]`, Code: 0}
		case strings.Contains(j, "conflicts()"):
			return run.Result{Stdout: "", Code: 0}
		default:
			return run.Result{Code: 0}
		}
	}}
	out, err := newTestCmd(r, "resume", "--epic", "weft-hjx.1", "--json")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	s := out.String()
	for _, want := range []string{`"epic": "weft-hjx.1"`, "weft-hjx.1.1", "weft-hjx.1.5", "weft-hjx.1.6"} {
		if !strings.Contains(s, want) {
			t.Errorf("resume output missing %q: %q", want, s)
		}
	}
	// Hard invariant: resume must NOT mutate. Scan every token of every recorded
	// call against an exact mutating-verb set — exact-match (not substring) so
	// "--status closed" is not mistaken for "close", and per-token (not c[1]) so
	// jj's "--no-pager"-prefixed calls (verb at c[2]) are still checked.
	mutating := map[string]bool{"update": true, "close": true, "rebase": true, "abandon": true}
	for _, c := range r.calls {
		for _, tok := range c {
			if mutating[tok] {
				t.Fatalf("resume must be read-only, saw mutating call: %v", c)
			}
		}
	}
}
