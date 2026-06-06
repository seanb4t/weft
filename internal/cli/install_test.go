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

// TestInstallDryRunOmitsRestartHint covers the Qodo PR #23 finding: a --dry-run
// installs nothing, so the envelope's "next" must not carry the post-install
// "Restart Claude Code to load" hint.
func TestInstallDryRunOmitsRestartHint(t *testing.T) {
	r := installRunner()
	out, err := newTestCmd(r, "install", "--ref", "main", "--dry-run", "--json")
	if err != nil {
		t.Fatalf("dry-run: %v", err)
	}
	if strings.Contains(out.String(), "Restart Claude Code to load") {
		t.Errorf("dry-run must not emit the post-install restart hint: %s", out.String())
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

// TestInstallEnvelopeCommandsNeverNull covers finding weft-i4r.7: the commands
// field must serialize as a JSON array, never null. The check uses JSON
// unmarshalling rather than a fragile substring match.
func TestInstallEnvelopeCommandsNeverNull(t *testing.T) {
	r := installRunner()
	out, err := newTestCmd(r, "install", "--ref", "main", "--dry-run", "--json")
	if err != nil {
		t.Fatalf("dry-run: %v", err)
	}
	// Unmarshal the top-level envelope and pull the raw "data" field.
	var top struct {
		Data json.RawMessage `json:"data"`
	}
	if e := json.Unmarshal(out.Bytes(), &top); e != nil {
		t.Fatalf("envelope unmarshal: %v\n%s", e, out.String())
	}
	// Pull the raw "commands" field out of data.
	var data map[string]json.RawMessage
	if e := json.Unmarshal(top.Data, &data); e != nil {
		t.Fatalf("data unmarshal: %v\n%s", e, out.String())
	}
	raw, ok := data["commands"]
	if !ok {
		t.Fatalf("commands key missing from data: %s", out.String())
	}
	// A JSON array starts with '['; null serializes as the literal "null".
	if len(raw) == 0 || raw[0] != '[' {
		t.Errorf("commands must be a JSON array (not null or missing); got %s", raw)
	}
}

// TestInstallDevBuildRefusalAtCLILayer covers finding weft-i4r.12: when
// cli.Version is the dev sentinel "0.0.0-dev" and neither --ref nor --local is
// provided, the install verb must exit 1 (invocation error) without running any
// subprocess.
func TestInstallDevBuildRefusalAtCLILayer(t *testing.T) {
	r := installRunner()
	// No --ref or --local; Version is "0.0.0-dev" (the test-time constant).
	_, err := newTestCmd(r, "install")
	if exit.Code(err) != 1 {
		t.Errorf("dev-build with no --ref/--local must be exit 1, got %v", err)
	}
	if len(r.calls) != 0 {
		t.Errorf("dev-build refusal must run no subprocess; saw %v", r.calls)
	}
}
