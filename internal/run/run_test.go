// internal/run/run_test.go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Weft Contributors

package run

import "testing"

func TestExecCapturesStdoutAndZeroExit(t *testing.T) {
	res, err := Exec{}.Run("echo", "hello")
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if res.Code != 0 {
		t.Errorf("Code = %d, want 0", res.Code)
	}
	if res.Stdout != "hello\n" {
		t.Errorf("Stdout = %q, want %q", res.Stdout, "hello\n")
	}
}

func TestExecNonZeroExitIsNotAnError(t *testing.T) {
	res, err := Exec{}.Run("false")
	if err != nil {
		t.Fatalf("non-zero exit must not be a Go error, got %v", err)
	}
	if res.Code == 0 {
		t.Error("Code should be non-zero for `false`")
	}
}

func TestExecMissingBinaryIsAnError(t *testing.T) {
	_, err := (Exec{}).Run("definitely-not-a-real-binary-xyz")
	if err == nil {
		t.Fatal("expected an error when the binary cannot start")
	}
}

func TestJJPrependsNoPager(t *testing.T) {
	var fake fakeRunner
	_, _ = JJ(&fake, "status")
	want := []string{"--no-pager", "status"}
	if fake.name != "jj" || !equal(fake.args, want) {
		t.Errorf("JJ ran %s %v, want jj %v", fake.name, fake.args, want)
	}
}

// TestBDInvokesRunnerWithNameAndArgs verifies that BD calls the runner with
// name "bd" and passes through args unchanged, returning its Result/error
// (qeg.10).
func TestBDInvokesRunnerWithNameAndArgs(t *testing.T) {
	want := Result{Stdout: "ok\n", Code: 0}
	fake := &resultRunner{res: want}
	got, err := BD(fake, "ready", "--parent", "weft-hjx", "--limit", "5")
	if err != nil {
		t.Fatalf("BD returned unexpected error: %v", err)
	}
	if got != want {
		t.Errorf("BD result = %+v, want %+v", got, want)
	}
	if fake.name != "bd" {
		t.Errorf("BD called runner with name %q, want %q", fake.name, "bd")
	}
	wantArgs := []string{"ready", "--parent", "weft-hjx", "--limit", "5"}
	if !equal(fake.args, wantArgs) {
		t.Errorf("BD args = %v, want %v", fake.args, wantArgs)
	}
}

func TestGHInvokesRunnerWithNameAndArgs(t *testing.T) {
	f := &fakeRunner{}
	if _, err := GH(f, "pr", "view", "weft-x", "--json", "state"); err != nil {
		t.Fatalf("GH: %v", err)
	}
	if f.name != "gh" {
		t.Errorf("binary = %q, want gh", f.name)
	}
	want := []string{"pr", "view", "weft-x", "--json", "state"}
	if !equal(f.args, want) {
		t.Errorf("args = %v, want %v", f.args, want)
	}
}

// resultRunner records the last call and returns a fixed Result.
type resultRunner struct {
	name string
	args []string
	res  Result
}

func (r *resultRunner) Run(name string, args ...string) (Result, error) {
	r.name, r.args = name, args
	return r.res, nil
}

type fakeRunner struct {
	name string
	args []string
}

func (f *fakeRunner) Run(name string, args ...string) (Result, error) {
	f.name, f.args = name, args
	return Result{}, nil
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
