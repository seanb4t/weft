// internal/cli/version_test.go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Weft Contributors

package cli

import (
	"bytes"
	"errors"
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
