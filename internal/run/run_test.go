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
