// internal/install/install_test.go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Weft Contributors

package install

import (
	"strings"
	"testing"

	"github.com/seanb4t/weft/internal/exit"
	"github.com/seanb4t/weft/internal/run"
)

func TestValidateScope(t *testing.T) {
	for _, ok := range []string{"user", "project", "local"} {
		if err := validateScope(ok); err != nil {
			t.Errorf("scope %q should be valid: %v", ok, err)
		}
	}
	for _, bad := range []string{"global", "", "User", "-x"} {
		if err := validateScope(bad); exit.Code(err) != 1 {
			t.Errorf("scope %q must be invocation error (exit 1), got %v", bad, err)
		}
	}
}

func TestValidateRefAllowlist(t *testing.T) {
	for _, ok := range []string{"main", "weft--v1.2.3", "v0.4.0", "0123456789abcdef0123456789abcdef01234567"} {
		if err := validateRef(ok); err != nil {
			t.Errorf("ref %q should be valid: %v", ok, err)
		}
	}
	for _, bad := range []string{"-rf", "a b", "a&b", "a|b", "a;b", "a$(x)", ".."} {
		if err := validateRef(bad); exit.Code(err) != 1 {
			t.Errorf("ref %q must be rejected (exit 1), got %v", bad, err)
		}
	}
}

func TestResolveSourceDefaultPinsPluginTag(t *testing.T) {
	src, ref, err := resolveSource("1.4.0", "", "")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if src != "seanb4t/weft" || ref != "weft--v1.4.0" {
		t.Errorf("default must pin seanb4t/weft@weft--v1.4.0, got %q@%q", src, ref)
	}
}

func TestResolveSourceDevVersionRefuses(t *testing.T) {
	if _, _, err := resolveSource("0.0.0-dev", "", ""); exit.Code(err) != 1 {
		t.Errorf("dev/untagged version with no --ref/--local must refuse (exit 1), got %v", err)
	}
}

func TestResolveSourceRefOverride(t *testing.T) {
	src, ref, err := resolveSource("0.0.0-dev", "main", "")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if src != "seanb4t/weft" || ref != "main" {
		t.Errorf("--ref must override to seanb4t/weft@main, got %q@%q", src, ref)
	}
}

func TestResolveSourceLocalUsesPathNoRef(t *testing.T) {
	src, ref, err := resolveSource("0.0.0-dev", "", "/tmp/weft-clone")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if src != "/tmp/weft-clone" || ref != "" {
		t.Errorf("--local must use the path with no ref, got %q@%q", src, ref)
	}
}

// scriptRunner records calls and returns scripted results keyed on the joined
// arg string. (Named distinctly from the cli package's routeRunner — different
// package, different fn signature: this one keys on the pre-joined string.)
type scriptRunner struct {
	fn    func(j string) run.Result
	calls [][]string
}

func (r *scriptRunner) Run(name string, args ...string) (run.Result, error) {
	r.calls = append(r.calls, append([]string{name}, args...))
	if r.fn == nil {
		return run.Result{Code: 0}, nil
	}
	return r.fn(strings.Join(append([]string{name}, args...), " ")), nil
}

func okRunner() *scriptRunner {
	return &scriptRunner{fn: func(j string) run.Result {
		if strings.Contains(j, "--version") { // the `claude` prereq probe
			return run.Result{Stdout: "2.1.165", Code: 0}
		}
		return run.Result{Code: 0}
	}}
}

func TestInstallDefaultDrivesMarketplaceThenInstall(t *testing.T) {
	r := okRunner()
	res, err := Install(r, Options{Version: "1.4.0", Scope: "user"})
	if err != nil {
		t.Fatalf("install: %v", err)
	}
	joined := make([]string, len(r.calls))
	for i, c := range r.calls {
		joined[i] = strings.Join(c, " ")
	}
	add, ins := -1, -1
	for i, j := range joined {
		if strings.Contains(j, "plugin marketplace add seanb4t/weft@weft--v1.4.0") {
			add = i
		}
		if strings.Contains(j, "plugin install weft@weft --scope user") {
			ins = i
		}
	}
	if add < 0 || ins < 0 || add > ins {
		t.Fatalf("must add marketplace then install (add=%d ins=%d): %v", add, ins, joined)
	}
	if !res.Registered || !res.Installed {
		t.Errorf("result must report registered+installed: %+v", res)
	}
}

func TestInstallClaudeAbsentIsHardError(t *testing.T) {
	// errRunner fails to start (simulates `claude` missing from PATH) → exit 2.
	if _, err := Install(&errRunner{}, Options{Version: "1.4.0", Scope: "user"}); exit.Code(err) != 2 {
		t.Errorf("claude absent must be hard error (exit 2), got %v", err)
	}
}

func TestInstallSubprocessFailureIsHardError(t *testing.T) {
	r := &scriptRunner{fn: func(j string) run.Result {
		if strings.Contains(j, "--version") {
			return run.Result{Stdout: "2.1.165", Code: 0}
		}
		if strings.Contains(j, "plugin install") {
			return run.Result{Code: 1, Stderr: "boom"}
		}
		return run.Result{Code: 0}
	}}
	if _, err := Install(r, Options{Version: "1.4.0", Scope: "user"}); exit.Code(err) != 2 {
		t.Errorf("non-zero claude plugin exit must be hard (exit 2), got %v", err)
	}
}

func TestInstallUninstallRunsUninstallOnly(t *testing.T) {
	r := okRunner()
	if _, err := Install(r, Options{Version: "1.4.0", Scope: "user", Uninstall: true}); err != nil {
		t.Fatalf("uninstall: %v", err)
	}
	var sawUninstall, sawInstall bool
	for _, c := range r.calls {
		j := strings.Join(c, " ")
		if strings.Contains(j, "plugin uninstall weft --scope user -y") {
			sawUninstall = true
		}
		if strings.Contains(j, "plugin install weft@") {
			sawInstall = true
		}
	}
	if !sawUninstall || sawInstall {
		t.Errorf("uninstall path must run uninstall -y and not install (un=%v in=%v)", sawUninstall, sawInstall)
	}
}

// errRunner fails to start (simulates `claude` missing from PATH).
type errRunner struct{}

func (errRunner) Run(string, ...string) (run.Result, error) {
	return run.Result{}, errStub
}

var errStub = stubErr("claude: not found")

type stubErr string

func (e stubErr) Error() string { return string(e) }
