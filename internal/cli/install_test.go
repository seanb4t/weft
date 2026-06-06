// internal/cli/install_test.go
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

func installRunner() *routeRunner {
	return &routeRunner{fn: func(name string, args []string) run.Result {
		j := strings.Join(append([]string{name}, args...), " ")
		if strings.Contains(j, "--version") {
			return run.Result{Stdout: "2.1.165", Code: 0}
		}
		return run.Result{Code: 0}
	}}
}

// NOTE: cli.Version is the dev sentinel "0.0.0-dev" during tests, which
// resolveSource refuses (it can't derive a release tag). So every cli install
// test that reaches resolveSource passes --ref to supply a resolvable source.
// (The validation/uninstall tests below short-circuit before resolveSource.)
func TestInstallDryRunRunsNoSubprocess(t *testing.T) {
	r := installRunner()
	out, err := newTestCmd(r, "install", "--ref", "main", "--dry-run", "--json")
	if err != nil {
		t.Fatalf("dry-run: %v", err)
	}
	if len(r.calls) != 0 {
		t.Errorf("dry-run must run no subprocess; saw %v", r.calls)
	}
	var env struct {
		Data struct {
			Commands []string `json:"commands"`
			DryRun   bool     `json:"dry_run"`
		} `json:"data"`
	}
	if e := json.Unmarshal(out.Bytes(), &env); e != nil {
		t.Fatalf("envelope: %v\n%s", e, out.String())
	}
	if !env.Data.DryRun || len(env.Data.Commands) != 2 {
		t.Errorf("dry-run envelope must carry dry_run:true + 2 commands: %s", out.String())
	}
}

func TestInstallRejectsBadScope(t *testing.T) {
	r := installRunner()
	_, err := newTestCmd(r, "install", "--scope", "global")
	if exit.Code(err) != 1 {
		t.Errorf("bad scope must be exit 1, got %v", err)
	}
	if len(r.calls) != 0 {
		t.Errorf("no subprocess before validation; saw %v", r.calls)
	}
}

func TestInstallRejectsInjectionRef(t *testing.T) {
	for _, bad := range []string{"-rf", "a b", "a&all()", ".."} {
		r := installRunner()
		_, err := newTestCmd(r, "install", "--ref", bad)
		if exit.Code(err) != 1 {
			t.Errorf("ref %q must be exit 1, got %v", bad, err)
		}
		if len(r.calls) != 0 {
			t.Errorf("ref %q: no subprocess before validation; saw %v", bad, r.calls)
		}
	}
}

func TestInstallEnvelopeCommandsNeverNull(t *testing.T) {
	r := installRunner()
	out, err := newTestCmd(r, "install", "--ref", "main", "--dry-run", "--json")
	if err != nil {
		t.Fatalf("dry-run: %v", err)
	}
	if !strings.Contains(out.String(), `"commands": [`) {
		t.Errorf("commands must serialize as a JSON array, never null: %s", out.String())
	}
}
