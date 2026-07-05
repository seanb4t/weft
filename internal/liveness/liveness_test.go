// internal/liveness/liveness_test.go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Weft Contributors

package liveness

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/seanb4t/weft/internal/run"
)

// fakeRunner routes jj calls; mirrors internal/cli's routeRunner shape
// (that type is package-private to cli, so liveness has its own).
type fakeRunner struct {
	fn func(name string, args []string) run.Result
}

func (f *fakeRunner) Run(name string, args ...string) (run.Result, error) {
	return f.fn(name, args), nil
}

// tsLayout is defined in liveness.go (same package); tests reuse it to build
// expected values, so the parse format under test and under assertion stay
// identical.

func jjTimestampRunner(ts string) *fakeRunner {
	return &fakeRunner{fn: func(name string, args []string) run.Result {
		return run.Result{Stdout: ts + "\n", Code: 0}
	}}
}

func TestLastActivityUsesWorkspaceCommitTimestamp(t *testing.T) {
	// No directory on disk: the jj signal alone carries the answer.
	r := jjTimestampRunner("2026-07-01T10:00:00-0400")
	got, err := LastActivity(r, "weft-abc__1", filepath.Join(t.TempDir(), "missing"))
	if err != nil {
		t.Fatal(err)
	}
	want, _ := time.Parse(tsLayout, "2026-07-01T10:00:00-0400")
	if !got.Equal(want) {
		t.Errorf("got %v want %v", got, want)
	}
}

func TestLastActivityNewerMtimeWins(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "edited.go")
	if err := os.WriteFile(f, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	newer := time.Now().Add(-1 * time.Minute)
	if err := os.Chtimes(f, newer, newer); err != nil {
		t.Fatal(err)
	}
	r := jjTimestampRunner("2026-01-01T00:00:00+0000") // stale jj signal
	got, err := LastActivity(r, "weft-abc__1", dir)
	if err != nil {
		t.Fatal(err)
	}
	if got.Sub(newer).Abs() > 2*time.Second {
		t.Errorf("mtime should win: got %v want ~%v", got, newer)
	}
}

func TestLastActivityIgnoresDotJJ(t *testing.T) {
	dir := t.TempDir()
	jjdir := filepath.Join(dir, ".jj", "working_copy")
	if err := os.MkdirAll(jjdir, 0o755); err != nil {
		t.Fatal(err)
	}
	f := filepath.Join(jjdir, "checkout")
	if err := os.WriteFile(f, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	// .jj mtime is NOW; the jj signal is old — .jj must not count as activity.
	r := jjTimestampRunner("2026-01-01T00:00:00+0000")
	got, err := LastActivity(r, "weft-abc__1", dir)
	if err != nil {
		t.Fatal(err)
	}
	old, _ := time.Parse(tsLayout, "2026-01-01T00:00:00+0000")
	if !got.Equal(old) {
		t.Errorf(".jj contents must be ignored: got %v want %v", got, old)
	}
}

func TestLastActivityRejectsUnsafeWorkspaceName(t *testing.T) {
	r := jjTimestampRunner("2026-01-01T00:00:00+0000")
	if _, err := LastActivity(r, "bad name & rev", t.TempDir()); err == nil {
		t.Fatal("unsafe workspace name must not reach a revset")
	}
}

func TestLastActivityJJFailureIsError(t *testing.T) {
	r := &fakeRunner{fn: func(string, []string) run.Result {
		return run.Result{Stderr: "boom", Code: 1}
	}}
	if _, err := LastActivity(r, "weft-abc__1", t.TempDir()); err == nil {
		t.Fatal("jj failure is an infrastructure anomaly, not silence")
	}
}

func TestLive(t *testing.T) {
	now := time.Now()
	if !Live(now.Add(-10*time.Minute), now, 45*time.Minute) {
		t.Error("10m ago within 45m threshold must be live")
	}
	if Live(now.Add(-2*time.Hour), now, 45*time.Minute) {
		t.Error("2h ago beyond 45m threshold must be dead")
	}
}
