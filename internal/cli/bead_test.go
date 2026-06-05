// internal/cli/bead_test.go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Weft Contributors

package cli

import (
	"fmt"
	"testing"

	"github.com/seanb4t/weft/internal/exit"
	"github.com/seanb4t/weft/internal/run"
)

func TestShowBeadParsesFields(t *testing.T) {
	r := &routeRunner{fn: func(_ string, _ []string) run.Result {
		return run.Result{Stdout: `[{"title":"T","status":"in_progress","labels":["phase:run","jj-change:abc123"]}]`, Code: 0}
	}}
	info, err := showBead(r, "weft-hjx.1.1")
	if err != nil {
		t.Fatalf("showBead error: %v", err)
	}
	if info.Title != "T" || info.Status != "in_progress" {
		t.Errorf("fields = %+v", info)
	}
}

// fp0.13: showBead Hardf branch when bd show returns a non-zero exit.
func TestShowBeadHardfOnBdError(t *testing.T) {
	r := &routeRunner{fn: func(name string, args []string) run.Result {
		return run.Result{Code: 1, Stderr: "bd: internal error"}
	}}
	_, err := showBead(r, "weft-hjx.1.1")
	if got := exit.Code(err); got != 2 {
		t.Fatalf("non-zero bd show must be exit 2 (Hardf), got %d (err=%v)", got, err)
	}
}

// fp0.13: showBead Hardf branch when the runner itself fails to start.
func TestShowBeadHardfOnRunnerError(t *testing.T) {
	r := &routeRunner{errFn: func(string, []string) error { return fmt.Errorf("exec: bd not found") }}
	_, err := showBead(r, "weft-hjx.1.1")
	if got := exit.Code(err); got != 2 {
		t.Fatalf("runner error on bd show must be exit 2 (Hardf), got %d (err=%v)", got, err)
	}
}

func TestChangeOfReadsSpineLabel(t *testing.T) {
	withLabels := func(labels string) run.Runner {
		return &routeRunner{fn: func(_ string, _ []string) run.Result {
			return run.Result{Stdout: `[{"title":"T","status":"in_progress","labels":` + labels + `}]`, Code: 0}
		}}
	}
	got, err := changeOf(withLabels(`["jj-change:abc123","phase:run"]`), "b")
	if err != nil || got != "abc123" {
		t.Fatalf("changeOf = %q, %v", got, err)
	}
	got, err = changeOf(withLabels(`["phase:run"]`), "b")
	if err != nil || got != "" {
		t.Fatalf("unsealed changeOf = %q, %v (want empty)", got, err)
	}
}
