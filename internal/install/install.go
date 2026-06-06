// internal/install/install.go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Weft Contributors

// Package install drives Claude Code's `claude plugin` CLI to register the weft
// repo as a marketplace and install the weft plugin, pinned to the running
// binary's release (spec docs/seams/07-weft-install.md). It depends only on the
// run.Runner interface so it is unit-testable with the engine's fake runner.
package install

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/seanb4t/weft/internal/exit"
	"github.com/seanb4t/weft/internal/run"
)

// validScopes are the Claude Code install scopes (claude plugin install --scope).
var validScopes = map[string]bool{"user": true, "project": true, "local": true}

// refPattern allowlists a git ref before it is interpolated into the
// `claude plugin marketplace add <source>@<ref>` argument. Leading char is
// alphanumeric (rejects a leading '-' that bd/claude could read as a flag); the
// rest is the git-ref-safe set [A-Za-z0-9._/-]. This excludes every shell/
// revset metacharacter. ".." traversal is rejected by the separate
// strings.Contains(ref, "..") guard in validateRef, not by this regex (the
// regex permits each '.' individually). Mirrors the guard idiom on
// changeIDPattern/epicIDPattern (conflict.go/finish.go); see the engram memory
// weft-cli-validate-user-id-before-revset-or-gh-api.
var refPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._/-]*$`)

func validateScope(scope string) error {
	if !validScopes[scope] {
		return exit.Invocationf("invalid --scope %q — must be user, project, or local", scope)
	}
	return nil
}

func validateRef(ref string) error {
	if !refPattern.MatchString(ref) || strings.Contains(ref, "..") {
		return exit.Invocationf("invalid --ref %q", ref)
	}
	return nil
}

// validateLocal requires an existing directory carrying a marketplace manifest,
// and rejects a leading dash (flag confusion) and any path containing '@'
// (the claude CLI interprets source@ref as a remote ref — passing a local path
// with '@' defeats the offline contract).
func validateLocal(path string) error {
	if path == "" || strings.HasPrefix(path, "-") {
		return exit.Invocationf("invalid --local path %q", path)
	}
	if strings.Contains(path, "@") {
		return exit.Invocationf("--local path %q must not contain '@' (use --ref for remote refs)", path)
	}
	if fi, err := os.Stat(filepath.Join(path, ".claude-plugin/marketplace.json")); err != nil || fi.IsDir() {
		return exit.Invocationf("--local %q has no .claude-plugin/marketplace.json", path)
	}
	return nil
}

const repoSlug = "seanb4t/weft"

// semverPattern matches a clean release version X.Y.Z only (no pre-release or
// build suffix). The dev sentinel "0.0.0-dev" — and any other suffixed build —
// deliberately fails it, so the default pin path refuses to float; pre-release
// or dev builds must pass --ref (or --local).
var semverPattern = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+$`)

// resolveSource picks the marketplace source + ref. Precedence: --local (a clone
// path, no ref) > --ref (override the ref on the repo) > default (pin the plugin
// tag weft--v<version> for a released binary). A dev/untagged version with no
// --ref/--local refuses rather than silently floating to a branch (spec §4.2).
// --ref and --local are mutually exclusive; Install enforces this before calling
// resolveSource.
func resolveSource(version, ref, local string) (source, refArg string, err error) {
	if local != "" {
		return local, "", nil
	}
	if ref != "" {
		return repoSlug, ref, nil
	}
	if !semverPattern.MatchString(version) {
		return "", "", exit.Invocationf(
			"weft %s is not a released build — pass --ref <git-ref> or --local <path> to install", version)
	}
	return repoSlug, "weft--v" + version, nil
}

// Options drives one weft install invocation.
type Options struct {
	Version   string // the binary's version (cli.Version); pins weft--v<Version>
	Scope     string // user | project | local
	Ref       string // optional ref override (branch/tag/sha); mutually exclusive with Local
	Local     string // optional local clone path (marketplace source); mutually exclusive with Ref
	Uninstall bool   // remove instead of install
	DryRun    bool   // report the commands without running them
}

// Result is the install outcome (becomes the envelope data).
type Result struct {
	Plugin      string
	Marketplace string
	Source      string
	Ref         string
	Scope       string
	Uninstall   bool
	Registered  bool
	Installed   bool
	Commands    []string
}

const pluginName = "weft"

