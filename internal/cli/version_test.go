// internal/cli/version_test.go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Weft Contributors

package cli

import (
	"bytes"
	"errors"
	"runtime/debug"
	"strings"
	"testing"

	"github.com/seanb4t/weft/internal/config"
	"github.com/seanb4t/weft/internal/run"
)

// newTestCmd builds a root command wired to a fake runner with captured output.
func newTestCmd(fake run.Runner, args ...string) (*bytes.Buffer, error) {
	out := &bytes.Buffer{}
	root := NewRootCmd(&App{Runner: fake})
	root.SetOut(out)
	root.SetErr(out)
	root.SetArgs(args)
	return out, root.Execute()
}

// scriptedRunner is the shared fake Runner for verb tests: it returns a fixed
// Result and records the command it was asked to run.
type scriptedRunner struct {
	res     run.Result
	gotName string
	gotArgs []string
}

func (s *scriptedRunner) Run(name string, args ...string) (run.Result, error) {
	s.gotName, s.gotArgs = name, args
	return s.res, nil
}

// errRunner is a fake Runner whose command never starts (e.g. missing binary):
// it returns a non-nil error, the signal Exec uses for "could not run".
type errRunner struct{}

func (errRunner) Run(string, ...string) (run.Result, error) {
	return run.Result{}, errors.New("exec: command not found")
}

// TestNewAppPanicsOnNilRunner verifies that NewApp panics immediately when
// given a nil Runner rather than deferring to a nil-deref deep in a verb
// (qeg.6).
func TestNewAppPanicsOnNilRunner(t *testing.T) {
	panicked := false
	func() {
		defer func() {
			if r := recover(); r != nil {
				panicked = true
			}
		}()
		NewApp(nil, config.Config{})
	}()
	if !panicked {
		t.Fatal("NewApp(nil, ...) should panic but did not")
	}
}

func TestVersionText(t *testing.T) {
	out, err := newTestCmd(nil, "version")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(out.String(), "weft "+Version) {
		t.Errorf("text output = %q, want it to contain %q", out.String(), "weft "+Version)
	}
}

func TestVersionJSON(t *testing.T) {
	out, err := newTestCmd(nil, "version", "--json")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	s := out.String()
	if !strings.Contains(s, `"verb": "version"`) || !strings.Contains(s, `"ok": true`) {
		t.Errorf("json output missing envelope fields: %q", s)
	}
}

func TestVersionPick(t *testing.T) {
	out, err := newTestCmd(nil, "version", "--pick", "data.version")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if strings.TrimSpace(out.String()) != Version {
		t.Errorf("pick output = %q, want %q", strings.TrimSpace(out.String()), Version)
	}
}

// TestPickStringBoolAndNumber verifies that pickString renders bools and JSON
// numbers (float64) without quoting or spurious decimals (qeg.22).
// JSON numbers decode as float64; 42 must render as "42", not "42.0".
func TestPickStringBoolAndNumber(t *testing.T) {
	cases := []struct {
		in   any
		want string
	}{
		{true, "true"},
		{false, "false"},
		{float64(42), "42"},
		{float64(3.14), "3.14"},
		{"hello", "hello"},
	}
	for _, tc := range cases {
		if got := pickString(tc.in); got != tc.want {
			t.Errorf("pickString(%v) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestResolveVersion(t *testing.T) {
	bi := func(v string) func() (*debug.BuildInfo, bool) {
		return func() (*debug.BuildInfo, bool) {
			if v == "" {
				return nil, false
			}
			return &debug.BuildInfo{Main: debug.Module{Version: v}}, true
		}
	}
	tests := []struct {
		name, ldflags, buildInfo, want string
	}{
		{"ldflags clean", "0.1.0", "", "0.1.0"},
		{"ldflags v-prefixed", "v0.1.0", "", "0.1.0"},
		{"go install module version", "", "v0.2.0", "0.2.0"},
		{"local devel placeholder", "", "(devel)", "0.0.0-dev"},
		{"no build info", "", "", "0.0.0-dev"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := resolveVersion(tt.ldflags, bi(tt.buildInfo)); got != tt.want {
				t.Errorf("resolveVersion(%q, bi=%q) = %q, want %q", tt.ldflags, tt.buildInfo, got, tt.want)
			}
		})
	}

	// ReadBuildInfo can report ok=true with an empty Main.Version (binaries built
	// outside a module context). The bi() helper above maps "" to ok=false, so it
	// never reaches this branch — assert it explicitly: an empty version must fall
	// through to the dev sentinel, not become a bare "" after TrimPrefix.
	t.Run("build info ok but empty version", func(t *testing.T) {
		okEmpty := func() (*debug.BuildInfo, bool) {
			return &debug.BuildInfo{Main: debug.Module{Version: ""}}, true
		}
		if got := resolveVersion("", okEmpty); got != "0.0.0-dev" {
			t.Errorf(`resolveVersion("", ok=true/empty version) = %q, want "0.0.0-dev"`, got)
		}
	})
}
