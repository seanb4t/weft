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
// revset metacharacter and rejects ".." traversal. Mirrors the guard idiom on
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
// and rejects a leading dash (flag confusion).
func validateLocal(path string) error {
	if path == "" || strings.HasPrefix(path, "-") {
		return exit.Invocationf("invalid --local path %q", path)
	}
	if fi, err := os.Stat(path + "/.claude-plugin/marketplace.json"); err != nil || fi.IsDir() {
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
	Ref       string // optional ref override (branch/tag/sha)
	Local     string // optional local clone path (marketplace source)
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

// claudeCheck probes that the claude CLI is reachable; a runner error means it is
// not on PATH (a hard failure — the verb cannot proceed without the host CLI).
func claudeCheck(r run.Runner) error {
	if _, err := run.Claude(r, "--version"); err != nil {
		return exit.Hardf("claude CLI not found on PATH — install Claude Code, or run the printed commands by hand: %v", err)
	}
	return nil
}

// runClaude runs one `claude plugin …` step, mapping both failure modes
// (could-not-start, non-zero exit) to a hard error with the stderr surfaced.
func runClaude(r run.Runner, args ...string) error {
	res, err := run.Claude(r, args...)
	if err != nil {
		return exit.Hardf("claude %s could not run: %v", strings.Join(args, " "), err)
	}
	if res.Code != 0 {
		return exit.Hardf("claude %s failed: %s", strings.Join(args, " "), strings.TrimSpace(res.Stderr))
	}
	return nil
}

// registerMarketplace adds the marketplace, tolerating an already-registered name
// by removing and re-adding (the live CLI's duplicate-add semantic is unconfirmed
// — spec §4.4; an integration test pins the real behavior).
func registerMarketplace(r run.Runner, addArg string) error {
	if res, err := run.Claude(r, "plugin", "marketplace", "add", addArg); err == nil && res.Code == 0 {
		return nil
	}
	// Fallback: remove then re-add.
	_, _ = run.Claude(r, "plugin", "marketplace", "remove", pluginName)
	return runClaude(r, "plugin", "marketplace", "add", addArg)
}