// Install validates options, ensures the claude CLI is reachable, then drives
// `claude plugin` to register the marketplace and install (or uninstall) the
// weft plugin. Best-effort nothing here: any non-zero claude exit is surfaced.
func Install(r run.Runner, o Options) (Result, error) {
	if err := validateScope(o.Scope); err != nil {
		return Result{}, err
	}
	// --ref and --local are mutually exclusive: --local is an offline path and
	// carries no ref; --ref applies only to the remote repo source.
	if o.Ref != "" && o.Local != "" {
		return Result{}, exit.Invocationf("--ref and --local are mutually exclusive")
	}
	if o.Ref != "" {
		if err := validateRef(o.Ref); err != nil {
			return Result{}, err
		}
	}
	if o.Local != "" {
		if err := validateLocal(o.Local); err != nil {
			return Result{}, err
		}
	}
	res := Result{Plugin: pluginName, Marketplace: pluginName, Scope: o.Scope, Uninstall: o.Uninstall, Commands: []string{}}

	if o.Uninstall {
		res.Commands = []string{"claude plugin uninstall " + pluginName + " --scope " + o.Scope + " -y"}
		if o.DryRun {
			return res, nil
		}
		if err := claudeCheck(r); err != nil {
			return res, err
		}
		if err := runClaude(r, "plugin", "uninstall", pluginName, "--scope", o.Scope, "-y"); err != nil {
			return res, err
		}
		return res, nil
	}

	source, refArg, err := resolveSource(o.Version, o.Ref, o.Local)
	if err != nil {
		return Result{}, err
	}
	res.Source, res.Ref = source, refArg
	addArg := source
	if refArg != "" {
		addArg = source + "@" + refArg
	}
	res.Commands = []string{
		"claude plugin marketplace add " + addArg,
		"claude plugin install " + pluginName + "@" + pluginName + " --scope " + o.Scope,
	}
	if o.DryRun {
		return res, nil
	}
	if err := claudeCheck(r); err != nil {
		return res, err
	}
	if err := registerMarketplace(r, addArg); err != nil {
		return res, err
	}
	res.Registered = true
	if err := runClaude(r, "plugin", "install", pluginName+"@"+pluginName, "--scope", o.Scope); err != nil {
		return res, err
	}
	res.Installed = true
	return res, nil
}

// claudeCheck probes that the claude CLI is reachable and functional. A runner
// error means claude is not on PATH; a non-zero exit from `claude --version`
// means it is present but not usable. Both are hard failures — the verb cannot
// proceed without the host CLI.
func claudeCheck(r run.Runner) error {
	res, err := run.Claude(r, "--version")
	if err != nil {
		return exit.Hardf("claude CLI not found on PATH — install Claude Code, or run the printed commands by hand: %v", err)
	}
	if res.Code != 0 {
		return exit.Hardf("claude --version exited %d — claude CLI is not usable; install Claude Code or run the printed commands by hand", res.Code)
	}
	return nil
}

// runClaude runs one `claude plugin …` step, mapping both failure modes
// (could-not-start, non-zero exit) to a hard error with the stderr surfaced.
// If stderr is empty, stdout is included instead so no diagnostic is lost.
func runClaude(r run.Runner, args ...string) error {
	res, err := run.Claude(r, args...)
	if err != nil {
		return exit.Hardf("claude %s could not run: %v", strings.Join(args, " "), err)
	}
	if res.Code != 0 {
		msg := strings.TrimSpace(res.Stderr)
		if msg == "" {
			msg = strings.TrimSpace(res.Stdout)
		}
		return exit.Hardf("claude %s failed: %s", strings.Join(args, " "), msg)
	}
	return nil
}

// registerMarketplace adds the marketplace, tolerating an already-registered name
// by removing and re-adding (the live CLI's duplicate-add semantic is unconfirmed
// — spec §4.4; an integration test pins the real behavior). For genuine failures
// (network/auth/invalid source) the original add error is preserved and surfaced
// so the real diagnostic is not clobbered by the fallback.
func registerMarketplace(r run.Runner, addArg string) error {
	res, err := run.Claude(r, "plugin", "marketplace", "add", addArg)
	if err == nil && res.Code == 0 {
		return nil
	}
	// Capture the original failure for diagnostics before attempting fallback.
	var origErr error
	if err != nil {
		origErr = exit.Hardf("claude plugin marketplace add %s could not run: %v", addArg, err)
	} else {
		origErr = exit.Hardf("claude plugin marketplace add %s failed (original): stderr=%q stdout=%q",
			addArg, strings.TrimSpace(res.Stderr), strings.TrimSpace(res.Stdout))
	}
	// Fallback: remove then re-add (duplicate-tolerance, spec §4.4).
	_, _ = run.Claude(r, "plugin", "marketplace", "remove", pluginName)
	if reAddErr := runClaude(r, "plugin", "marketplace", "add", addArg); reAddErr != nil {
		// Re-add also failed — the original error is more informative.
		return origErr
	}
	return nil
}
