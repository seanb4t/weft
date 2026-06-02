// internal/cli/shed_test.go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Weft Contributors

package cli

import (
	"strings"
	"testing"

	"github.com/seanb4t/weft/internal/exit"
	"github.com/seanb4t/weft/internal/run"
)

func TestShedFormBuildsWaveFromBdReady(t *testing.T) {
	fake := &scriptedRunner{res: run.Result{
		Stdout: `[{"id":"weft-a1","title":"x"},{"id":"weft-a2","title":"y"}]`,
		Code:   0,
	}}
	out, err := newTestCmd(fake, "shed", "form", "--epic", "weft-hjx", "--json")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	s := out.String()
	if !strings.Contains(s, `"weft-a1"`) || !strings.Contains(s, `"weft-a2"`) {
		t.Errorf("wave missing expected picks: %q", s)
	}
	// Verify it scoped bd ready to the epic.
	joined := strings.Join(fake.gotArgs, " ")
	if fake.gotName != "bd" || !strings.Contains(joined, "ready") || !strings.Contains(joined, "--parent weft-hjx") {
		t.Errorf("ran %s %v, want bd ready --parent weft-hjx ...", fake.gotName, fake.gotArgs)
	}
}

func TestShedFormRequiresEpic(t *testing.T) {
	_, err := newTestCmd(&scriptedRunner{}, "shed", "form")
	if got := exit.Code(err); got != 1 {
		t.Fatalf("missing --epic should be exit code 1, got %d (err=%v)", got, err)
	}
}

func TestShedFormEmptyWaveEmitsJSONArrayNotNull(t *testing.T) {
	fake := &scriptedRunner{res: run.Result{Stdout: `[]`, Code: 0}}
	out, err := newTestCmd(fake, "shed", "form", "--epic", "weft-hjx", "--json")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if s := out.String(); !strings.Contains(s, `"wave": []`) {
		t.Errorf("empty wave must serialize as [], not null: %q", s)
	}
}

func TestShedFormNonZeroBdExitIsHardFailure(t *testing.T) {
	fake := &scriptedRunner{res: run.Result{Code: 1, Stderr: "bd: unknown epic"}}
	_, err := newTestCmd(fake, "shed", "form", "--epic", "weft-hjx")
	if got := exit.Code(err); got != 2 {
		t.Fatalf("non-zero bd exit should be a hard failure (exit 2), got %d (err=%v)", got, err)
	}
	if err == nil || !strings.Contains(err.Error(), "bd: unknown epic") {
		t.Errorf("hard-failure error should surface bd stderr, got %v", err)
	}
}

func TestShedFormRunnerErrorIsHardFailure(t *testing.T) {
	_, err := newTestCmd(errRunner{}, "shed", "form", "--epic", "weft-hjx")
	if got := exit.Code(err); got != 2 {
		t.Fatalf("bd that cannot start should be a hard failure (exit 2), got %d (err=%v)", got, err)
	}
}

func TestShedFormMaxMustBePositive(t *testing.T) {
	_, err := newTestCmd(&scriptedRunner{}, "shed", "form", "--epic", "weft-hjx", "--max", "0")
	if got := exit.Code(err); got != 1 {
		t.Fatalf("--max 0 should be an invocation error (exit 1), got %d (err=%v)", got, err)
	}
}

func TestShedFormPassesMaxAsLimit(t *testing.T) {
	fake := &scriptedRunner{res: run.Result{Stdout: `[]`, Code: 0}}
	if _, err := newTestCmd(fake, "shed", "form", "--epic", "weft-hjx", "--max", "3"); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if joined := strings.Join(fake.gotArgs, " "); !strings.Contains(joined, "--limit 3") {
		t.Errorf("--max 3 should pass --limit 3 to bd, got args %v", fake.gotArgs)
	}
}
