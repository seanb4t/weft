// internal/cli/bead_test.go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Weft Contributors

package cli

import (
	"testing"

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
